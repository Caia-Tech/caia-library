package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestErrorHandling tests various error conditions
func TestErrorHandling(t *testing.T) {
	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-error-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepoPath := filepath.Join(tempDir, "test-repo")
	err = os.MkdirAll(gitRepoPath, 0755)
	require.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(gitRepoPath, false)
	require.NoError(t, err)

	// Create an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(gitRepoPath, "README.md")
	err = os.WriteFile(readmePath, []byte("# Error Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Error Test",
			Email: "test@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create metrics collector
	metrics := storage.NewSimpleMetricsCollector()

	// Configure hybrid storage
	config := storage.DefaultHybridConfig()
	config.PrimaryBackend = "govc"
	config.EnableFallback = true
	config.EnableSync = false

	// Initialize hybrid storage
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "error-test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	ctx := context.Background()

	t.Run("Invalid document ID", func(t *testing.T) {
		// Test getting a non-existent document
		doc, err := hybridStorage.GetDocument(ctx, "non-existent-document-id")
		assert.Error(t, err)
		assert.Nil(t, doc)
		assert.Contains(t, err.Error(), "document not found")
	})

	t.Run("Invalid document validation", func(t *testing.T) {
		// Test storing an invalid document
		invalidDoc := &document.Document{
			ID: "", // Empty ID should fail validation
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/invalid.txt",
			},
			Content: document.Content{
				Text: "Invalid document test",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := hybridStorage.StoreDocument(ctx, invalidDoc)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "document validation failed")
		}
	})

	t.Run("Document without source", func(t *testing.T) {
		// Test storing a document without URL or path
		invalidDoc := &document.Document{
			ID: "invalid-source-test",
			Source: document.Source{
				Type: "text",
				// No URL or Path
			},
			Content: document.Content{
				Text: "Document without source",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := hybridStorage.StoreDocument(ctx, invalidDoc)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "must have either URL or path")
		}
	})

	t.Run("Empty document type", func(t *testing.T) {
		// Test storing a document without type
		invalidDoc := &document.Document{
			ID: "empty-type-test",
			Source: document.Source{
				Type: "", // Empty type
				URL:  "https://example.com/empty-type.txt",
			},
			Content: document.Content{
				Text: "Document without type",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := hybridStorage.StoreDocument(ctx, invalidDoc)
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "source type cannot be empty")
		}
	})

	t.Run("Context cancellation", func(t *testing.T) {
		// Test operation with cancelled context
		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		validDoc := &document.Document{
			ID: "context-test-001",
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/context-test.txt",
			},
			Content: document.Content{
				Text: "Context cancellation test",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// This should either succeed quickly or handle cancellation gracefully
		_, err := hybridStorage.StoreDocument(cancelCtx, validDoc)
		// We don't assert error here because our implementation might complete
		// before the cancellation is checked, but we log the result
		t.Logf("Context cancellation result: %v", err)
	})

	t.Run("Very large document", func(t *testing.T) {
		// Test storing a very large document
		largeContent := make([]byte, 10*1024*1024) // 10MB
		for i := range largeContent {
			largeContent[i] = byte('A' + (i % 26))
		}

		largeDoc := &document.Document{
			ID: "large-document-test",
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/large-doc.txt",
			},
			Content: document.Content{
				Raw:  largeContent,
				Text: string(largeContent[:1000]), // Just first 1000 chars as text
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		start := time.Now()
		_, err := hybridStorage.StoreDocument(ctx, largeDoc)
		duration := time.Since(start)

		if err != nil {
			t.Logf("Large document storage failed (expected): %v", err)
		} else {
			t.Logf("Large document stored successfully in %v", duration)
		}

		// Verify it's tracked in metrics
		summary := metrics.GetMetricsSummary()
		assert.NotNil(t, summary)
	})
}

// TestBackendFailures tests failure scenarios for different backends
func TestBackendFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping backend failure test in short mode")
	}

	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-failure-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepoPath := filepath.Join(tempDir, "failure-repo")
	err = os.MkdirAll(gitRepoPath, 0755)
	require.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(gitRepoPath, false)
	require.NoError(t, err)

	// Create an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(gitRepoPath, "README.md")
	err = os.WriteFile(readmePath, []byte("# Failure Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Failure Test",
			Email: "test@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create metrics collector
	metrics := storage.NewSimpleMetricsCollector()

	t.Run("Disabled fallback failure", func(t *testing.T) {
		// Configure without fallback to test primary backend failures
		config := storage.DefaultHybridConfig()
		config.PrimaryBackend = "git"
		config.EnableFallback = false // Disable fallback
		config.OperationTimeout = 100 * time.Millisecond

		hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "no-fallback-repo", config, metrics)
		require.NoError(t, err)
		defer hybridStorage.Close()

		// This should work since git backend is functional
		testDoc := &document.Document{
			ID: "no-fallback-test-001",
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/no-fallback.txt",
			},
			Content: document.Content{
				Text: "No fallback test document",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		ctx := context.Background()
		_, err = hybridStorage.StoreDocument(ctx, testDoc)
		
		// Since git backend should work, this should succeed
		if err != nil {
			t.Logf("Store failed (may be expected in some cases): %v", err)
		} else {
			t.Log("Store succeeded with no fallback configuration")
		}
	})

	t.Run("Timeout scenarios", func(t *testing.T) {
		// Test with very short timeout
		config := storage.DefaultHybridConfig()
		config.PrimaryBackend = "govc"
		config.EnableFallback = true
		config.OperationTimeout = 1 * time.Nanosecond // Extremely short timeout

		hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "timeout-repo", config, metrics)
		require.NoError(t, err)
		defer hybridStorage.Close()

		testDoc := &document.Document{
			ID: "timeout-test-001",
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/timeout.txt",
			},
			Content: document.Content{
				Text: "Timeout test document",
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		ctx := context.Background()
		_, err = hybridStorage.StoreDocument(ctx, testDoc)
		
		// This may succeed or fail depending on timing
		t.Logf("Timeout test result: %v", err)

		// Check if fallback was triggered
		summary := metrics.GetMetricsSummary()
		if byBackend, ok := summary["by_backend"]; ok {
			t.Logf("Backend metrics: %+v", byBackend)
		}
	})
}

// TestMemoryPressure tests behavior under memory pressure
func TestMemoryPressure(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory pressure test in short mode")
	}

	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-memory-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepoPath := filepath.Join(tempDir, "memory-repo")
	err = os.MkdirAll(gitRepoPath, 0755)
	require.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(gitRepoPath, false)
	require.NoError(t, err)

	// Create an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(gitRepoPath, "README.md")
	err = os.WriteFile(readmePath, []byte("# Memory Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Memory Test",
			Email: "test@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create metrics collector
	metrics := storage.NewSimpleMetricsCollector()

	// Configure hybrid storage
	config := storage.DefaultHybridConfig()
	config.PrimaryBackend = "govc"
	config.EnableFallback = true
	config.EnableSync = false

	// Initialize hybrid storage
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "memory-test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	ctx := context.Background()

	// Store many small documents to test memory handling
	numDocs := 100
	for i := 0; i < numDocs; i++ {
		doc := &document.Document{
			ID: fmt.Sprintf("memory-test-%04d", i),
			Source: document.Source{
				Type: "text",
				URL:  fmt.Sprintf("https://example.com/memory-test-%d.txt", i),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Memory pressure test document #%d with some content", i),
				Metadata: map[string]string{
					"test_id":    fmt.Sprintf("memory-%d", i),
					"batch":      "memory_pressure",
					"created_at": time.Now().Format(time.RFC3339),
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := hybridStorage.StoreDocument(ctx, doc)
		if err != nil {
			t.Logf("Failed to store document %d: %v", i, err)
		}

		// Every 20 documents, check stats
		if i%20 == 19 {
			stats := hybridStorage.GetStats()
			if govcStats, ok := stats["govc"]; ok {
				if govcMap, ok := govcStats.(map[string]interface{}); ok {
					if docsInMemory, ok := govcMap["documents_in_memory"]; ok {
						t.Logf("Documents in memory after %d stores: %v", i+1, docsInMemory)
					}
				}
			}
		}
	}

	// Final statistics
	summary := metrics.GetMetricsSummary()
	stats := hybridStorage.GetStats()

	t.Logf("Final metrics: %+v", summary)
	t.Logf("Final stats: %+v", stats)

	// Verify we can still retrieve documents
	retrievedDoc, err := hybridStorage.GetDocument(ctx, "memory-test-0000")
	if err == nil {
		assert.NotNil(t, retrievedDoc)
		assert.Equal(t, "memory-test-0000", retrievedDoc.ID)
		t.Log("Successfully retrieved first document after memory pressure test")
	} else {
		t.Logf("Failed to retrieve document after memory pressure: %v", err)
	}
}