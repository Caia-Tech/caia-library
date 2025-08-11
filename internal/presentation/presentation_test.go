package presentation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/presentation"
	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStorage implements presentation.Storage for testing
type MockStorage struct {
	documents map[string]*presentation.Document
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		documents: make(map[string]*presentation.Document),
	}
}

func (ms *MockStorage) Store(doc *presentation.Document) error {
	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc_%d", len(ms.documents)+1)
	}
	ms.documents[doc.ID] = doc
	return nil
}

func (ms *MockStorage) Get(id string) (*presentation.Document, error) {
	doc, exists := ms.documents[id]
	if !exists {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	return doc, nil
}

func (ms *MockStorage) List(prefix string, offset, limit int) ([]*presentation.Document, error) {
	var docs []*presentation.Document
	for _, doc := range ms.documents {
		if prefix == "" || strings.HasPrefix(doc.ID, prefix) {
			docs = append(docs, doc)
		}
	}
	
	// Apply pagination
	if offset >= len(docs) {
		return []*presentation.Document{}, nil
	}
	
	end := offset + limit
	if end > len(docs) || limit == 0 {
		end = len(docs)
	}
	
	return docs[offset:end], nil
}

func (ms *MockStorage) Delete(id string) error {
	delete(ms.documents, id)
	return nil
}

func (ms *MockStorage) Search(ctx context.Context, query string, options *presentation.SearchOptionsStorage) ([]*presentation.Document, error) {
	var results []*presentation.Document
	queryLower := strings.ToLower(query)
	
	for _, doc := range ms.documents {
		if strings.Contains(strings.ToLower(doc.Content), queryLower) {
			results = append(results, doc)
		}
	}
	
	return results, nil
}

func (ms *MockStorage) GetStats() (*presentation.Stats, error) {
	return &presentation.Stats{
		TotalDocuments: int64(len(ms.documents)),
		TotalSize:      0,
		LastUpdated:    time.Now(),
	}, nil
}

func (ms *MockStorage) Close() error {
	return nil
}

func TestRenderer(t *testing.T) {
	renderer := presentation.NewRenderer(nil)

	t.Run("Render Single Document", func(t *testing.T) {
		doc := &presentation.Document{
			ID:      "test-doc-1",
			Content: "This is a test document with some content. It contains multiple sentences for testing.",
			Metadata: map[string]interface{}{
				"title":       "Test Document",
				"source":      "test",
				"created_at":  time.Now().Format(time.RFC3339),
				"quality_tier": "high",
			},
		}

		options := &presentation.RenderOptions{
			Format:          presentation.FormatJSON,
			IncludeMetadata: true,
			IncludeQuality:  false,
		}

		rendered, err := renderer.RenderDocument(doc, options)
		require.NoError(t, err)
		assert.Equal(t, "test-doc-1", rendered.ID)
		assert.Equal(t, "Test Document", rendered.Title)
		assert.NotEmpty(t, rendered.Content)
		assert.NotNil(t, rendered.Metadata)
		assert.Equal(t, presentation.FormatJSON, rendered.Format)
	})

	t.Run("Render Document Collection", func(t *testing.T) {
		docs := []*presentation.Document{
			{
				ID:      "doc-1",
				Content: "First document content",
				Metadata: map[string]interface{}{
					"source": "synthetic",
					"quality_score": 0.8,
				},
			},
			{
				ID:      "doc-2",
				Content: "Second document content",
				Metadata: map[string]interface{}{
					"source": "scraped",
					"quality_score": 0.6,
				},
			},
			{
				ID:      "doc-3",
				Content: "Third document content",
				Metadata: map[string]interface{}{
					"source": "synthetic",
					"quality_score": 0.9,
				},
			},
		}

		options := &presentation.CollectionOptions{
			RenderOptions: presentation.RenderOptions{
				Format:    presentation.FormatMarkdown,
				MaxLength: 100,
			},
			PageSize:       2,
			PageNumber:     1,
			ShowStatistics: true,
		}

		collection, err := renderer.RenderCollection(docs, options)
		require.NoError(t, err)
		assert.Equal(t, 2, len(collection.Documents))
		assert.Equal(t, 3, collection.TotalCount)
		assert.NotNil(t, collection.Statistics)
		assert.Equal(t, 3, collection.Statistics.TotalDocuments)
	})

	t.Run("Render Search Results", func(t *testing.T) {
		searchResults := &presentation.SearchResults{
			Query: "test query",
			Documents: []*presentation.Document{
				{
					ID:      "search-1",
					Content: "This document contains the test query we're looking for.",
				},
				{
					ID:      "search-2",
					Content: "Another document with test query in the middle of the content.",
				},
			},
			Scores: map[string]float64{
				"search-1": 0.95,
				"search-2": 0.85,
			},
			TotalHits:  2,
			SearchTime: 50 * time.Millisecond,
		}

		options := &presentation.SearchOptions{
			CollectionOptions: presentation.CollectionOptions{
				RenderOptions: presentation.RenderOptions{
					Format:         presentation.FormatHTML,
					HighlightTerms: []string{"test", "query"},
				},
				PageSize:   10,
				PageNumber: 1,
			},
			ShowSnippets:     true,
			SnippetLength:    50,
			HighlightMatches: true,
		}

		rendered, err := renderer.RenderSearch(searchResults, options)
		require.NoError(t, err)
		assert.Equal(t, "test query", rendered.Query)
		assert.Equal(t, 2, len(rendered.Results))
		assert.Equal(t, 2, rendered.TotalHits)
		
		// Check snippets
		for _, result := range rendered.Results {
			assert.NotEmpty(t, result.Snippet)
			assert.True(t, len(result.Snippet) <= 150) // With ellipsis
		}
	})

	t.Run("Export Document Formats", func(t *testing.T) {
		doc := &presentation.Document{
			ID:      "export-test",
			Content: "Content to export in various formats.",
			Metadata: map[string]interface{}{
				"title":  "Export Test",
				"author": "Test Author",
			},
		}

		// Test JSON export
		jsonData, err := renderer.ExportDocument(doc, presentation.ExportJSON)
		require.NoError(t, err)
		assert.Contains(t, string(jsonData), "export-test")
		assert.Contains(t, string(jsonData), "Content to export")

		// Test Markdown export
		mdData, err := renderer.ExportDocument(doc, presentation.ExportMarkdown)
		require.NoError(t, err)
		assert.Contains(t, string(mdData), "# Export Test")
		assert.Contains(t, string(mdData), "## Metadata")
		assert.Contains(t, string(mdData), "## Content")

		// Test XML export
		xmlData, err := renderer.ExportDocument(doc, presentation.ExportXML)
		require.NoError(t, err)
		assert.Contains(t, string(xmlData), "<?xml version")
		assert.Contains(t, string(xmlData), "<document>")
		assert.Contains(t, string(xmlData), "<id>export-test</id>")
	})

	t.Run("Content Highlighting", func(t *testing.T) {
		doc := &presentation.Document{
			ID:      "highlight-test",
			Content: "This is a document about artificial intelligence and machine learning.",
		}

		options := &presentation.RenderOptions{
			Format:         presentation.FormatMarkdown,
			HighlightTerms: []string{"artificial", "machine"},
		}

		rendered, err := renderer.RenderDocument(doc, options)
		require.NoError(t, err)
		
		// Check that terms are highlighted with markdown bold
		assert.Contains(t, rendered.Content, "**artificial**")
		assert.Contains(t, rendered.Content, "**machine**")
	})

	t.Run("Quality Metrics Inclusion", func(t *testing.T) {
		qualityResult := &procurement.ValidationResult{
			OverallScore:    0.85,
			QualityTier:     "high",
			ConfidenceLevel: 0.9,
			DimensionScores: map[string]float64{
				"accuracy":     0.9,
				"completeness": 0.8,
				"relevance":    0.85,
			},
		}

		qualityJSON, _ := json.Marshal(qualityResult)

		doc := &presentation.Document{
			ID:      "quality-test",
			Content: "High quality content",
			Metadata: map[string]interface{}{
				"quality": string(qualityJSON),
			},
		}

		options := &presentation.RenderOptions{
			Format:         presentation.FormatJSON,
			IncludeQuality: true,
		}

		rendered, err := renderer.RenderDocument(doc, options)
		require.NoError(t, err)
		assert.NotNil(t, rendered.QualityMetrics)
		assert.Equal(t, 0.85, rendered.QualityMetrics.OverallScore)
		assert.Equal(t, "high", rendered.QualityMetrics.QualityTier)
	})
}

func TestPresentationAPI(t *testing.T) {
	// Setup
	storage := NewMockStorage()
	renderer := presentation.NewRenderer(nil)
	_ = presentation.NewAPI(renderer, storage, nil) // API is tested separately

	// Add test documents
	testDocs := []*presentation.Document{
		{
			ID:      "api-doc-1",
			Content: "First API test document about technology and innovation.",
			Metadata: map[string]interface{}{
				"title":  "Tech Innovation",
				"source": "synthetic",
				"language": "en",
				"quality_tier": "high",
			},
		},
		{
			ID:      "api-doc-2",
			Content: "Second API test document discussing artificial intelligence.",
			Metadata: map[string]interface{}{
				"title":  "AI Discussion",
				"source": "scraped",
				"language": "en",
				"quality_tier": "medium",
			},
		},
		{
			ID:      "api-doc-3",
			Content: "Third document about machine learning and deep learning.",
			Metadata: map[string]interface{}{
				"title":  "ML & DL",
				"source": "synthetic",
				"language": "en",
				"quality_tier": "high",
			},
		},
	}

	for _, doc := range testDocs {
		require.NoError(t, storage.Store(doc))
	}

	// Setup router for testing
	router := mux.NewRouter()
	base := router.PathPrefix("/api/v1").Subrouter()
	
	base.HandleFunc("/documents", func(w http.ResponseWriter, r *http.Request) {
		// Simple handler for testing
		docs, _ := storage.List("", 0, 20)
		response := map[string]interface{}{
			"documents": docs,
			"total":     len(docs),
		}
		json.NewEncoder(w).Encode(response)
	}).Methods("GET")
	
	base.HandleFunc("/documents/{id}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		doc, err := storage.Get(vars["id"])
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
			return
		}
		json.NewEncoder(w).Encode(doc)
	}).Methods("GET")

	base.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
	}).Methods("GET")

	t.Run("List Documents", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/documents", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, float64(3), response["total"])
	})

	t.Run("Get Single Document", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/documents/api-doc-1", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var doc presentation.Document
		err := json.Unmarshal(w.Body.Bytes(), &doc)
		require.NoError(t, err)
		assert.Equal(t, "api-doc-1", doc.ID)
		assert.Contains(t, doc.Content, "technology")
	})

	t.Run("Document Not Found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/documents/non-existent", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Health Check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		
		var health map[string]string
		err := json.Unmarshal(w.Body.Bytes(), &health)
		require.NoError(t, err)
		assert.Equal(t, "healthy", health["status"])
	})
}

func TestRenderingPerformance(t *testing.T) {
	renderer := presentation.NewRenderer(nil)

	// Create a large document for performance testing
	var contentBuilder strings.Builder
	for i := 0; i < 100; i++ {
		contentBuilder.WriteString(fmt.Sprintf("Paragraph %d: This is a test paragraph with substantial content to test rendering performance. ", i))
		contentBuilder.WriteString("It contains multiple sentences and various words that need to be processed. ")
		contentBuilder.WriteString("The renderer should handle this efficiently even with large documents.\n\n")
	}

	doc := &presentation.Document{
		ID:      "perf-test",
		Content: contentBuilder.String(),
		Metadata: map[string]interface{}{
			"title":        "Performance Test Document",
			"source":       "test",
			"quality_tier": "high",
			"created_at":   time.Now().Format(time.RFC3339),
		},
	}

	t.Run("Render Large Document", func(t *testing.T) {
		start := time.Now()
		
		options := &presentation.RenderOptions{
			Format:          presentation.FormatHTML,
			IncludeMetadata: true,
			MaxLength:       0, // No truncation
		}

		rendered, err := renderer.RenderDocument(doc, options)
		require.NoError(t, err)
		
		duration := time.Since(start)
		
		assert.NotEmpty(t, rendered.Content)
		assert.Less(t, duration, 100*time.Millisecond, "Rendering should be fast")
		
		t.Logf("Rendered large document (%d chars) in %v", len(doc.Content), duration)
	})

	t.Run("Render With Highlighting", func(t *testing.T) {
		start := time.Now()
		
		options := &presentation.RenderOptions{
			Format:         presentation.FormatMarkdown,
			HighlightTerms: []string{"test", "paragraph", "content"},
		}

		rendered, err := renderer.RenderDocument(doc, options)
		require.NoError(t, err)
		
		duration := time.Since(start)
		
		assert.Contains(t, rendered.Content, "**test**")
		assert.Contains(t, rendered.Content, "**paragraph**")
		assert.Less(t, duration, 200*time.Millisecond, "Highlighting should be reasonably fast")
		
		t.Logf("Rendered with highlighting in %v", duration)
	})

	t.Run("Export Large Document", func(t *testing.T) {
		formats := []presentation.ExportFormat{
			presentation.ExportJSON,
			presentation.ExportMarkdown,
			presentation.ExportXML,
		}

		for _, format := range formats {
			start := time.Now()
			
			data, err := renderer.ExportDocument(doc, format)
			require.NoError(t, err)
			
			duration := time.Since(start)
			
			assert.NotEmpty(t, data)
			assert.Less(t, duration, 100*time.Millisecond, fmt.Sprintf("Export to %s should be fast", format))
			
			t.Logf("Exported to %s (%d bytes) in %v", format, len(data), duration)
		}
	})
}

func TestCollectionStatistics(t *testing.T) {
	renderer := presentation.NewRenderer(nil)

	// Create documents with various metadata
	docs := []*presentation.Document{}
	sources := []string{"synthetic", "scraped", "curated"}
	languages := []string{"en", "es", "fr"}
	qualityTiers := []string{"high", "medium", "low"}

	for i := 0; i < 30; i++ {
		doc := &presentation.Document{
			ID:      fmt.Sprintf("stat-doc-%d", i),
			Content: fmt.Sprintf("Document %d content", i),
			Metadata: map[string]interface{}{
				"source":       sources[i%3],
				"language":     languages[i%3],
				"quality_tier": qualityTiers[i%3],
				"quality_score": float64(i%10) / 10.0,
				"created_at":   time.Now().Add(-time.Hour * time.Duration(i)).Format(time.RFC3339),
			},
		}
		docs = append(docs, doc)
	}

	options := &presentation.CollectionOptions{
		RenderOptions: presentation.RenderOptions{
			Format: presentation.FormatJSON,
		},
		PageSize:       10,
		PageNumber:     1,
		ShowStatistics: true,
	}

	collection, err := renderer.RenderCollection(docs, options)
	require.NoError(t, err)

	stats := collection.Statistics
	require.NotNil(t, stats)

	// Verify statistics
	assert.Equal(t, 30, stats.TotalDocuments)
	assert.Equal(t, 10, stats.SourceDistribution["synthetic"])
	assert.Equal(t, 10, stats.SourceDistribution["scraped"])
	assert.Equal(t, 10, stats.SourceDistribution["curated"])
	assert.Equal(t, 10, stats.LanguageDistribution["en"])
	assert.Equal(t, 10, stats.QualityDistribution["high"])
	assert.NotNil(t, stats.DateRange)
	assert.True(t, stats.AverageQuality >= 0 && stats.AverageQuality <= 1)
}