package storage

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDocumentIndex tests the document index functionality
func TestDocumentIndex(t *testing.T) {
	// Create govc backend with memory mode
	backend, err := NewGovcBackend("index-test", NewSimpleMetricsCollector())
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Store multiple documents
	numDocs := 10
	docIDs := make([]string, numDocs)
	
	for i := 0; i < numDocs; i++ {
		docID := fmt.Sprintf("index-test-%03d", i)
		docIDs[i] = docID
		
		doc := &document.Document{
			ID: docID,
			Source: document.Source{
				Type: "text",
				URL:  fmt.Sprintf("https://example.com/index-%d.txt", i),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Index test document %d", i),
				Metadata: make(map[string]string),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		_, err := backend.StoreDocument(ctx, doc)
		require.NoError(t, err)
	}
	
	// Verify index size
	stats := backend.GetMemoryStats()
	indexedCount, ok := stats["indexed_documents"].(int)
	assert.True(t, ok)
	assert.Equal(t, numDocs, indexedCount, "Index should contain all stored documents")
	
	// Test retrieval speed
	startTime := time.Now()
	for _, docID := range docIDs {
		doc, err := backend.GetDocument(ctx, docID)
		assert.NoError(t, err)
		assert.NotNil(t, doc)
		assert.Equal(t, docID, doc.ID)
	}
	retrievalTime := time.Since(startTime)
	
	// Calculate average retrieval time
	avgRetrievalTime := retrievalTime / time.Duration(numDocs)
	t.Logf("Average retrieval time with index: %v", avgRetrievalTime)
	
	// Should be very fast (under 1ms per document)
	assert.Less(t, avgRetrievalTime.Microseconds(), int64(1000), "Indexed retrieval should be under 1ms")
}

// TestIndexRebuild tests rebuilding the index from repository
func TestIndexRebuild(t *testing.T) {
	// Create backend and store documents
	backend, err := NewGovcBackend("rebuild-test", NewSimpleMetricsCollector())
	require.NoError(t, err)
	
	ctx := context.Background()
	
	// Store a document
	testDoc := &document.Document{
		ID: "rebuild-test-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/rebuild.txt",
		},
		Content: document.Content{
			Text: "Rebuild test document",
			Metadata: make(map[string]string),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	_, err = backend.StoreDocument(ctx, testDoc)
	require.NoError(t, err)
	
	// Clear the index
	backend.docIndex.Clear()
	assert.Equal(t, 0, backend.docIndex.Size(), "Index should be empty after clear")
	
	// Rebuild index
	err = backend.buildIndex()
	require.NoError(t, err)
	
	// Verify document can still be retrieved
	retrieved, err := backend.GetDocument(ctx, testDoc.ID)
	assert.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, testDoc.ID, retrieved.ID)
	
	// Check index was rebuilt
	stats := backend.GetMemoryStats()
	indexedCount, ok := stats["indexed_documents"].(int)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, indexedCount, 1, "Index should be rebuilt with at least one document")
}

// BenchmarkIndexedRetrieval benchmarks document retrieval with index
func BenchmarkIndexedRetrieval(b *testing.B) {
	backend, err := NewGovcBackend("bench-test", NewSimpleMetricsCollector())
	require.NoError(b, err)
	
	ctx := context.Background()
	
	// Store a test document
	doc := &document.Document{
		ID: "bench-test-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/bench.txt",
		},
		Content: document.Content{
			Text: "Benchmark test document",
			Metadata: make(map[string]string),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	_, err = backend.StoreDocument(ctx, doc)
	require.NoError(b, err)
	
	// Reset timer to exclude setup
	b.ResetTimer()
	
	// Benchmark retrieval
	for i := 0; i < b.N; i++ {
		retrieved, err := backend.GetDocument(ctx, doc.ID)
		if err != nil || retrieved == nil {
			b.Fatal("Failed to retrieve document")
		}
	}
}