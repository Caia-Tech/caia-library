package activities

import (
	"context"
	"fmt"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
)

// Global storage instance - should be injected via dependency injection in production
var globalHybridStorage *storage.HybridStorage
var globalMetrics *storage.SimpleMetricsCollector

// SetGlobalStorage sets the global storage instance for activities
func SetGlobalStorage(hybridStorage *storage.HybridStorage, metrics *storage.SimpleMetricsCollector) {
	globalHybridStorage = hybridStorage
	globalMetrics = metrics
}

func StoreDocumentActivity(ctx context.Context, input workflows.StoreInput) (string, error) {
	logger := activity.GetLogger(ctx)
	logger.Info("Storing document", "url", input.URL, "type", input.Type)

	if globalHybridStorage == nil {
		return "", fmt.Errorf("hybrid storage not initialized")
	}

	// Create document
	doc := &document.Document{
		ID: uuid.New().String(),
		Source: document.Source{
			Type: input.Type,
			URL:  input.URL,
		},
		Content: document.Content{
			Raw:        input.Content,
			Text:       input.Text,
			Metadata:   input.Metadata,
			Embeddings: input.Embeddings,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	commitHash, err := globalHybridStorage.StoreDocument(ctx, doc)
	if err != nil {
		return "", fmt.Errorf("failed to store document: %w", err)
	}

	logger.Info("Document stored successfully", "documentID", doc.ID, "commitHash", commitHash)
	return commitHash, nil
}