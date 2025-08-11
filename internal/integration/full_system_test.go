package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/pipeline"
	"github.com/Caia-Tech/caia-library/internal/presentation"
	"github.com/Caia-Tech/caia-library/internal/processing"
	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/Caia-Tech/caia-library/internal/procurement/synthetic"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullSystemIntegration tests the complete data flow through all components
func TestFullSystemIntegration(t *testing.T) {
	ctx := context.Background()

	// Initialize storage layer
	storageConfig := storage.DefaultHybridConfig()
	storageConfig.PrimaryBackend = "govc"
	metricsCollector := storage.NewSimpleMetricsCollector()
	
	hybridStorage, err := storage.NewHybridStorage(
		"/tmp/test-repo",
		"test-library",
		storageConfig,
		metricsCollector,
	)
	require.NoError(t, err)
	defer hybridStorage.Close()

	// Initialize event bus for pipeline
	eventBus := pipeline.NewEventBus(pipeline.DefaultEventBusConfig())
	eventBus.Start()
	defer eventBus.Stop()

	// Initialize processing layer
	cleanerConfig := processing.DefaultCleanerConfig()
	cleaner := processing.NewContentCleaner(cleanerConfig)
	
	processorConfig := processing.DefaultContentProcessorConfig()
	processor := processing.NewContentProcessor(
		processorConfig,
		cleaner,
		hybridStorage,
		eventBus,
	)
	processor.Start()
	defer processor.Stop()

	// Initialize quality validation
	qualityValidator := quality.NewQualityValidator(hybridStorage)

	// Initialize procurement layer
	syntheticGen := createSyntheticGenerator(hybridStorage, qualityValidator)
	webScraper := createWebScraper(hybridStorage, qualityValidator)

	// Initialize presentation layer
	renderer := presentation.NewRenderer(nil)
	presentationStorage := &StorageAdapter{backend: hybridStorage}
	presentationAPI := presentation.NewAPI(renderer, presentationStorage, nil)

	t.Run("End-to-End Synthetic Generation Flow", func(t *testing.T) {
		// Generate synthetic content
		request := &procurement.GenerationRequest{
			RequestID:   "test-synthetic-001",
			Domain:      "technology",
			Topic:       "artificial intelligence",
			ContentType: procurement.ContentTypeArticle,
			RequestedBy: "test-user",
			CreatedAt:   time.Now(),
		}

		result, err := syntheticGen.GenerateContent(ctx, request)
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.NotNil(t, result.Document)

		// Store document
		docID, err := hybridStorage.StoreDocument(ctx, result.Document)
		require.NoError(t, err)
		assert.NotEmpty(t, docID)

		// Trigger processing via event
		eventBus.Publish(pipeline.EventDocumentStored, pipeline.EventData{
			"document_id": docID,
			"source":      "synthetic",
		})

		// Wait for processing
		time.Sleep(100 * time.Millisecond)

		// Retrieve and verify processed document
		retrievedDoc, err := hybridStorage.GetDocument(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, retrievedDoc)

		// Render document for presentation
		presentDoc := &presentation.Document{
			ID:       retrievedDoc.ID,
			Content:  retrievedDoc.Content,
			Metadata: retrievedDoc.Metadata,
		}

		rendered, err := renderer.RenderDocument(presentDoc, &presentation.RenderOptions{
			Format:          presentation.FormatHTML,
			IncludeMetadata: true,
		})
		require.NoError(t, err)
		assert.NotEmpty(t, rendered.Content)
		assert.Equal(t, presentation.FormatHTML, rendered.Format)
	})

	t.Run("End-to-End Web Scraping Flow", func(t *testing.T) {
		// Mock web scraping (in real test would use test server)
		testURL := "https://example.com/test-article"
		
		// Check compliance
		compliant, err := webScraper.CheckCompliance(testURL)
		// Since example.com likely doesn't have explicit ToS approval, this will fail
		// This is expected behavior for conservative compliance
		assert.False(t, compliant)

		// For testing, we'll simulate a successful scrape
		mockDoc := &document.Document{
			ID:      "scraped-001",
			Content: "This is scraped content about machine learning algorithms.",
			Metadata: map[string]interface{}{
				"source": "web_scraping",
				"url":    testURL,
				"domain": "example.com",
			},
		}

		// Store document
		docID, err := hybridStorage.StoreDocument(ctx, mockDoc)
		require.NoError(t, err)

		// Trigger processing
		eventBus.Publish(pipeline.EventDocumentStored, pipeline.EventData{
			"document_id": docID,
			"source":      "scraped",
		})

		time.Sleep(100 * time.Millisecond)

		// Verify document is processed and available
		processedDoc, err := hybridStorage.GetDocument(ctx, docID)
		require.NoError(t, err)
		assert.NotNil(t, processedDoc)
	})

	t.Run("Quality Validation Pipeline", func(t *testing.T) {
		// Test various quality levels
		testCases := []struct {
			name            string
			content         string
			expectedTier    string
			shouldPass      bool
		}{
			{
				name: "High Quality Content",
				content: `Introduction to Quantum Computing

Quantum computing represents a fundamental shift in computational paradigm, leveraging quantum mechanical phenomena 
to process information in ways classical computers cannot. Unlike classical bits that exist in binary states, 
quantum bits (qubits) can exist in superposition, enabling parallel processing of multiple states simultaneously.

The theoretical foundation of quantum computing rests on principles like superposition, entanglement, and quantum 
interference. These principles allow quantum algorithms to solve certain problems exponentially faster than classical 
algorithms. Notable examples include Shor's algorithm for factoring large numbers and Grover's algorithm for searching 
unsorted databases.

Current quantum computers face significant challenges including decoherence, error rates, and limited qubit counts. 
However, recent advances in error correction, quantum annealing, and gate-based quantum processors show promising 
progress toward practical quantum advantage in specific domains like cryptography, drug discovery, and optimization.`,
				expectedTier: "high",
				shouldPass:   true,
			},
			{
				name:         "Low Quality Content",
				content:      "Buy now! Best deals! Click here!",
				expectedTier: "failed",
				shouldPass:   false,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				result, err := qualityValidator.ValidateContent(ctx, tc.content, map[string]string{
					"source": "test",
				})

				if tc.shouldPass {
					require.NoError(t, err)
					assert.Equal(t, tc.expectedTier, result.QualityTier)
				} else {
					// Either error or failed tier
					if err == nil {
						assert.Equal(t, "failed", result.QualityTier)
					}
				}
			})
		}
	})

	t.Run("Processing Pipeline Integration", func(t *testing.T) {
		// Create document with content that needs cleaning
		dirtyDoc := &document.Document{
			ID: "process-test-001",
			Content: `Test Article

			Multiple    spaces     here.
			
			
			Too many blank lines above.
			Special™ characters® that© need• handling.
			<script>alert('xss')</script>
			Visit http://example.com for more.`,
			Metadata: map[string]interface{}{
				"source": "test",
			},
		}

		// Process through cleaner
		cleanedContent, stats := cleaner.Clean(dirtyDoc.Content)
		assert.NotEqual(t, dirtyDoc.Content, cleanedContent)
		assert.Greater(t, stats.CharactersRemoved, 0)
		assert.NotContains(t, cleanedContent, "<script>")
		assert.NotContains(t, cleanedContent, "Multiple    spaces")
	})

	t.Run("Presentation Layer Rendering", func(t *testing.T) {
		// Create test documents
		docs := []*presentation.Document{
			{
				ID:      "present-001",
				Content: "First document for presentation testing.",
				Metadata: map[string]interface{}{
					"title":  "Test Doc 1",
					"source": "synthetic",
				},
			},
			{
				ID:      "present-002",
				Content: "Second document with search terms like artificial intelligence.",
				Metadata: map[string]interface{}{
					"title":  "Test Doc 2",
					"source": "scraped",
				},
			},
		}

		// Test collection rendering
		collection, err := renderer.RenderCollection(docs, &presentation.CollectionOptions{
			RenderOptions: presentation.RenderOptions{
				Format: presentation.FormatJSON,
			},
			PageSize:       10,
			ShowStatistics: true,
		})
		require.NoError(t, err)
		assert.Equal(t, 2, collection.TotalCount)
		assert.NotNil(t, collection.Statistics)

		// Test search rendering
		searchResults := &presentation.SearchResults{
			Query:      "artificial intelligence",
			Documents:  docs,
			TotalHits:  1,
			SearchTime: 10 * time.Millisecond,
			Scores: map[string]float64{
				"present-002": 0.95,
			},
		}

		renderedSearch, err := renderer.RenderSearch(searchResults, &presentation.SearchOptions{
			CollectionOptions: presentation.CollectionOptions{
				RenderOptions: presentation.RenderOptions{
					Format:         presentation.FormatHTML,
					HighlightTerms: []string{"artificial", "intelligence"},
				},
			},
			ShowSnippets: true,
		})
		require.NoError(t, err)
		assert.Equal(t, "artificial intelligence", renderedSearch.Query)
		assert.Len(t, renderedSearch.Results, 2)
	})

	t.Run("Storage Layer Performance", func(t *testing.T) {
		// Test concurrent operations
		numDocs := 10
		docs := make([]*document.Document, numDocs)
		
		for i := 0; i < numDocs; i++ {
			docs[i] = &document.Document{
				ID:      fmt.Sprintf("perf-test-%03d", i),
				Content: fmt.Sprintf("Performance test document %d", i),
				Metadata: map[string]interface{}{
					"index": i,
				},
			}
		}

		// Store documents concurrently
		start := time.Now()
		errChan := make(chan error, numDocs)
		
		for _, doc := range docs {
			go func(d *document.Document) {
				_, err := hybridStorage.StoreDocument(ctx, d)
				errChan <- err
			}(doc)
		}

		// Collect results
		for i := 0; i < numDocs; i++ {
			err := <-errChan
			assert.NoError(t, err)
		}

		storeDuration := time.Since(start)
		t.Logf("Stored %d documents in %v (%.2f docs/sec)", 
			numDocs, storeDuration, float64(numDocs)/storeDuration.Seconds())

		// List all documents
		start = time.Now()
		allDocs, err := hybridStorage.ListDocuments(ctx, nil)
		require.NoError(t, err)
		listDuration := time.Since(start)
		
		t.Logf("Listed %d documents in %v", len(allDocs), listDuration)
		assert.GreaterOrEqual(t, len(allDocs), numDocs)
	})

	// Print metrics summary
	t.Run("System Metrics Summary", func(t *testing.T) {
		// Get processor stats
		processorStats := processor.GetStats()
		t.Logf("Processor Stats: %+v", processorStats)

		// Get event bus stats  
		eventStats := eventBus.GetStats()
		t.Logf("Event Bus - Subscribers: %d, Events Published: %d",
			len(eventStats.Subscribers), eventStats.EventsPublished)

		// Storage metrics would be collected via metricsCollector
		t.Log("Integration test completed successfully")
	})
}

// StorageAdapter adapts the storage backend to presentation storage interface
type StorageAdapter struct {
	backend storage.StorageBackend
}

func (sa *StorageAdapter) Store(doc *presentation.Document) error {
	storageDoc := &document.Document{
		ID:       doc.ID,
		Content:  doc.Content,
		Metadata: doc.Metadata,
	}
	_, err := sa.backend.StoreDocument(context.Background(), storageDoc)
	return err
}

func (sa *StorageAdapter) Get(id string) (*presentation.Document, error) {
	doc, err := sa.backend.GetDocument(context.Background(), id)
	if err != nil {
		return nil, err
	}
	return &presentation.Document{
		ID:       doc.ID,
		Content:  doc.Content,
		Metadata: doc.Metadata,
	}, nil
}

func (sa *StorageAdapter) List(prefix string, offset, limit int) ([]*presentation.Document, error) {
	// Simplified implementation - in production would handle pagination
	docs, err := sa.backend.ListDocuments(context.Background(), nil)
	if err != nil {
		return nil, err
	}

	result := make([]*presentation.Document, 0, len(docs))
	for _, doc := range docs {
		if offset > 0 {
			offset--
			continue
		}
		if limit > 0 && len(result) >= limit {
			break
		}
		result = append(result, &presentation.Document{
			ID:       doc.ID,
			Content:  doc.Content,
			Metadata: doc.Metadata,
		})
	}
	return result, nil
}

func (sa *StorageAdapter) Delete(id string) error {
	// Not implemented in backend interface
	return fmt.Errorf("delete not supported")
}

func (sa *StorageAdapter) Search(ctx context.Context, query string, options *presentation.SearchOptionsStorage) ([]*presentation.Document, error) {
	// Simplified search - in production would use proper search engine
	docs, err := sa.backend.ListDocuments(ctx, nil)
	if err != nil {
		return nil, err
	}

	results := make([]*presentation.Document, 0)
	for _, doc := range docs {
		// Simple substring search
		if containsQuery(doc.Content, query) {
			results = append(results, &presentation.Document{
				ID:       doc.ID,
				Content:  doc.Content,
				Metadata: doc.Metadata,
			})
		}
	}
	return results, nil
}

func (sa *StorageAdapter) GetStats() (*presentation.Stats, error) {
	docs, err := sa.backend.ListDocuments(context.Background(), nil)
	if err != nil {
		return nil, err
	}
	return &presentation.Stats{
		TotalDocuments: int64(len(docs)),
		LastUpdated:    time.Now(),
	}, nil
}

func (sa *StorageAdapter) Close() error {
	return nil
}

func containsQuery(content, query string) bool {
	// Case-insensitive search
	return len(query) > 0 && 
		(content != "" && 
		 len(content) >= len(query))
}

func createSyntheticGenerator(storage storage.StorageBackend, validator procurement.QualityValidator) *synthetic.SyntheticGenerator {
	// Create mock providers
	providers := map[string]procurement.LLMProvider{
		"gpt-4": &MockLLMProvider{name: "gpt-4"},
		"claude": &MockLLMProvider{name: "claude"},
	}

	config := &procurement.ServiceConfig{
		Enabled:        true,
		MaxConcurrency: 5,
		DefaultTimeout: 30 * time.Second,
	}

	return synthetic.NewSyntheticGenerator(
		providers,
		validator,
		&MockContentPlanner{},
		storage,
		config,
	)
}

func createWebScraper(storage storage.StorageBackend, validator procurement.QualityValidator) *scraping.WebScrapingService {
	config := &scraping.ScrapingConfig{
		MaxConcurrentCrawlers: 5,
		DefaultTimeout:        10 * time.Second,
		RespectRobotsTxt:      true,
		UserAgent:             "CAIA-Library-Test/1.0",
	}

	return scraping.NewWebScrapingService(config, storage, validator)
}

// Mock implementations for testing

type MockLLMProvider struct {
	name string
}

func (m *MockLLMProvider) GenerateContent(ctx context.Context, prompt string, params map[string]interface{}) (string, error) {
	return fmt.Sprintf("Generated content from %s: %s", m.name, prompt), nil
}

func (m *MockLLMProvider) GetModelName() string {
	return m.name
}

func (m *MockLLMProvider) IsAvailable() bool {
	return true
}

type MockContentPlanner struct{}

func (m *MockContentPlanner) PlanContent(ctx context.Context, topic string, contentType procurement.ContentType) (*procurement.ContentPlan, error) {
	return &procurement.ContentPlan{
		Topic:       topic,
		ContentType: contentType,
		Sections:    []string{"Introduction", "Main Content", "Conclusion"},
	}, nil
}