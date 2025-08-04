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

	// Merge to main
	if err := workflow.ExecuteActivity(ctx, MergeBranchActivityName, commitHash).Get(ctx, nil); err != nil {
		return err
	}

	logger.Info("Document ingestion completed", "commitHash", commitHash)
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

// Activity names for registration
const (
	FetchDocumentActivityName      = "FetchDocumentActivity"
	ExtractTextActivityName        = "ExtractTextActivity"
	GenerateEmbeddingsActivityName = "GenerateEmbeddingsActivity"
	StoreDocumentActivityName      = "StoreDocumentActivity"
	IndexDocumentActivityName      = "IndexDocumentActivity"
	MergeBranchActivityName        = "MergeBranchActivity"
)