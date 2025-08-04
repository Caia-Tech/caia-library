package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRepo(t *testing.T) (*Repository, string) {
	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "caia-test-repo-*")
	require.NoError(t, err)

	// Initialize Git repository
	gitRepo, err := git.PlainInit(tmpDir, false)
	require.NoError(t, err)

	// Create initial commit
	w, err := gitRepo.Worktree()
	require.NoError(t, err)

	// Create initial file
	readmeFile := filepath.Join(tmpDir, "README.md")
	err = os.WriteFile(readmeFile, []byte("# Caia Library Repository"), 0644)
	require.NoError(t, err)

	_, err = w.Add("README.md")
	require.NoError(t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Caia Library",
			Email: "library@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Open the repository
	repo, err := NewRepository(tmpDir)
	require.NoError(t, err)

	return repo, tmpDir
}

func TestNewRepository(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		expectError bool
	}{
		{
			name:        "empty path",
			path:        "",
			expectError: true,
		},
		{
			name:        "non-existent path",
			path:        "/non/existent/path",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, err := NewRepository(tt.path)
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, repo)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repo)
			}
		})
	}
}

func TestRepository_StoreDocument(t *testing.T) {
	repo, tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Create test document
	doc := &document.Document{
		ID: "test-doc-123",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/test.txt",
		},
		Content: document.Content{
			Raw:        []byte("This is a test document for Caia Library"),
			Text:       "This is a test document for Caia Library",
			Metadata:   map[string]string{"author": "Test Author"},
			Embeddings: []float32{0.1, 0.2, 0.3, 0.4},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store document
	ctx := context.Background()
	commitHash, err := repo.StoreDocument(ctx, doc)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Verify ingest branch was created
	gitRepo := repo.GetRepo()
	branchName := "ingest/" + doc.ID
	branchRef, err := gitRepo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	assert.NoError(t, err)
	assert.NotNil(t, branchRef)

	// Verify files were created in the correct location
	docPath := filepath.Join(tmpDir, doc.GitPath())
	assert.FileExists(t, filepath.Join(docPath, "raw"))
	assert.FileExists(t, filepath.Join(docPath, "text.txt"))
	assert.FileExists(t, filepath.Join(docPath, "metadata.json"))
	assert.FileExists(t, filepath.Join(docPath, "embeddings.bin"))

	// Verify content
	textContent, err := os.ReadFile(filepath.Join(docPath, "text.txt"))
	assert.NoError(t, err)
	assert.Equal(t, doc.Content.Text, string(textContent))

	rawContent, err := os.ReadFile(filepath.Join(docPath, "raw"))
	assert.NoError(t, err)
	assert.Equal(t, doc.Content.Raw, rawContent)
}

func TestRepository_StoreDocument_InvalidDocument(t *testing.T) {
	repo, tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name string
		doc  *document.Document
	}{
		{
			name: "empty ID",
			doc: &document.Document{
				ID: "",
				Source: document.Source{
					Type: "text",
					URL:  "http://example.com/test.txt",
				},
			},
		},
		{
			name: "empty source type",
			doc: &document.Document{
				ID: "test-123",
				Source: document.Source{
					Type: "",
					URL:  "http://example.com/test.txt",
				},
			},
		},
		{
			name: "no URL or path",
			doc: &document.Document{
				ID: "test-123",
				Source: document.Source{
					Type: "text",
				},
			},
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			commitHash, err := repo.StoreDocument(ctx, tt.doc)
			assert.Error(t, err)
			assert.Empty(t, commitHash)
		})
	}
}

func TestRepository_MergeBranch(t *testing.T) {
	repo, tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Store a document (creates ingest branch)
	doc := &document.Document{
		ID: "test-merge-doc",
		Source: document.Source{
			Type: "text",
			URL:  "http://example.com/merge-test.txt",
		},
		Content: document.Content{
			Raw:  []byte("Merge test content"),
			Text: "Merge test content",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	_, err := repo.StoreDocument(ctx, doc)
	require.NoError(t, err)

	// Verify branch exists
	branchName := "ingest/" + doc.ID
	gitRepo := repo.GetRepo()
	branchRef, err := gitRepo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	require.NoError(t, err)
	assert.NotNil(t, branchRef)

	// Test merge
	err = repo.MergeBranch(ctx, branchName)
	assert.NoError(t, err)

	// Verify files are now in main branch
	docPath := filepath.Join(tmpDir, doc.GitPath())
	assert.FileExists(t, filepath.Join(docPath, "text.txt"))
	
	// Verify we're on main branch and content is accessible
	head, err := gitRepo.Head()
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/main", head.Name().String())
	
	textContent, err := os.ReadFile(filepath.Join(docPath, "text.txt"))
	assert.NoError(t, err)
	assert.Equal(t, "Merge test content", string(textContent))
}

func TestRepository_MergeBranch_NonExistentBranch(t *testing.T) {
	repo, tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()
	err := repo.MergeBranch(ctx, "ingest/non-existent-branch")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "reference not found")
}

func TestDocument_GitPath(t *testing.T) {
	tests := []struct {
		name     string
		doc      *document.Document
		expected string
	}{
		{
			name: "text document",
			doc: &document.Document{
				ID: "test-doc-123",
				Source: document.Source{Type: "text"},
				CreatedAt: time.Date(2025, 8, 4, 12, 0, 0, 0, time.UTC),
			},
			expected: "documents/text/2025/08/test-doc-123",
		},
		{
			name: "pdf document",
			doc: &document.Document{
				ID: "pdf-doc-456",
				Source: document.Source{Type: "pdf"},
				CreatedAt: time.Date(2024, 12, 25, 10, 30, 0, 0, time.UTC),
			},
			expected: "documents/pdf/2024/12/pdf-doc-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.doc.GitPath()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteEmbeddings(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "caia-embeddings-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	embeddingFile := filepath.Join(tempDir, "test_embeddings.bin")
	testEmbeddings := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

	err = writeEmbeddings(embeddingFile, testEmbeddings)
	assert.NoError(t, err)

	// Verify file exists and has expected size
	stat, err := os.Stat(embeddingFile)
	assert.NoError(t, err)
	expectedSize := int64(len(testEmbeddings) * 4) // 4 bytes per float32
	assert.Equal(t, expectedSize, stat.Size())

	// Verify file content
	content, err := os.ReadFile(embeddingFile)
	assert.NoError(t, err)
	assert.Len(t, content, len(testEmbeddings)*4)
}

func TestRepository_ConcurrentWrites(t *testing.T) {
	repo, tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Test concurrent document writes
	ctx := context.Background()
	errChan := make(chan error, 3)
	numDocs := 3

	for i := 0; i < numDocs; i++ {
		go func(index int) {
			doc := &document.Document{
				ID: fmt.Sprintf("concurrent-doc-%d", index),
				Source: document.Source{
					Type: "text",
					URL:  fmt.Sprintf("https://example.com/doc%d.txt", index),
				},
				Content: document.Content{
					Raw:  []byte(fmt.Sprintf("Concurrent document content %d", index)),
					Text: fmt.Sprintf("Concurrent document %d", index),
					Metadata: map[string]string{
						"index": fmt.Sprintf("%d", index),
					},
				},
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			_, err := repo.StoreDocument(ctx, doc)
			errChan <- err
		}(i)
	}

	// Collect errors
	for i := 0; i < numDocs; i++ {
		err := <-errChan
		assert.NoError(t, err)
	}

	// Verify all documents were stored with correct branch structure
	gitRepo := repo.GetRepo()
	for i := 0; i < numDocs; i++ {
		branchName := fmt.Sprintf("ingest/concurrent-doc-%d", i)
		branchRef, err := gitRepo.Reference(plumbing.NewBranchReferenceName(branchName), true)
		assert.NoError(t, err, "Branch %s should exist", branchName)
		assert.NotNil(t, branchRef)
	}
}

func TestRepository_StoreAndMerge_EndToEnd(t *testing.T) {
	repo, tmpDir := setupTestRepo(t)
	defer os.RemoveAll(tmpDir)

	// Test complete workflow: store -> merge -> verify
	doc := &document.Document{
		ID: "end-to-end-doc",
		Source: document.Source{
			Type: "pdf",
			URL:  "https://arxiv.org/pdf/test.pdf",
		},
		Content: document.Content{
			Raw:        []byte("End-to-end test PDF content"),
			Text:       "End-to-end test content",
			Metadata:   map[string]string{"source": "arXiv", "attribution": "Caia Tech"},
			Embeddings: make([]float32, 384), // Standard embedding size
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	ctx := context.Background()
	
	// Step 1: Store document
	commitHash, err := repo.StoreDocument(ctx, doc)
	require.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Step 2: Verify ingest branch exists
	branchName := "ingest/" + doc.ID
	gitRepo := repo.GetRepo()
	branchRef, err := gitRepo.Reference(plumbing.NewBranchReferenceName(branchName), true)
	require.NoError(t, err)
	assert.NotNil(t, branchRef)

	// Step 3: Merge to main
	err = repo.MergeBranch(ctx, branchName)
	require.NoError(t, err)

	// Step 4: Verify document is accessible from main
	docPath := filepath.Join(tmpDir, doc.GitPath())
	assert.FileExists(t, filepath.Join(docPath, "raw"))
	assert.FileExists(t, filepath.Join(docPath, "text.txt"))
	assert.FileExists(t, filepath.Join(docPath, "metadata.json"))
	assert.FileExists(t, filepath.Join(docPath, "embeddings.bin"))

	// Step 5: Verify content integrity
	textContent, err := os.ReadFile(filepath.Join(docPath, "text.txt"))
	require.NoError(t, err)
	assert.Equal(t, doc.Content.Text, string(textContent))

	rawContent, err := os.ReadFile(filepath.Join(docPath, "raw"))
	require.NoError(t, err)
	assert.Equal(t, doc.Content.Raw, rawContent)

	// Step 6: Verify we're on main branch
	head, err := gitRepo.Head()
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/main", head.Name().String())
}

func BenchmarkStoreDocument(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "caia-bench-repo-*")
	require.NoError(b, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepo, err := git.PlainInit(tempDir, false)
	require.NoError(b, err)

	w, err := gitRepo.Worktree()
	require.NoError(b, err)

	readmeFile := filepath.Join(tempDir, "README.md")
	err = os.WriteFile(readmeFile, []byte("# Benchmark Repository"), 0644)
	require.NoError(b, err)

	_, err = w.Add("README.md")
	require.NoError(b, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Benchmark",
			Email: "bench@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(b, err)

	repo, err := NewRepository(tempDir)
	require.NoError(b, err)

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testDoc := &document.Document{
			ID: fmt.Sprintf("bench-doc-%d", i),
			Source: document.Source{
				Type: "text",
				URL:  fmt.Sprintf("http://example.com/bench-%d.txt", i),
			},
			Content: document.Content{
				Raw:        []byte("Benchmark document content"),
				Text:       "Benchmark document content",
				Metadata:   map[string]string{"iteration": fmt.Sprintf("%d", i)},
				Embeddings: make([]float32, 384), // Typical embedding size
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := repo.StoreDocument(ctx, testDoc)
		if err != nil {
			b.Fatalf("Failed to store document: %v", err)
		}
	}
}