package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/rs/zerolog/log"
)

// GitBackend implements StorageBackend using traditional Git operations
type GitBackend struct {
	repo            *git.Repository
	repoPath        string
	metricsCollector MetricsCollector
}

// NewGitBackend creates a new Git-based storage backend
func NewGitBackend(repoPath string, metrics MetricsCollector) (*GitBackend, error) {
	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository: %w", err)
	}

	return &GitBackend{
		repo:            repo,
		repoPath:        repoPath,
		metricsCollector: metrics,
	}, nil
}

func (g *GitBackend) StoreDocument(ctx context.Context, doc *document.Document) (string, error) {
	start := time.Now()
	commitHash, err := g.storeDocumentInGit(ctx, doc)
	
	g.recordMetric("store", start, err == nil, err)
	return commitHash, err
}

func (g *GitBackend) GetDocument(ctx context.Context, id string) (*document.Document, error) {
	start := time.Now()
	
	// Search for document in git repository
	doc, err := g.findDocumentByID(ctx, id)
	
	g.recordMetric("get", start, err == nil, err)
	return doc, err
}

func (g *GitBackend) MergeBranch(ctx context.Context, branchName string) error {
	start := time.Now()
	err := g.mergeBranchInGit(ctx, branchName)
	
	g.recordMetric("merge", start, err == nil, err)
	return err
}

func (g *GitBackend) ListDocuments(ctx context.Context, filters map[string]string) ([]*document.Document, error) {
	start := time.Now()
	
	// For now, return empty slice - this would require walking the git tree
	// TODO: Implement document listing from git repository
	documents := []*document.Document{}
	
	g.recordMetric("list", start, true, nil)
	return documents, nil
}

func (g *GitBackend) Health(ctx context.Context) error {
	start := time.Now()
	
	// Check if repository is accessible
	_, err := g.repo.Head()
	
	g.recordMetric("health", start, err == nil, err)
	return err
}

// findDocumentByID searches for a document by ID in the git repository
func (g *GitBackend) findDocumentByID(ctx context.Context, id string) (*document.Document, error) {
	// Search in documents directory using common patterns
	searchPaths := []string{
		fmt.Sprintf("documents/*/*/*/%s", id),
		fmt.Sprintf("documents/*/*/%s", id),
		fmt.Sprintf("documents/*/%s", id),
	}

	for _, pattern := range searchPaths {
		matches, err := filepath.Glob(filepath.Join(g.repoPath, pattern))
		if err != nil {
			continue
		}

		for _, match := range matches {
			doc, err := g.loadDocumentFromPath(match, id)
			if err != nil {
				log.Warn().Err(err).Str("path", match).Msg("Failed to load document")
				continue
			}
			if doc != nil {
				return doc, nil
			}
		}
	}

	return nil, fmt.Errorf("document not found: %s", id)
}

// loadDocumentFromPath loads a document from a filesystem path
func (g *GitBackend) loadDocumentFromPath(docPath, id string) (*document.Document, error) {
	metadataPath := filepath.Join(docPath, "metadata.json")
	
	// Check if metadata file exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, nil // Not a valid document path
	}

	// Read metadata
	metadataBytes, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(metadataBytes, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	// Create document from metadata
	doc := &document.Document{
		ID: id,
	}

	// Parse metadata fields
	if source, ok := metadata["source"].(string); ok {
		doc.Source.URL = source
	}
	
	// Determine document type from path
	pathParts := strings.Split(docPath, string(filepath.Separator))
	for i, part := range pathParts {
		if part == "documents" && i+1 < len(pathParts) {
			doc.Source.Type = pathParts[i+1]
			break
		}
	}

	// Parse timestamps
	if createdAt, ok := metadata["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			doc.CreatedAt = t
		}
	}
	if updatedAt, ok := metadata["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			doc.UpdatedAt = t
		}
	}

	// Read text content
	textPath := filepath.Join(docPath, "text.txt")
	if textBytes, err := os.ReadFile(textPath); err == nil {
		doc.Content.Text = string(textBytes)
	}

	// Read raw content
	rawPath := filepath.Join(docPath, "raw")
	if rawBytes, err := os.ReadFile(rawPath); err == nil {
		doc.Content.Raw = rawBytes
	}

	// Parse content metadata
	if contentMetadata, ok := metadata["metadata"].(map[string]interface{}); ok {
		doc.Content.Metadata = make(map[string]string)
		for k, v := range contentMetadata {
			if str, ok := v.(string); ok {
				doc.Content.Metadata[k] = str
			}
		}
	}

	return doc, nil
}

// storeDocumentInGit stores a document in the git repository
func (g *GitBackend) storeDocumentInGit(ctx context.Context, doc *document.Document) (string, error) {
	// Validate document before storing
	if err := doc.Validate(); err != nil {
		return "", fmt.Errorf("document validation failed: %w", err)
	}
	// Get worktree
	w, err := g.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}

	// Create document directory
	docPath := filepath.Join(g.repoPath, doc.GitPath())
	if err := os.MkdirAll(docPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", docPath, err)
	}

	// Write document files (similar to original implementation)
	if len(doc.Content.Raw) > 0 {
		rawPath := filepath.Join(docPath, "raw")
		if err := os.WriteFile(rawPath, doc.Content.Raw, 0644); err != nil {
			return "", fmt.Errorf("failed to write raw content: %w", err)
		}
	}

	if doc.Content.Text != "" {
		textPath := filepath.Join(docPath, "text.txt")
		if err := os.WriteFile(textPath, []byte(doc.Content.Text), 0644); err != nil {
			return "", fmt.Errorf("failed to write text content: %w", err)
		}
	}

	// Write metadata
	metadata := map[string]interface{}{
		"id":         doc.ID,
		"source":     doc.Source.URL,
		"created_at": doc.CreatedAt,
		"updated_at": doc.UpdatedAt,
		"metadata":   doc.Content.Metadata,
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}

	metadataPath := filepath.Join(docPath, "metadata.json")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}

	// Add files to git
	if _, err := w.Add(doc.GitPath()); err != nil {
		return "", fmt.Errorf("failed to add files: %w", err)
	}

	// Create commit
	commit, err := w.Commit(fmt.Sprintf("Add document %s", doc.ID), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Caia Library",
			Email: "library@caiatech.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}

	return commit.String(), nil
}

// mergeBranchInGit merges a branch (simplified implementation)
func (g *GitBackend) mergeBranchInGit(ctx context.Context, branchName string) error {
	// For now, just log the merge attempt - full merge implementation would be complex
	log.Info().Str("branch", branchName).Msg("Git merge requested (simplified implementation)")
	return nil
}

func (g *GitBackend) recordMetric(operation string, start time.Time, success bool, err error) {
	if g.metricsCollector != nil {
		g.metricsCollector.RecordMetric(StorageMetrics{
			OperationType: operation,
			Duration:      time.Since(start).Nanoseconds(),
			Success:       success,
			Backend:       "git",
			Error:         err,
		})
	}
}