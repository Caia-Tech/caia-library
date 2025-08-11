package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/api"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/activities"
	"github.com/Caia-Tech/caia-library/pkg/document"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAPIEndpoints tests the storage monitoring API endpoints
func TestAPIEndpoints(t *testing.T) {
	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-api-test-*")
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
	err = os.WriteFile(readmePath, []byte("# API Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "API Test",
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
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "api-test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	// Set global storage for activities
	activities.SetGlobalStorage(hybridStorage, metrics)

	// Create storage handler
	storageHandler := api.NewStorageHandler(hybridStorage, metrics)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add API routes
	v1 := app.Group("/api/v1")
	storage := v1.Group("/storage")
	storage.Get("/stats", storageHandler.GetStorageStats)
	storage.Get("/metrics", storageHandler.GetStorageMetrics)
	storage.Get("/health", storageHandler.GetStorageHealth)
	storage.Delete("/metrics", storageHandler.ClearMetrics)

	// Test storage stats endpoint
	t.Run("GET /api/v1/storage/stats", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/storage/stats", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result, "storage_stats")
		
		t.Logf("Storage stats response: %+v", result)
	})

	// Test metrics endpoint
	t.Run("GET /api/v1/storage/metrics", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/storage/metrics", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result, "metrics_summary")
		assert.Contains(t, result, "total_operations")
		
		t.Logf("Metrics response: %+v", result)
	})

	// Test health endpoint
	t.Run("GET /api/v1/storage/health", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/storage/health", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result, "healthy")
		assert.Equal(t, true, result["healthy"])
		
		t.Logf("Health response: %+v", result)
	})

	// Test clear metrics endpoint
	t.Run("DELETE /api/v1/storage/metrics", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/v1/storage/metrics", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result, "message")
		
		t.Logf("Clear metrics response: %+v", result)
	})
}

// TestAPIIntegrationWithStorage tests API endpoints with actual storage operations
func TestAPIIntegrationWithStorage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-api-integration-*")
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
	err = os.WriteFile(readmePath, []byte("# Integration Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Integration Test",
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
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "integration-test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	// Set global storage for activities
	activities.SetGlobalStorage(hybridStorage, metrics)

	// Create storage handler
	storageHandler := api.NewStorageHandler(hybridStorage, metrics)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	// Add API routes
	v1 := app.Group("/api/v1")
	storage := v1.Group("/storage")
	storage.Get("/stats", storageHandler.GetStorageStats)
	storage.Get("/metrics", storageHandler.GetStorageMetrics)
	storage.Get("/health", storageHandler.GetStorageHealth)

	// Perform some storage operations to generate metrics
	testDoc := &document.Document{
		ID: "api-integration-test-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/api-test.txt",
		},
		Content: document.Content{
			Text:     "API integration test document",
			Metadata: map[string]string{"test": "api_integration"},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store document to generate metrics
	_, err = hybridStorage.StoreDocument(context.Background(), testDoc)
	require.NoError(t, err)

	// Get document to generate more metrics
	_, err = hybridStorage.GetDocument(context.Background(), testDoc.ID)
	require.NoError(t, err)

	// Test that metrics now contain our operations
	t.Run("Verify metrics after storage operations", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/storage/metrics", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result, "metrics_summary")
		assert.Contains(t, result, "total_operations")

		// Should have operations recorded
		totalOps := result["total_operations"]
		assert.Greater(t, totalOps, float64(0))

		t.Logf("Operations recorded: %v", totalOps)
		t.Logf("Full metrics: %+v", result)
	})

	// Test stats include our stored document
	t.Run("Verify stats after storage operations", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/storage/stats", nil)
		resp, err := app.Test(req, -1)
		require.NoError(t, err)

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var result map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)

		assert.Contains(t, result, "storage_stats")

		storageStats := result["storage_stats"].(map[string]interface{})
		assert.Contains(t, storageStats, "govc")

		govcStats := storageStats["govc"].(map[string]interface{})
		assert.Contains(t, govcStats, "documents_in_memory")
		
		docsInMemory := govcStats["documents_in_memory"]
		assert.Greater(t, docsInMemory, float64(0))

		t.Logf("Documents in memory: %v", docsInMemory)
		t.Logf("Full stats: %+v", result)
	})
}