package workflows

import (
	"fmt"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// ScheduledIngestionInput defines a scheduled collection source
type ScheduledIngestionInput struct {
	Name     string            `json:"name"`
	Type     string            `json:"type"` // "rss", "web", "api"
	URL      string            `json:"url"`
	Schedule string            `json:"schedule"` // Cron expression
	Filters  []string          `json:"filters"`
	Metadata map[string]string `json:"metadata"`
}

// ScheduledIngestionWorkflow runs document collection on a schedule
func ScheduledIngestionWorkflow(ctx workflow.Context, input ScheduledIngestionInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting scheduled ingestion", "name", input.Name, "schedule", input.Schedule)

	// Cron schedule is set via workflow options when starting the workflow

	// Activity options with retries
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 30 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    time.Minute,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Collect documents from source
	var documents []CollectedDocument
	
	// Use appropriate collector based on source type
	activityName := "CollectFromSourceActivity"
	if isAcademicSource(input.Name) {
		activityName = "CollectAcademicSourcesActivity"
		logger.Info("Using academic collector with ethical rate limiting", "source", input.Name)
	}
	
	err := workflow.ExecuteActivity(ctx, activityName, input).Get(ctx, &documents)
	if err != nil {
		logger.Error("Failed to collect documents", "error", err)
		return err
	}

	logger.Info("Collected documents", "count", len(documents))

	// Process each document
	var futures []workflow.Future
	for _, doc := range documents {
		// Check if we've seen this document before
		var isDuplicate bool
		checkFuture := workflow.ExecuteActivity(ctx, "CheckDuplicateActivity", doc.ID)
		if err := checkFuture.Get(ctx, &isDuplicate); err == nil && isDuplicate {
			logger.Info("Skipping duplicate document", "id", doc.ID)
			continue
		}

		// Start ingestion workflow for new document
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: "ingest-" + doc.ID,
		})

		future := workflow.ExecuteChildWorkflow(childCtx, DocumentIngestionWorkflow, DocumentInput{
			URL:      doc.URL,
			Type:     doc.Type,
			Metadata: doc.Metadata,
		})
		futures = append(futures, future)
	}

	// Wait for all ingestions to complete
	for _, future := range futures {
		if err := future.Get(ctx, nil); err != nil {
			logger.Error("Document ingestion failed", "error", err)
			// Continue with other documents
		}
	}

	logger.Info("Scheduled ingestion completed", "processed", len(futures))
	return nil
}

// CollectedDocument represents a document found by a collector
type CollectedDocument struct {
	ID       string            `json:"id"`
	URL      string            `json:"url"`
	Type     string            `json:"type"`
	Metadata map[string]string `json:"metadata"`
}

// isAcademicSource checks if the source is an academic provider
func isAcademicSource(name string) bool {
	academicSources := map[string]bool{
		"arxiv":            true,
		"pubmed":           true,
		"doaj":             true,
		"plos":             true,
		"semantic_scholar": true,
		"core":             true,
	}
	return academicSources[name]
}

// BatchIngestionWorkflow processes multiple documents in parallel
func BatchIngestionWorkflow(ctx workflow.Context, documents []DocumentInput) error {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting batch ingestion", "count", len(documents))

	// Process in parallel with controlled concurrency
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)

	var futures []workflow.Future
	for i, doc := range documents {
		// Copy loop variables for closure
		index := i
		document := doc

		// Control concurrency with channel
		sem <- struct{}{}
		
		// Execute child workflow
		childCtx := workflow.WithChildOptions(ctx, workflow.ChildWorkflowOptions{
			WorkflowID: fmt.Sprintf("batch-ingest-%d", index),
		})

		future := workflow.ExecuteChildWorkflow(childCtx, DocumentIngestionWorkflow, document)
		futures = append(futures, future)

		// Release semaphore after workflow starts
		workflow.Go(ctx, func(ctx workflow.Context) {
			future.Get(ctx, nil)
			<-sem
		})
	}

	// Wait for all to complete
	var errors []error
	for _, future := range futures {
		if err := future.Get(ctx, nil); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		logger.Error("Batch ingestion completed with errors", "errorCount", len(errors))
		return fmt.Errorf("batch ingestion had %d errors", len(errors))
	}

	logger.Info("Batch ingestion completed successfully")
	return nil
}

