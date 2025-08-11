package processing

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/pipeline"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContentProcessorIntegration(t *testing.T) {
	// Create govc backend with event bus
	backend, err := storage.NewGovcBackend("integration-test", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create content processor with custom config
	config := &ContentProcessorConfig{
		Enabled:             true,
		ProcessInBackground: true,
		BatchSize:           5,
		ProcessingTimeout:   10 * time.Second,
		StrictMode:          false,
		PreserveStructure:   true,
	}

	processor, err := NewContentProcessor(backend, backend.GetEventBus(), config)
	require.NoError(t, err)
	defer processor.Close()

	// Channel to receive cleaning events
	cleaningEvents := make(chan *pipeline.DocumentEvent, 10)
	var wg sync.WaitGroup

	// Subscribe to document cleaning events
	handler := func(ctx context.Context, event *pipeline.DocumentEvent) error {
		if event.Type == pipeline.EventDocumentCleaned {
			cleaningEvents <- event
		}
		return nil
	}

	_, err = backend.GetEventBus().Subscribe(
		[]pipeline.EventType{pipeline.EventDocumentCleaned},
		handler,
		10,
	)
	require.NoError(t, err)

	// Store documents with various content issues
	testDocs := []*document.Document{
		{
			ID: "integration-test-001",
			Source: document.Source{
				Type: "html",
				URL:  "https://example.com/dirty1.html",
			},
			Content: document.Content{
				Text: `<html><body>
					<p>This  has   excessive    spaces   and HTML tags.</p>
					<p>Visit https://example.com or email test@example.com</p>
					<p>Numbers like 3.14159265359 and punctuation!!!!!</p>
					<p>Encoding problems: â€™smart quotesâ€ everywhere</p>
				</body></html>`,
				Metadata: make(map[string]string),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID: "integration-test-002",
			Source: document.Source{
				Type: "text",
				URL:  "https://example.com/dirty2.txt",
			},
			Content: document.Content{
				Text: `  Multiple   whitespace   issues    here.

Visit www.google.com and ftp://files.example.com

Contact person@domain.org for details.

Same line repeated
Same line repeated
Different line
Different line`,
				Metadata: make(map[string]string),
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	// Store documents (should trigger automatic cleaning)
	for _, doc := range testDocs {
		wg.Add(1)
		go func(d *document.Document) {
			defer wg.Done()
			_, err := backend.StoreDocument(context.Background(), d)
			require.NoError(t, err)
		}(doc)
	}

	// Wait for storage operations
	wg.Wait()

	// Wait for cleaning events (with timeout)
	cleanedDocs := make([]*pipeline.DocumentEvent, 0)
	timeout := time.After(5 * time.Second)

	for len(cleanedDocs) < len(testDocs) {
		select {
		case event := <-cleaningEvents:
			cleanedDocs = append(cleanedDocs, event)
		case <-timeout:
			t.Fatalf("Timeout waiting for cleaning events. Got %d/%d", len(cleanedDocs), len(testDocs))
		}
	}

	// Verify cleaning events
	assert.Len(t, cleanedDocs, len(testDocs))

	for _, event := range cleanedDocs {
		assert.Equal(t, pipeline.EventDocumentCleaned, event.Type)
		assert.NotNil(t, event.Document)
		
		// Check metadata contains cleaning info
		assert.Contains(t, event.Metadata, "original_length")
		assert.Contains(t, event.Metadata, "cleaned_length")
		assert.Contains(t, event.Metadata, "bytes_removed")
		assert.Contains(t, event.Metadata, "rules_applied")
		
		// Original length should be greater than cleaned length
		originalLen := event.Metadata["original_length"].(int)
		cleanedLen := event.Metadata["cleaned_length"].(int)
		assert.Greater(t, originalLen, cleanedLen, "Content should be shorter after cleaning")
		
		// Verify document content was actually cleaned
		doc := event.Document
		content := doc.Content.Text
		
		// Should not contain HTML tags
		assert.NotContains(t, content, "<html>")
		assert.NotContains(t, content, "<p>")
		assert.NotContains(t, content, "<body>")
		
		// URLs should be replaced
		assert.NotContains(t, content, "https://example.com")
		assert.NotContains(t, content, "www.google.com")
		assert.Contains(t, content, "[URL]")
		
		// Emails should be replaced
		assert.NotContains(t, content, "@example.com")
		assert.NotContains(t, content, "@domain.org")
		assert.Contains(t, content, "[EMAIL]")
		
		// Excessive punctuation should be reduced
		assert.NotContains(t, content, "!!!!!")
		
		// Encoding issues should be fixed
		assert.NotContains(t, content, "â€™")
		assert.NotContains(t, content, "â€œ")
		assert.NotContains(t, content, "â€")
		
		// Document metadata should indicate cleaning
		assert.Equal(t, "true", doc.Content.Metadata["cleaned"])
		assert.NotEmpty(t, doc.Content.Metadata["cleaned_at"])
		assert.NotEmpty(t, doc.Content.Metadata["rules_applied"])
	}

	// Check processor statistics
	stats := processor.GetStats()
	assert.GreaterOrEqual(t, stats.DocumentsProcessed, int64(len(testDocs)))
	assert.Equal(t, int64(0), stats.DocumentsFailed)
	assert.Greater(t, stats.TotalBytesProcessed, int64(0))
	assert.Greater(t, stats.TotalBytesRemoved, int64(0))
	assert.Greater(t, stats.AverageProcessTime, time.Duration(0))

	t.Logf("Integration test results:")
	t.Logf("  Documents processed: %d", stats.DocumentsProcessed)
	t.Logf("  Documents failed: %d", stats.DocumentsFailed)
	t.Logf("  Total bytes processed: %d", stats.TotalBytesProcessed)
	t.Logf("  Total bytes removed: %d", stats.TotalBytesRemoved)
	t.Logf("  Average processing time: %v", stats.AverageProcessTime)
}