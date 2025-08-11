package storage

import (
	"context"

	"github.com/Caia-Tech/caia-library/pkg/document"
)

// StorageBackend defines the interface for document storage implementations
type StorageBackend interface {
	StoreDocument(ctx context.Context, doc *document.Document) (string, error)
	GetDocument(ctx context.Context, id string) (*document.Document, error)
	MergeBranch(ctx context.Context, branchName string) error
	ListDocuments(ctx context.Context, filters map[string]string) ([]*document.Document, error)
	Health(ctx context.Context) error
}

// StorageMetrics provides telemetry for storage operations
type StorageMetrics struct {
	OperationType string
	Duration      int64  // nanoseconds
	Success       bool
	Backend       string
	Error         error
}

// MetricsCollector receives storage operation metrics
type MetricsCollector interface {
	RecordMetric(metric StorageMetrics)
}