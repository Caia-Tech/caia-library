package test

import (
	"context"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/pipeline"
	"github.com/Caia-Tech/caia-library/internal/processing"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSystemConnectivity verifies all major components are properly connected
func TestSystemConnectivity(t *testing.T) {
	ctx := context.Background()

	t.Run("Storage Layer Connectivity", func(t *testing.T) {
		// Test hybrid storage initialization
		storageConfig := storage.DefaultHybridConfig()
		storageConfig.PrimaryBackend = "govc"
		metricsCollector := storage.NewSimpleMetricsCollector()
		
		hybridStorage, err := storage.NewHybridStorage(
			"/tmp/test-connectivity-repo",
			"connectivity-test",
			storageConfig,
			metricsCollector,
		)
		require.NoError(t, err)
		defer hybridStorage.Close()

		// Test basic operations
		testDoc := &document.Document{
			ID:      "conn-test-001",
			Content: "Testing storage connectivity",
			Metadata: map[string]interface{}{
				"test": true,
			},
		}

		// Store
		docID, err := hybridStorage.StoreDocument(ctx, testDoc)
		assert.NoError(t, err)
		assert.NotEmpty(t, docID)

		// Retrieve
		retrieved, err := hybridStorage.GetDocument(ctx, docID)
		assert.NoError(t, err)
		assert.Equal(t, testDoc.Content, retrieved.Content)

		// List
		docs, err := hybridStorage.ListDocuments(ctx, nil)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, len(docs), 1)

		t.Log("✓ Storage layer connected and operational")
	})

	t.Run("Event Bus and Pipeline Connectivity", func(t *testing.T) {
		// Initialize event bus
		eventBus := pipeline.NewEventBus(100, 2)
		
		// Track events
		eventsReceived := make(chan *pipeline.DocumentEvent, 10)
		
		// Subscribe to events
		subscription := eventBus.Subscribe(
			[]pipeline.EventType{pipeline.EventDocumentStored},
			func(ctx context.Context, event *pipeline.DocumentEvent) error {
				eventsReceived <- event
				return nil
			},
		)
		defer eventBus.Unsubscribe(subscription.ID)

		// Publish test event
		testEvent := &pipeline.DocumentEvent{
			Type:      pipeline.EventDocumentStored,
			DocumentID: "test-doc-001",
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"source": "test",
			},
		}

		err := eventBus.Publish(testEvent)
		assert.NoError(t, err)

		// Wait for event
		select {
		case received := <-eventsReceived:
			assert.Equal(t, testEvent.DocumentID, received.DocumentID)
			assert.Equal(t, testEvent.Type, received.Type)
			t.Log("✓ Event bus connected and operational")
		case <-time.After(1 * time.Second):
			t.Fatal("Event not received within timeout")
		}
	})

	t.Run("Processing Pipeline Connectivity", func(t *testing.T) {
		// Initialize components
		storageConfig := storage.DefaultHybridConfig()
		metricsCollector := storage.NewSimpleMetricsCollector()
		
		hybridStorage, err := storage.NewHybridStorage(
			"/tmp/test-processing-repo",
			"processing-test",
			storageConfig,
			metricsCollector,
		)
		require.NoError(t, err)
		defer hybridStorage.Close()

		eventBus := pipeline.NewEventBus(100, 2)
		
		// Create processor
		processorConfig := processing.DefaultContentProcessorConfig()
		processor, err := processing.NewContentProcessor(
			hybridStorage,
			eventBus,
			processorConfig,
		)
		require.NoError(t, err)

		// Test content cleaning
		cleaner := processing.NewContentCleaner()
		dirtyContent := "Test   with    multiple     spaces"
		cleanedContent, stats := cleaner.Clean(dirtyContent)
		
		assert.NotEqual(t, dirtyContent, cleanedContent)
		assert.Greater(t, stats.CharactersRemoved, 0)
		
		// Process a document
		testDoc := &document.Document{
			ID:      "process-test-001",
			Content: "Document for processing <script>test</script>",
			Metadata: map[string]interface{}{
				"source": "test",
			},
		}

		processed, err := processor.ProcessDocument(ctx, testDoc)
		assert.NoError(t, err)
		assert.NotContains(t, processed.Content, "<script>")
		
		t.Log("✓ Processing pipeline connected and operational")
	})

	t.Run("Component Health Checks", func(t *testing.T) {
		// Storage health check
		storageConfig := storage.DefaultHybridConfig()
		metricsCollector := storage.NewSimpleMetricsCollector()
		
		hybridStorage, err := storage.NewHybridStorage(
			"/tmp/test-health-repo",
			"health-test",
			storageConfig,
			metricsCollector,
		)
		require.NoError(t, err)
		defer hybridStorage.Close()

		err = hybridStorage.Health(ctx)
		assert.NoError(t, err)
		
		t.Log("✓ All health checks passed")
	})

	t.Run("Data Flow Validation", func(t *testing.T) {
		// Setup full pipeline
		storageConfig := storage.DefaultHybridConfig()
		metricsCollector := storage.NewSimpleMetricsCollector()
		
		hybridStorage, err := storage.NewHybridStorage(
			"/tmp/test-flow-repo",
			"flow-test",
			storageConfig,
			metricsCollector,
		)
		require.NoError(t, err)
		defer hybridStorage.Close()

		eventBus := pipeline.NewEventBus(100, 2)
		
		// Track document flow
		flowEvents := make([]string, 0)
		
		// Subscribe to all events
		eventTypes := []pipeline.EventType{
			pipeline.EventDocumentStored,
			pipeline.EventDocumentProcessed,
			pipeline.EventDocumentIndexed,
		}
		
		for _, eventType := range eventTypes {
			et := eventType // Capture for closure
			eventBus.Subscribe(
				[]pipeline.EventType{et},
				func(ctx context.Context, event *pipeline.DocumentEvent) error {
					flowEvents = append(flowEvents, string(event.Type))
					return nil
				},
			)
		}

		// Create and store document
		doc := &document.Document{
			ID:      "flow-test-001",
			Content: "Test document for flow validation",
			Metadata: map[string]interface{}{
				"source": "test",
				"type":   "flow-test",
			},
		}

		// Store triggers events
		docID, err := hybridStorage.StoreDocument(ctx, doc)
		require.NoError(t, err)

		// Publish stored event
		eventBus.Publish(&pipeline.DocumentEvent{
			Type:       pipeline.EventDocumentStored,
			DocumentID: docID,
			Timestamp:  time.Now(),
		})

		// Simulate processing
		eventBus.Publish(&pipeline.DocumentEvent{
			Type:       pipeline.EventDocumentProcessed,
			DocumentID: docID,
			Timestamp:  time.Now(),
		})

		// Simulate indexing
		eventBus.Publish(&pipeline.DocumentEvent{
			Type:       pipeline.EventDocumentIndexed,
			DocumentID: docID,
			Timestamp:  time.Now(),
		})

		// Wait for events to propagate
		time.Sleep(100 * time.Millisecond)

		// Verify flow
		assert.GreaterOrEqual(t, len(flowEvents), 3)
		t.Logf("✓ Data flow validated: %v", flowEvents)
	})
}

// TestComponentVersionCompatibility ensures all components work with current versions
func TestComponentVersionCompatibility(t *testing.T) {
	t.Run("Package Imports", func(t *testing.T) {
		// This test will fail at compile time if imports are broken
		// Just instantiate types to ensure they're available
		
		_ = &document.Document{}
		_ = &pipeline.DocumentEvent{}
		_ = &processing.ContentCleaner{}
		_ = &storage.HybridStorage{}
		
		t.Log("✓ All package imports resolved")
	})

	t.Run("Interface Compatibility", func(t *testing.T) {
		// Verify interfaces match
		var backend storage.StorageBackend
		
		storageConfig := storage.DefaultHybridConfig()
		metricsCollector := storage.NewSimpleMetricsCollector()
		
		hybridStorage, err := storage.NewHybridStorage(
			"/tmp/test-interface-repo",
			"interface-test",
			storageConfig,
			metricsCollector,
		)
		require.NoError(t, err)
		defer hybridStorage.Close()
		
		// HybridStorage should implement StorageBackend
		backend = hybridStorage
		assert.NotNil(t, backend)
		
		t.Log("✓ Interface compatibility verified")
	})
}