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

	"github.com/Caia-Tech/caia-library/pkg/document"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/rs/zerolog/log"
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

// GetRepo returns the underlying go-git repository for advanced operations
func (r *Repository) GetRepo() *git.Repository {
	return r.repo
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

// MergeBranch merges a document branch into main
func (r *Repository) MergeBranch(ctx context.Context, branchName string) error {
	logger := log.With().Str("branch", branchName).Logger()
	logger.Info().Msg("Starting merge")

	w, err := r.repo.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Get the branch reference
	branchRef, err := r.repo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	if err != nil {
		return fmt.Errorf("failed to get branch reference %s: %w", branchName, err)
	}

	// Check if main branch exists
	mainBranchRef := plumbing.NewBranchReferenceName("main")
	mainRef, err := r.repo.Reference(mainBranchRef, true)
	if err != nil {
		// Main doesn't exist, create it from the branch
		logger.Info().Msg("Main branch doesn't exist, creating from branch")
		
		// Create main branch pointing to branch commit
		newMainRef := plumbing.NewHashReference(mainBranchRef, branchRef.Hash())
		if err := r.repo.Storer.SetReference(newMainRef); err != nil {
			return fmt.Errorf("failed to create main branch: %w", err)
		}
		
		// Checkout main
		if err := w.Checkout(&git.CheckoutOptions{
			Branch: mainBranchRef,
			Force:  true,
		}); err != nil {
			return fmt.Errorf("failed to checkout new main: %w", err)
		}
		
		logger.Info().Msg("Created main branch from branch")
		return nil
	}

	// Get commits
	branchCommit, err := r.repo.CommitObject(branchRef.Hash())
	if err != nil {
		return fmt.Errorf("failed to get branch commit: %w", err)
	}

	mainCommit, err := r.repo.CommitObject(mainRef.Hash())
	if err != nil {
		return fmt.Errorf("failed to get main commit: %w", err)
	}

	// Check if already merged
	if branchCommit.Hash == mainCommit.Hash {
		logger.Info().Msg("Branch already merged")
		return nil
	}

	// Check if we can fast-forward
	isAncestor, err := r.isAncestor(mainCommit.Hash, branchCommit.Hash)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to check ancestry, attempting merge anyway")
		isAncestor = false
	}

	if isAncestor {
		// Fast-forward merge
		logger.Info().Msg("Performing fast-forward merge")
		
		// Update main to point to branch commit
		newMainRef := plumbing.NewHashReference(mainBranchRef, branchRef.Hash())
		if err := r.repo.Storer.SetReference(newMainRef); err != nil {
			return fmt.Errorf("failed to update main ref: %w", err)
		}
		
		// Checkout to update working tree
		if err := w.Checkout(&git.CheckoutOptions{
			Branch: mainBranchRef,
			Force:  true,
		}); err != nil {
			return fmt.Errorf("failed to checkout after fast-forward: %w", err)
		}
		
		logger.Info().Str("commit", branchCommit.Hash.String()).Msg("Fast-forward completed")
		return nil
	}

	// Real merge needed - checkout main first
	if err := w.Checkout(&git.CheckoutOptions{
		Branch: mainBranchRef,
	}); err != nil {
		return fmt.Errorf("failed to checkout main: %w", err)
	}

	// Get trees
	mainTree, err := mainCommit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get main tree: %w", err)
	}

	branchTree, err := branchCommit.Tree()
	if err != nil {
		return fmt.Errorf("failed to get branch tree: %w", err)
	}

	// Merge by copying new/modified files from branch
	hasChanges := false
	iter := branchTree.Files()

	err = iter.ForEach(func(branchFile *object.File) error {
		// Check if file exists in main
		mainFile, err := mainTree.File(branchFile.Name)
		if err != nil || mainFile.Hash != branchFile.Hash {
			// File is new or modified
			hasChanges = true
			
			// Get content
			content, err := branchFile.Contents()
			if err != nil {
				return fmt.Errorf("failed to get content of %s: %w", branchFile.Name, err)
			}
			
			// Write file
			filePath := filepath.Join(r.path, branchFile.Name)
			if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			
			if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			
			// Stage file
			if _, err := w.Add(branchFile.Name); err != nil {
				return fmt.Errorf("failed to stage file: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to merge files: %w", err)
	}

	if !hasChanges {
		logger.Info().Msg("No changes to merge")
		return nil
	}

	// Create merge commit
	mergeCommit, err := w.Commit(fmt.Sprintf("Merge branch '%s' into main", branchName), &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Caia Library",
			Email: "library@caiatech.com",
			When:  time.Now(),
		},
		Parents: []plumbing.Hash{mainCommit.Hash, branchCommit.Hash},
	})
	if err != nil {
		return fmt.Errorf("failed to create merge commit: %w", err)
	}

	logger.Info().
		Str("commit", mergeCommit.String()).
		Str("branch_commit", branchCommit.Hash.String()).
		Str("main_commit", mainCommit.Hash.String()).
		Msg("Merge completed")

	return nil
}

// isAncestor checks if ancestor is an ancestor of descendant
func (r *Repository) isAncestor(ancestor, descendant plumbing.Hash) (bool, error) {
	iter, err := r.repo.Log(&git.LogOptions{From: descendant})
	if err != nil {
		return false, err
	}
	
	found := false
	err = iter.ForEach(func(c *object.Commit) error {
		if c.Hash == ancestor {
			found = true
			return storer.ErrStop
		}
		return nil
	})
	if err != nil && err != storer.ErrStop {
		return false, err
	}
	
	return found, nil
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