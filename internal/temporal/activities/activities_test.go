package activities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer() *httptest.Server {
	mux := http.NewServeMux()
	
	// Test document endpoint
	mux.HandleFunc("/test-document.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Test document content for unit testing"))
	})
	
	// Test PDF endpoint
	mux.HandleFunc("/test-document.pdf", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusOK)
		// Minimal PDF-like content (not a real PDF, but has right header)
		w.Write([]byte("%PDF-1.4\nTest PDF content"))
	})
	
	// 404 endpoint
	mux.HandleFunc("/not-found", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not found"))
	})
	
	// Slow endpoint for timeout testing
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("Slow response"))
	})

	return httptest.NewServer(mux)
}

func TestFetchDocumentActivity(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	tests := []struct {
		name        string
		url         string
		expectError bool
		expectType  string
	}{
		{
			name:        "successful text fetch",
			url:         server.URL + "/test-document.txt",
			expectError: false,
			expectType:  "text/plain",
		},
		{
			name:        "successful PDF fetch",
			url:         server.URL + "/test-document.pdf",
			expectError: false,
			expectType:  "application/pdf",
		},
		{
			name:        "404 error",
			url:         server.URL + "/not-found",
			expectError: true,
		},
		{
			name:        "invalid URL",
			url:         "not-a-valid-url",
			expectError: true,
		},
		{
			name:        "empty URL",
			url:         "",
			expectError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := FetchDocumentActivity(ctx, tt.url)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result.Content)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result.Content)
				assert.Equal(t, tt.expectType, result.ContentType)
			}
		})
	}
}

func TestExtractTextActivity(t *testing.T) {
	tests := []struct {
		name        string
		input       workflows.ExtractInput
		expectError bool
		expectText  string
	}{
		{
			name: "text content",
			input: workflows.ExtractInput{
				Content:     []byte("Simple text content"),
				Type: "text/plain",
			},
			expectError: false,
			expectText:  "Simple text content",
		},
		{
			name: "empty content",
			input: workflows.ExtractInput{
				Content:     []byte{},
				Type: "text/plain",
			},
			expectError: true,
		},
		{
			name: "unsupported content type",
			input: workflows.ExtractInput{
				Content:     []byte("Some content"),
				Type: "application/unknown",
			},
			expectError: true,
		},
		{
			name: "invalid PDF content",
			input: workflows.ExtractInput{
				Content:     []byte("Not a PDF"),
				Type: "application/pdf",
			},
			expectError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractTextActivity(ctx, tt.input)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result.Text)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectText, result.Text)
				assert.NotEmpty(t, result.Metadata)
			}
		})
	}
}

func TestGenerateEmbeddingsActivity(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{
			name:        "normal content",
			content:     []byte("This is test content for embedding generation"),
			expectError: false,
		},
		{
			name:        "empty content",
			content:     []byte{},
			expectError: true,
		},
		{
			name:        "nil content",
			content:     nil,
			expectError: true,
		},
		{
			name:        "very long content",
			content:     make([]byte, 10000), // 10KB of zeros
			expectError: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			embeddings, err := GenerateEmbeddingsActivity(ctx, tt.content)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, embeddings)
			} else {
				assert.NoError(t, err)
				assert.Len(t, embeddings, 384) // Expected embedding dimension
				
				// Check that embeddings are not all zeros (basic sanity check)
				hasNonZero := false
				for _, val := range embeddings {
					if val != 0 {
						hasNonZero = true
						break
					}
				}
				assert.True(t, hasNonZero, "Embeddings should not be all zeros")
			}
		})
	}
}

func TestStoreDocumentActivity(t *testing.T) {
	// Create temporary repository
	tempDir, err := os.MkdirTemp("", "caia-store-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepo, err := git.PlainInit(tempDir, false)
	require.NoError(t, err)

	w, err := gitRepo.Worktree()
	require.NoError(t, err)

	readmeFile := filepath.Join(tempDir, "README.md")
	err = os.WriteFile(readmeFile, []byte("# Test Repository"), 0644)
	require.NoError(t, err)

	_, err = w.Add("README.md")
	require.NoError(t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Set repository path for activity
	originalPath := os.Getenv("CAIA_REPO_PATH")
	os.Setenv("CAIA_REPO_PATH", tempDir)
	defer os.Setenv("CAIA_REPO_PATH", originalPath)

	input := workflows.StoreInput{
		URL:        "http://example.com/test.txt",
		Type:       "text",
		Text:       "Test document content",
		Metadata:   map[string]string{"author": "test"},
		Embeddings: []float32{0.1, 0.2, 0.3, 0.4},
		Content:    []byte("Test document content"),
	}

	ctx := context.Background()
	commitHash, err := StoreDocumentActivity(ctx, input)
	
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)
	
	// Verify commit hash is valid
	// Note: In real implementation, branch name is derived from document ID
	// and ingest branches are created for document storage
}

func TestIndexDocumentActivity(t *testing.T) {
	tests := []struct {
		name        string
		commitHash  string
		expectError bool
	}{
		{
			name:        "valid commit hash",
			commitHash:  "abc123def456",
			expectError: false,
		},
		{
			name:        "empty commit hash",
			commitHash:  "",
			expectError: true,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IndexDocumentActivity(ctx, tt.commitHash)
			
			if tt.expectError {
				assert.Error(t, err)
			} else {
				// For now, this activity is a placeholder, so it should always succeed
				assert.NoError(t, err)
			}
		})
	}
}

func TestMergeBranchActivity(t *testing.T) {
	// Create temporary repository
	tempDir, err := os.MkdirTemp("", "caia-merge-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepo, err := git.PlainInit(tempDir, false)
	require.NoError(t, err)

	w, err := gitRepo.Worktree()
	require.NoError(t, err)

	readmeFile := filepath.Join(tempDir, "README.md")
	err = os.WriteFile(readmeFile, []byte("# Test Repository"), 0644)
	require.NoError(t, err)

	_, err = w.Add("README.md")
	require.NoError(t, err)

	_, err = w.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test",
			Email: "test@example.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Set repository path for activity
	originalPath := os.Getenv("CAIA_REPO_PATH")
	os.Setenv("CAIA_REPO_PATH", tempDir)
	defer os.Setenv("CAIA_REPO_PATH", originalPath)

	ctx := context.Background()
	
	// Test with empty repository (no ingest branches)
	err = MergeBranchActivity(ctx, "dummy-commit-hash")
	assert.NoError(t, err) // Should succeed even with no branches to merge
}

func TestActivities_ErrorHandling(t *testing.T) {
	ctx := context.Background()

	// Test FetchDocumentActivity with timeout
	t.Run("fetch timeout", func(t *testing.T) {
		server := setupTestServer()
		defer server.Close()
		
		// Use a context with a very short timeout
		shortCtx, cancel := context.WithTimeout(ctx, 1*time.Millisecond)
		defer cancel()
		
		_, err := FetchDocumentActivity(shortCtx, server.URL+"/slow")
		assert.Error(t, err)
	})

	// Test ExtractTextActivity with malformed input
	t.Run("extract malformed input", func(t *testing.T) {
		input := workflows.ExtractInput{
			Content: []byte("content"),
			Type:    "", // Missing content type
		}
		
		_, err := ExtractTextActivity(ctx, input)
		assert.Error(t, err)
	})

	// Test StoreDocumentActivity with invalid repository path
	t.Run("store invalid repo", func(t *testing.T) {
		originalPath := os.Getenv("CAIA_REPO_PATH")
		os.Setenv("CAIA_REPO_PATH", "/non/existent/path")
		defer os.Setenv("CAIA_REPO_PATH", originalPath)

		input := workflows.StoreInput{
			URL:  "http://example.com/test.txt",
			Type: "text",
			Text: "Test content",
		}
		
		_, err := StoreDocumentActivity(ctx, input)
		assert.Error(t, err)
	})
}

func BenchmarkFetchDocumentActivity(b *testing.B) {
	server := setupTestServer()
	defer server.Close()

	ctx := context.Background()
	url := server.URL + "/test-document.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := FetchDocumentActivity(ctx, url)
		if err != nil {
			b.Fatalf("Failed to fetch document: %v", err)
		}
	}
}

func BenchmarkGenerateEmbeddingsActivity(b *testing.B) {
	content := []byte("This is benchmark content for embedding generation testing")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateEmbeddingsActivity(ctx, content)
		if err != nil {
			b.Fatalf("Failed to generate embeddings: %v", err)
		}
	}
}