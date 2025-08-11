package presentation

import (
	"context"
	"time"
)

// Document represents a stored document
type Document struct {
	ID       string                 `json:"id"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Storage interface for document storage
type Storage interface {
	Store(doc *Document) error
	Get(id string) (*Document, error)
	List(prefix string, offset, limit int) ([]*Document, error)
	Delete(id string) error
	Search(ctx context.Context, query string, options *SearchOptionsStorage) ([]*Document, error)
	GetStats() (*Stats, error)
	Close() error
}

// SearchOptions for storage search
type SearchOptionsStorage struct {
	MaxResults int
	Filters    map[string]string
}

// Stats represents storage statistics
type Stats struct {
	TotalDocuments int64     `json:"total_documents"`
	TotalSize      int64     `json:"total_size"`
	LastUpdated    time.Time `json:"last_updated"`
}