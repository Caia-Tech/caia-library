package workflows

import (
	"fmt"
	"strings"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type DocumentInput struct {
	URL      string
	Type     string
	Metadata map[string]string
}

// FileProcessingInput represents the input for file processing workflow
type FileProcessingInput struct {
	Filename    string            `json:"filename"`
	ContentType string            `json:"content_type"`
	Content     []byte            `json:"content"`
	Metadata    map[string]string `json:"metadata"`
}

func DocumentIngestionWorkflow(ctx workflow.Context, input DocumentInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting document ingestion", "url", input.URL)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        3,
			InitialInterval:        1 * time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        30 * time.Second,
			NonRetryableErrorTypes: []string{"InvalidInputError", "PDFProcessingError", "*extractor.PDFProcessingError"},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Fetch document
	var fetchResult FetchResult
	if err := workflow.ExecuteActivity(ctx, FetchDocumentActivityName, input.URL).Get(ctx, &fetchResult); err != nil {
		return err
	}

	// Validate content type matches expected type
	if err := validateContentType(fetchResult.ContentType, input.Type); err != nil {
		logger.Warn("Content type mismatch", "expected", input.Type, "actual", fetchResult.ContentType, "error", err)
		// Continue processing but log the mismatch - don't fail completely
	}

	// Parallel processing
	var futures []workflow.Future

	// Extract text
	textFuture := workflow.ExecuteActivity(ctx, ExtractTextActivityName, ExtractInput{
		Content: fetchResult.Content,
		Type:    input.Type,
	})
	futures = append(futures, textFuture)

	// Generate embeddings
	embedFuture := workflow.ExecuteActivity(ctx, GenerateEmbeddingsActivityName, fetchResult.Content)
	futures = append(futures, embedFuture)

	// Wait for parallel activities
	var extractResult ExtractResult
	if err := textFuture.Get(ctx, &extractResult); err != nil {
		return err
	}

	var embeddings []float32
	if err := embedFuture.Get(ctx, &embeddings); err != nil {
		return err
	}

	// Store in Git
	storeInput := StoreInput{
		URL:        input.URL,
		Type:       input.Type,
		Content:    fetchResult.Content,
		Text:       extractResult.Text,
		Metadata:   extractResult.Metadata,
		Embeddings: embeddings,
	}

	var commitHash string
	if err := workflow.ExecuteActivity(ctx, StoreDocumentActivityName, storeInput).Get(ctx, &commitHash); err != nil {
		return err
	}

	// Index document
	if err := workflow.ExecuteActivity(ctx, IndexDocumentActivityName, commitHash).Get(ctx, nil); err != nil {
		return err
	}

	// Merge to main - pass branch name instead of commit hash
	branchName := fmt.Sprintf("ingest/%s", extractResult.Metadata["document_id"])
	if branchName == "ingest/" {
		// Fallback to commit hash if document ID not available
		branchName = fmt.Sprintf("commit-%s", commitHash[:8])
	}
	if err := workflow.ExecuteActivity(ctx, MergeBranchActivityName, branchName).Get(ctx, nil); err != nil {
		return err
	}

	logger.Info("Document ingestion completed", "commitHash", commitHash)
	return nil
}

// FileProcessingWorkflow processes uploaded files (PDF, DOCX, images with OCR)
func FileProcessingWorkflow(ctx workflow.Context, input FileProcessingInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting file processing", "filename", input.Filename, "type", input.ContentType)

	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Minute, // Longer timeout for OCR processing
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts:        3,
			InitialInterval:        1 * time.Second,
			BackoffCoefficient:     2.0,
			MaximumInterval:        30 * time.Second,
			NonRetryableErrorTypes: []string{"InvalidInputError", "PDFProcessingError", "*extractor.PDFProcessingError"},
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Parallel processing
	var futures []workflow.Future

	// Extract text from file
	textFuture := workflow.ExecuteActivity(ctx, ExtractTextActivityName, ExtractInput{
		Content: input.Content,
		Type:    input.ContentType,
	})
	futures = append(futures, textFuture)

	// Generate embeddings from raw content
	embedFuture := workflow.ExecuteActivity(ctx, GenerateEmbeddingsActivityName, input.Content)
	futures = append(futures, embedFuture)

	// Wait for parallel activities
	var extractResult ExtractResult
	if err := textFuture.Get(ctx, &extractResult); err != nil {
		return err
	}

	var embeddings []float32
	if err := embedFuture.Get(ctx, &embeddings); err != nil {
		return err
	}

	// Combine file metadata with extraction metadata
	combinedMetadata := make(map[string]string)
	for k, v := range input.Metadata {
		combinedMetadata[k] = v
	}
	for k, v := range extractResult.Metadata {
		combinedMetadata[k] = v
	}

	// Store in CAIA Library storage
	storeInput := FileStoreInput{
		Filename:   input.Filename,
		Type:       input.ContentType,
		Content:    input.Content,
		Text:       extractResult.Text,
		Metadata:   combinedMetadata,
		Embeddings: embeddings,
	}

	var documentID string
	if err := workflow.ExecuteActivity(ctx, StoreFileActivityName, storeInput).Get(ctx, &documentID); err != nil {
		return err
	}

	// Index the document
	if err := workflow.ExecuteActivity(ctx, IndexDocumentActivityName, documentID).Get(ctx, nil); err != nil {
		return err
	}

	logger.Info("File processing completed", "documentID", documentID, "filename", input.Filename)
	return nil
}

// validateContentType checks if the fetched content type matches the expected document type
func validateContentType(contentType, expectedType string) error {
	contentType = strings.ToLower(contentType)
	expectedType = strings.ToLower(expectedType)

	switch expectedType {
	case "pdf":
		if !strings.Contains(contentType, "application/pdf") {
			return fmt.Errorf("expected PDF but got %s", contentType)
		}
	case "text":
		if !strings.Contains(contentType, "text/") {
			return fmt.Errorf("expected text but got %s", contentType)
		}
	case "html", "web":
		if !strings.Contains(contentType, "text/html") {
			return fmt.Errorf("expected HTML but got %s", contentType)
		}
	default:
		// Unknown type, allow through
		return nil
	}
	return nil
}

// Activity types
type FetchResult struct {
	Content     []byte
	ContentType string
}

type ExtractInput struct {
	Content []byte
	Type    string
}

type ExtractResult struct {
	Text     string
	Metadata map[string]string
}

type StoreInput struct {
	URL        string
	Type       string
	Content    []byte
	Text       string
	Metadata   map[string]string
	Embeddings []float32
}

// FileStoreInput represents input for storing uploaded files
type FileStoreInput struct {
	Filename   string            `json:"filename"`
	Type       string            `json:"type"`
	Content    []byte            `json:"content"`
	Text       string            `json:"text"`
	Metadata   map[string]string `json:"metadata"`
	Embeddings []float32         `json:"embeddings"`
}

// Activity names for registration
const (
	FetchDocumentActivityName      = "FetchDocumentActivity"
	ExtractTextActivityName        = "ExtractTextActivity"
	GenerateEmbeddingsActivityName = "GenerateEmbeddingsActivity"
	StoreDocumentActivityName      = "StoreDocumentActivity"
	StoreFileActivityName          = "StoreFileActivity"
	IndexDocumentActivityName      = "IndexDocumentActivity"
	MergeBranchActivityName        = "MergeBranchActivity"
)