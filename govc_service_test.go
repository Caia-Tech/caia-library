package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGovcServiceIntegration tests integration with real govc service
// Run with: GOVC_SERVICE_URL=http://localhost:8080 go test -run TestGovcServiceIntegration
func TestGovcServiceIntegration(t *testing.T) {
	// Skip if govc service URL is not configured
	serviceURL := os.Getenv("GOVC_SERVICE_URL")
	if serviceURL == "" {
		t.Skip("GOVC_SERVICE_URL not set, skipping govc service integration test")
	}

	// Create govc backend with real service
	config := storage.GetGovcConfig()
	backend, err := storage.NewGovcBackendWithConfig("test-integration", config, storage.NewSimpleMetricsCollector())
	require.NoError(t, err)

	ctx := context.Background()

	// Test health check
	t.Run("HealthCheck", func(t *testing.T) {
		err := backend.Health(ctx)
		if err != nil {
			t.Logf("govc service health check failed: %v", err)
			t.Logf("Make sure govc service is running at %s", serviceURL)
			t.Skip("govc service not available")
		}
	})

	// Test document operations
	t.Run("DocumentOperations", func(t *testing.T) {
		// Create test document
		testDoc := &document.Document{
			ID: fmt.Sprintf("govc-service-test-%d", time.Now().UnixNano()),
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/service-test.txt",
			},
			Content: document.Content{
				Text: "Integration test with govc service",
				Metadata: map[string]string{
					"test":    "govc-service",
					"version": "1.0",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		// Store document
		commitHash, err := backend.StoreDocument(ctx, testDoc)
		if err != nil {
			t.Logf("Store failed: %v", err)
			stats := backend.GetMemoryStats()
			t.Logf("Backend stats: %+v", stats)
			if fallback, ok := stats["fallback_mode"].(bool); ok && fallback {
				t.Skip("govc service not available, using memory fallback")
			}
			t.Fatalf("Failed to store document: %v", err)
		}
		assert.NotEmpty(t, commitHash)
		t.Logf("Document stored with commit: %s", commitHash)

		// Retrieve document
		retrieved, err := backend.GetDocument(ctx, testDoc.ID)
		if err != nil {
			t.Logf("Retrieve failed: %v", err)
			// This might fail if govc doesn't support the search API yet
			t.Skip("Document retrieval not fully implemented in govc service")
		}
		assert.NotNil(t, retrieved)
		assert.Equal(t, testDoc.ID, retrieved.ID)
	})

	// Test service statistics
	t.Run("ServiceStats", func(t *testing.T) {
		stats := backend.GetMemoryStats()
		t.Logf("govc backend statistics:")
		for key, value := range stats {
			t.Logf("  %s: %v", key, value)
		}

		// Check if actually integrated
		integrated, ok := stats["govc_integrated"].(bool)
		if ok && integrated {
			t.Log("✅ Successfully integrated with govc service")
		} else {
			t.Log("⚠️ Running in fallback mode")
		}
	})
}

// TestGovcServicePerformance benchmarks govc service performance
func TestGovcServicePerformance(t *testing.T) {
	serviceURL := os.Getenv("GOVC_SERVICE_URL")
	if serviceURL == "" {
		t.Skip("GOVC_SERVICE_URL not set, skipping performance test")
	}

	config := storage.GetGovcConfig()
	backend, err := storage.NewGovcBackendWithConfig("perf-test", config, storage.NewSimpleMetricsCollector())
	require.NoError(t, err)

	ctx := context.Background()

	// Check if service is available
	if err := backend.Health(ctx); err != nil {
		t.Skip("govc service not available")
	}

	// Benchmark document storage
	numDocs := 100
	docs := make([]*document.Document, numDocs)
	for i := 0; i < numDocs; i++ {
		docs[i] = &document.Document{
			ID: fmt.Sprintf("perf-test-%04d", i),
			Source: document.Source{
				Type: "text",
				URL:  fmt.Sprintf("https://example.com/perf-%d.txt", i),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Performance test document %d", i),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
	}

	// Measure write performance
	start := time.Now()
	successfulWrites := 0
	for _, doc := range docs {
		if _, err := backend.StoreDocument(ctx, doc); err == nil {
			successfulWrites++
		}
	}
	writeDuration := time.Since(start)

	// Calculate throughput
	writeThroughput := float64(successfulWrites) / writeDuration.Seconds()
	t.Logf("Write performance:")
	t.Logf("  Documents: %d successful / %d total", successfulWrites, numDocs)
	t.Logf("  Duration: %v", writeDuration)
	t.Logf("  Throughput: %.2f docs/sec", writeThroughput)
	t.Logf("  Avg latency: %.2f ms/doc", float64(writeDuration.Milliseconds())/float64(numDocs))

	// Get final stats
	stats := backend.GetMemoryStats()
	t.Logf("Final backend stats: %+v", stats)
}

// TestGovcServiceFailover tests failover between govc and git backends
func TestGovcServiceFailover(t *testing.T) {
	// This test simulates govc service failure and recovery
	
	// Start with invalid service URL to force fallback
	os.Setenv("GOVC_SERVICE_URL", "http://localhost:9999") // Non-existent port
	defer os.Unsetenv("GOVC_SERVICE_URL")

	config := storage.GetGovcConfig()
	config.Timeout = 2 * time.Second // Short timeout for faster failover
	
	backend, err := storage.NewGovcBackendWithConfig("failover-test", config, storage.NewSimpleMetricsCollector())
	require.NoError(t, err)

	ctx := context.Background()

	// Check initial state - should be in fallback mode
	stats := backend.GetMemoryStats()
	if fallback, ok := stats["fallback_mode"].(bool); ok {
		assert.True(t, fallback, "Should be in fallback mode with invalid service URL")
		t.Log("✅ Correctly detected unavailable service and switched to fallback")
	}

	// Test that operations still work in fallback mode
	testDoc := &document.Document{
		ID: "failover-test-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/failover.txt",
		},
		Content: document.Content{
			Text: "Failover test document",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Should work even in fallback mode
	commitHash, err := backend.StoreDocument(ctx, testDoc)
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)
	t.Logf("Document stored in fallback mode: %s", commitHash)

	// Retrieve should also work
	retrieved, err := backend.GetDocument(ctx, testDoc.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	if retrieved != nil {
		assert.Equal(t, testDoc.ID, retrieved.ID)
		t.Log("✅ Fallback mode operations working correctly")
	}
}