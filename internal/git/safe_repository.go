package git

import (
	"context"
	"sync"

	"github.com/Caia-Tech/caia-library/pkg/document"
)

// SafeRepository wraps Repository with mutex protection for concurrent operations
type SafeRepository struct {
	repo *Repository
	mu   sync.Mutex
}

// NewSafeRepository creates a new thread-safe repository wrapper
func NewSafeRepository(path string) (*SafeRepository, error) {
	repo, err := NewRepository(path)
	if err != nil {
		return nil, err
	}
	return &SafeRepository{repo: repo}, nil
}

// GetRepo returns the underlying repository (for direct access when needed)
func (sr *SafeRepository) GetRepo() *Repository {
	return sr.repo
}

// StoreDocument stores a document with mutex protection
func (sr *SafeRepository) StoreDocument(ctx context.Context, doc *document.Document) (string, error) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.repo.StoreDocument(ctx, doc)
}

// MergeBranch merges a branch with mutex protection
func (sr *SafeRepository) MergeBranch(ctx context.Context, branchName string) error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.repo.MergeBranch(ctx, branchName)
}