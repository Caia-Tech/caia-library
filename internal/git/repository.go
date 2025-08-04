package git

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/caiatech/caia-library/pkg/document"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Repository wraps Git operations for document storage
type Repository struct {
	repo *git.Repository
	path string
}

// NewRepository opens an existing Git repository
func NewRepository(path string) (*Repository, error) {
	if path == "" {
		return nil, fmt.Errorf("repository path cannot be empty")
	}

	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository at %s: %w", path, err)
	}
	
	return &Repository{
		repo: repo,
		path: path,
	}, nil
}

// StoreDocument saves a document to the repository in a new branch
func (r *Repository) StoreDocument(ctx context.Context, doc *document.Document) (string, error) {
	// Validate document
	if err := doc.Validate(); err != nil {
		return "", fmt.Errorf("invalid document: %w", err)
	}

	branchName := fmt.Sprintf("ingest/%s", doc.ID)
	
	// Get current HEAD
	headRef, err := r.repo.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}
	
	// Create new branch from HEAD
	ref := plumbing.NewHashReference(plumbing.NewBranchReferenceName(branchName), headRef.Hash())
	if err := r.repo.Storer.SetReference(ref); err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w", branchName, err)
	}
	
	// Get worktree
	w, err := r.repo.Worktree()
	if err != nil {
		return "", fmt.Errorf("failed to get worktree: %w", err)
	}
	
	// Checkout new branch
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: ref.Name(),
		Create: false,
	}); err != nil {
		return "", fmt.Errorf("failed to checkout branch %s: %w", branchName, err)
	}
	
	// Create document directory
	docPath := filepath.Join(r.path, doc.GitPath())
	if err := os.MkdirAll(docPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", docPath, err)
	}
	
	// Write raw content
	if len(doc.Content.Raw) > 0 {
		rawPath := filepath.Join(docPath, "raw")
		if err := os.WriteFile(rawPath, doc.Content.Raw, 0644); err != nil {
			return "", fmt.Errorf("failed to write raw content: %w", err)
		}
	}
	
	// Write extracted text
	if doc.Content.Text != "" {
		textPath := filepath.Join(docPath, "text.txt")
		if err := os.WriteFile(textPath, []byte(doc.Content.Text), 0644); err != nil {
			return "", fmt.Errorf("failed to write text content: %w", err)
		}
	}
	
	// Write metadata as JSON
	metadata := map[string]interface{}{
		"id":         doc.ID,
		"source":     doc.Source,
		"created_at": doc.CreatedAt,
		"updated_at": doc.UpdatedAt,
		"metadata":   doc.Content.Metadata,
	}
	
	if len(doc.Content.Embeddings) > 0 {
		metadata["embedding_dimensions"] = len(doc.Content.Embeddings)
	}
	
	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal metadata: %w", err)
	}
	
	metadataPath := filepath.Join(docPath, "metadata.json")
	if err := os.WriteFile(metadataPath, metadataBytes, 0644); err != nil {
		return "", fmt.Errorf("failed to write metadata: %w", err)
	}
	
	// Write embeddings as binary file if present
	if len(doc.Content.Embeddings) > 0 {
		embeddingPath := filepath.Join(docPath, "embeddings.bin")
		if err := writeEmbeddings(embeddingPath, doc.Content.Embeddings); err != nil {
			return "", fmt.Errorf("failed to write embeddings: %w", err)
		}
	}
	
	// Add all files in document directory
	if _, err := w.Add(doc.GitPath()); err != nil {
		return "", fmt.Errorf("failed to add files: %w", err)
	}
	
	// Create commit
	commit, err := w.Commit(fmt.Sprintf("Ingest document %s", doc.ID), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "CAIA Library",
			Email: "library@caiatech.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to commit: %w", err)
	}
	
	return commit.String(), nil
}

// MergeBranch merges a document branch into main
func (r *Repository) MergeBranch(ctx context.Context, branchName string) error {
	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Checkout main branch
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("main"),
	}); err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	// TODO: Implement actual merge
	// For now, this is a placeholder
	// In production, you'd merge the branch or create a pull request

	return nil
}

// writeEmbeddings writes float32 embeddings to a binary file
func writeEmbeddings(path string, embeddings []float32) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write embeddings as binary data
	for _, val := range embeddings {
		bytes := float32ToBytes(val)
		if _, err := file.Write(bytes); err != nil {
			return err
		}
	}

	return nil
}

// float32ToBytes converts a float32 to 4 bytes using little-endian encoding
func float32ToBytes(f float32) []byte {
	buf := make([]byte, 4)
	binary.LittleEndian.PutUint32(buf, *(*uint32)(unsafe.Pointer(&f)))
	return buf
}