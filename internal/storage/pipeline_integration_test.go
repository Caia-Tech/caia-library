package storage

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/pipeline"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPipelineIntegration tests that the storage backend publishes events
func TestPipelineIntegration(t *testing.T) {
	// Create govc backend
	backend, err := NewGovcBackend("pipeline-test", NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()
	
	ctx := context.Background()
	
	// Track received events
	var receivedEvents int32
	var lastEvent *pipeline.DocumentEvent
	
	// Subscribe to document added events
	eventBus := backend.GetEventBus()
	require.NotNil(t, eventBus)
	
	handler := func(ctx context.Context, event *pipeline.DocumentEvent) error {
		atomic.AddInt32(&receivedEvents, 1)
		lastEvent = event
		return nil
	}
	
	_, err = eventBus.Subscribe([]pipeline.EventType{pipeline.EventDocumentAdded}, handler, 10)
	require.NoError(t, err)
	
	// Create and store a document
	doc := &document.Document{
		ID: "pipeline-test-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/pipeline-test.txt",
		},
		Content: document.Content{
			Text: "Pipeline integration test document",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	commitHash, err := backend.StoreDocument(ctx, doc)
	require.NoError(t, err)
	require.NotEmpty(t, commitHash)
	
	// Wait for event processing
	time.Sleep(200 * time.Millisecond)
	
	// Verify event was published and received
	assert.Equal(t, int32(1), atomic.LoadInt32(&receivedEvents))
	assert.NotNil(t, lastEvent)
	assert.Equal(t, pipeline.EventDocumentAdded, lastEvent.Type)
	assert.Equal(t, doc.ID, lastEvent.Document.ID)
	assert.Equal(t, commitHash, lastEvent.Metadata["commit_hash"])
	assert.Equal(t, "govc", lastEvent.Metadata["backend"])
	
	// Check event bus stats
	stats := eventBus.GetStats()
	assert.Equal(t, int64(1), stats.EventsPublished)
	assert.GreaterOrEqual(t, stats.EventsDelivered, int64(1))
}

// TestMultipleDocumentEvents tests multiple documents generating events
func TestMultipleDocumentEvents(t *testing.T) {
	backend, err := NewGovcBackend("multi-events-test", NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()
	
	ctx := context.Background()
	eventBus := backend.GetEventBus()
	
	// Track all received events
	var totalEvents int32
	receivedEvents := make([]*pipeline.DocumentEvent, 0)
	
	handler := func(ctx context.Context, event *pipeline.DocumentEvent) error {
		atomic.AddInt32(&totalEvents, 1)
		receivedEvents = append(receivedEvents, event)
		return nil
	}
	
	_, err = eventBus.Subscribe([]pipeline.EventType{pipeline.EventDocumentAdded}, handler, 50)
	require.NoError(t, err)
	
	// Store multiple documents
	numDocs := 5
	for i := 0; i < numDocs; i++ {
		doc := &document.Document{
			ID: fmt.Sprintf("multi-test-%03d", i),
			Source: document.Source{
				Type: "text",
				URL:  fmt.Sprintf("https://example.com/multi-%d.txt", i),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Multi-event test document %d", i),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		
		_, err := backend.StoreDocument(ctx, doc)
		require.NoError(t, err)
	}
	
	// Wait for all events to be processed
	time.Sleep(500 * time.Millisecond)
	
	// Verify all events were received
	assert.Equal(t, int32(numDocs), atomic.LoadInt32(&totalEvents))
	
	// Check that all document IDs were received
	receivedIDs := make(map[string]bool)
	for _, event := range receivedEvents {
		if event != nil && event.Document != nil {
			receivedIDs[event.Document.ID] = true
		}
	}
	
	for i := 0; i < numDocs; i++ {
		expectedID := fmt.Sprintf("multi-test-%03d", i)
		assert.True(t, receivedIDs[expectedID], "Should have received event for document %s", expectedID)
	}
}

// TestPipelinePerformance tests that the event pipeline doesn't slow down storage
func TestPipelinePerformance(t *testing.T) {
	backend, err := NewGovcBackend("perf-pipeline-test", NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()
	
	ctx := context.Background()
	
	// Subscribe with a simple handler
	eventBus := backend.GetEventBus()
	handler := func(ctx context.Context, event *pipeline.DocumentEvent) error {
		// Just count events, don't slow down processing
		return nil
	}
	
	_, err = eventBus.Subscribe([]pipeline.EventType{pipeline.EventDocumentAdded}, handler, 100)
	require.NoError(t, err)
	
	// Measure storage performance with events
	numDocs := 10
	startTime := time.Now()
	
	for i := 0; i < numDocs; i++ {
		doc := &document.Document{
			ID: fmt.Sprintf("perf-pipeline-%03d", i),
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
		
		_, err := backend.StoreDocument(ctx, doc)
		require.NoError(t, err)
	}
	
	totalTime := time.Since(startTime)
	avgTime := totalTime / time.Duration(numDocs)
	
	t.Logf("Storage with pipeline: %d docs in %v (avg: %v per doc)", numDocs, totalTime, avgTime)
	
	// Should still be fast (under 5ms per document on average)
	assert.Less(t, avgTime.Milliseconds(), int64(5), "Storage should remain fast with pipeline")
	
	// Wait for event processing to complete
	time.Sleep(200 * time.Millisecond)
	
	// Check that events were published
	stats := eventBus.GetStats()
	assert.Equal(t, int64(numDocs), stats.EventsPublished)
}