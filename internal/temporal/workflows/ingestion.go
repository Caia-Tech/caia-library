package workflows

import (
	"time"

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
		StartToCloseTimeout: 10 * time.Minute,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Fetch document
	var fetchResult FetchResult
	if err := workflow.ExecuteActivity(ctx, FetchDocumentActivityName, input.URL).Get(ctx, &fetchResult); err != nil {
		return err
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

// Activity types
type FetchResult struct {
	Content []byte
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