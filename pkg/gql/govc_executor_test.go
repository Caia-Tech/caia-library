package gql

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGovcExecutor_DocumentQuery(t *testing.T) {
	// Create govc backend for testing
	backend, err := storage.NewGovcBackend("test-gql", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create executor
	executor := NewGovcExecutor(backend)

	// Store test documents
	testDocs := []*document.Document{
		{
			ID: "gql-test-001",
			Source: document.Source{
				Type: "arXiv",
				URL:  "https://arxiv.org/pdf/2301.00001.pdf",
			},
			Content: document.Content{
				Text: "This is a machine learning paper about neural networks.",
				Metadata: map[string]string{
					"title":       "Neural Networks in Machine Learning",
					"author":      "John Smith",
					"attribution": "Content from arXiv.org, collected by Caia Tech",
				},
			},
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
		},
		{
			ID: "gql-test-002",
			Source: document.Source{
				Type: "PubMed",
				URL:  "https://pubmed.ncbi.nlm.nih.gov/123456",
			},
			Content: document.Content{
				Text: "This is a medical research paper.",
				Metadata: map[string]string{
					"title":       "Medical Research Advances",
					"authors":     "Jane Doe, Bob Wilson",
					"attribution": "Content from PubMed, collected by Caia Tech",
				},
			},
			CreatedAt: time.Now().Add(-12 * time.Hour),
			UpdatedAt: time.Now().Add(-12 * time.Hour),
		},
		{
			ID: "gql-test-003",
			Source: document.Source{
				Type: "arXiv",
				URL:  "https://arxiv.org/pdf/2301.00002.pdf",
			},
			Content: document.Content{
				Text: "Another paper about transformers and attention mechanisms.",
				Metadata: map[string]string{
					"title":       "Transformer Architecture Analysis",
					"author":      "Alice Johnson",
					"attribution": "Content from arXiv.org, collected by Caia Tech",
				},
			},
			CreatedAt: time.Now().Add(-6 * time.Hour),
			UpdatedAt: time.Now().Add(-6 * time.Hour),
		},
	}

	// Store documents
	ctx := context.Background()
	for _, doc := range testDocs {
		_, err := backend.StoreDocument(ctx, doc)
		require.NoError(t, err)
	}

	// Give some time for indexing
	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name     string
		query    string
		expected int
		checks   func(t *testing.T, result *Result)
	}{
		{
			name:     "select all documents",
			query:    "SELECT FROM documents",
			expected: 3,
			checks: func(t *testing.T, result *Result) {
				assert.Equal(t, QueryDocuments, result.Type)
				assert.Len(t, result.Items, 3)
			},
		},
		{
			name:     "filter by source",
			query:    "SELECT FROM documents WHERE source = \"arXiv\"",
			expected: 2,
			checks: func(t *testing.T, result *Result) {
				assert.Len(t, result.Items, 2)
				for _, item := range result.Items {
					doc := item.(DocumentResult)
					assert.Equal(t, "arXiv", doc.Source)
				}
			},
		},
		{
			name:     "search by title contains",
			query:    "SELECT FROM documents WHERE title ~ \"neural\"",
			expected: 1,
			checks: func(t *testing.T, result *Result) {
				assert.Len(t, result.Items, 1)
				doc := result.Items[0].(DocumentResult)
				assert.Contains(t, doc.Title, "Neural")
			},
		},
		{
			name:     "filter by author",
			query:    "SELECT FROM documents WHERE author = \"John Smith\"",
			expected: 1,
			checks: func(t *testing.T, result *Result) {
				assert.Len(t, result.Items, 1)
				doc := result.Items[0].(DocumentResult)
				assert.Equal(t, "John Smith", doc.Author)
			},
		},
		{
			name:     "multiple filters",
			query:    "SELECT FROM documents WHERE source = \"arXiv\" AND author = \"Alice Johnson\"",
			expected: 1,
			checks: func(t *testing.T, result *Result) {
				assert.Len(t, result.Items, 1)
				doc := result.Items[0].(DocumentResult)
				assert.Equal(t, "arXiv", doc.Source)
				assert.Equal(t, "Alice Johnson", doc.Author)
			},
		},
		{
			name:     "order by created_at desc",
			query:    "SELECT FROM documents ORDER BY created_at DESC",
			expected: 3,
			checks: func(t *testing.T, result *Result) {
				assert.Len(t, result.Items, 3)
				// Should be ordered newest first
				doc1 := result.Items[0].(DocumentResult)
				doc2 := result.Items[1].(DocumentResult)
				assert.True(t, doc1.CreatedAt.After(doc2.CreatedAt) || doc1.CreatedAt.Equal(doc2.CreatedAt))
			},
		},
		{
			name:     "limit results",
			query:    "SELECT FROM documents LIMIT 2",
			expected: 2,
			checks: func(t *testing.T, result *Result) {
				assert.Len(t, result.Items, 2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(ctx, tt.query)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, tt.expected, result.Count)
			assert.Greater(t, result.Elapsed.Nanoseconds(), int64(0))

			if tt.checks != nil {
				tt.checks(t, result)
			}
		})
	}
}

func TestGovcExecutor_AttributionQuery(t *testing.T) {
	// Create govc backend for testing
	backend, err := storage.NewGovcBackend("test-attribution", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create executor
	executor := NewGovcExecutor(backend)

	// Store documents with different attribution
	testDocs := []*document.Document{
		{
			ID: "attr-001",
			Source: document.Source{Type: "arXiv"},
			Content: document.Content{
				Metadata: map[string]string{
					"attribution": "Content from arXiv.org, collected by Caia Tech",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID: "attr-002",
			Source: document.Source{Type: "arXiv"},
			Content: document.Content{
				Metadata: map[string]string{
					"attribution": "Content from arXiv.org, collected by Caia Tech",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		{
			ID: "attr-003",
			Source: document.Source{Type: "PubMed"},
			Content: document.Content{
				Metadata: map[string]string{
					"attribution": "Content from PubMed, no Caia attribution",
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}

	// Store documents
	ctx := context.Background()
	for _, doc := range testDocs {
		_, err := backend.StoreDocument(ctx, doc)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Query attribution
	result, err := executor.Execute(ctx, "SELECT FROM attribution")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, QueryAttribution, result.Type)
	assert.Greater(t, result.Count, 0)
	assert.Greater(t, result.Elapsed.Nanoseconds(), int64(0))

	// Check that we have attribution results
	found := false
	for _, item := range result.Items {
		if attr, ok := item.(*AttributionResult); ok {
			if attr.Source == "arXiv" {
				assert.Equal(t, 2, attr.DocumentCount)
				assert.True(t, attr.CAIAAttribution)
				found = true
				break
			}
		}
	}
	assert.True(t, found, "Should find arXiv attribution result")
}

func TestGovcExecutor_SourcesQuery(t *testing.T) {
	// Create govc backend for testing
	backend, err := storage.NewGovcBackend("test-sources", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create executor
	executor := NewGovcExecutor(backend)

	// Store documents from different sources
	sources := []string{"arXiv", "arXiv", "PubMed", "IEEE", "arXiv"}
	for i, source := range sources {
		doc := &document.Document{
			ID: fmt.Sprintf("src-%03d", i+1),
			Source: document.Source{
				Type: source,
				URL:  fmt.Sprintf("https://%s.com/doc%d", strings.ToLower(source), i+1),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Content for document %d", i+1),
				Metadata: map[string]string{
					"title": fmt.Sprintf("Document %d", i+1),
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := backend.StoreDocument(context.Background(), doc)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Query sources
	result, err := executor.Execute(context.Background(), "SELECT FROM sources")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, QuerySources, result.Type)
	assert.Greater(t, result.Count, 0)

	// Check source counts
	sourceMap := make(map[string]int)
	for _, item := range result.Items {
		srcResult := item.(map[string]interface{})
		source := srcResult["source"].(string)
		count := srcResult["count"].(int)
		sourceMap[source] = count
	}

	assert.Equal(t, 3, sourceMap["arXiv"])
	assert.Equal(t, 1, sourceMap["PubMed"])
	assert.Equal(t, 1, sourceMap["IEEE"])
}

func TestGovcExecutor_AuthorsQuery(t *testing.T) {
	// Create govc backend for testing
	backend, err := storage.NewGovcBackend("test-authors", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create executor
	executor := NewGovcExecutor(backend)

	// Store documents with different authors
	authors := []string{"John Smith", "Jane Doe", "John Smith", "Bob Wilson"}
	for i, author := range authors {
		doc := &document.Document{
			ID: fmt.Sprintf("auth-%03d", i+1),
			Source: document.Source{
				Type: "test",
				URL:  fmt.Sprintf("https://test.com/paper%d", i+1),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Paper content %d", i+1),
				Metadata: map[string]string{
					"author": author,
					"title":  fmt.Sprintf("Paper %d", i+1),
				},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		_, err := backend.StoreDocument(context.Background(), doc)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Query authors
	result, err := executor.Execute(context.Background(), "SELECT FROM authors")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, QueryAuthors, result.Type)
	assert.Greater(t, result.Count, 0)

	// Check author counts
	authorMap := make(map[string]int)
	for _, item := range result.Items {
		authResult := item.(map[string]interface{})
		author := authResult["author"].(string)
		count := authResult["count"].(int)
		authorMap[author] = count
	}

	assert.Equal(t, 2, authorMap["John Smith"])
	assert.Equal(t, 1, authorMap["Jane Doe"])
	assert.Equal(t, 1, authorMap["Bob Wilson"])
}

func TestGovcExecutor_Performance(t *testing.T) {
	// Create govc backend for testing
	backend, err := storage.NewGovcBackend("test-perf", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create executor
	executor := NewGovcExecutor(backend)

	// Store multiple documents
	ctx := context.Background()
	numDocs := 50
	for i := 0; i < numDocs; i++ {
		doc := &document.Document{
			ID: fmt.Sprintf("perf-%03d", i+1),
			Source: document.Source{
				Type: fmt.Sprintf("source-%d", i%5), // 5 different sources
				URL:  fmt.Sprintf("https://source-%d.com/doc%d", i%5, i+1),
			},
			Content: document.Content{
				Text: fmt.Sprintf("Document content %d", i),
				Metadata: map[string]string{
					"title":  fmt.Sprintf("Document %d Title", i),
					"author": fmt.Sprintf("Author %d", i%10), // 10 different authors
				},
			},
			CreatedAt: time.Now().Add(time.Duration(-i) * time.Minute),
			UpdatedAt: time.Now().Add(time.Duration(-i) * time.Minute),
		}

		_, err := backend.StoreDocument(ctx, doc)
		require.NoError(t, err)
	}

	time.Sleep(200 * time.Millisecond)

	// Test query performance
	start := time.Now()
	result, err := executor.Execute(ctx, "SELECT FROM documents LIMIT 20")
	elapsed := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 20, result.Count)

	// Performance should be fast with govc backend
	t.Logf("Query executed in %v", elapsed)
	assert.Less(t, elapsed.Milliseconds(), int64(50), "Query should complete in under 50ms")

	// Test filtered query performance
	start = time.Now()
	result, err = executor.Execute(ctx, "SELECT FROM documents WHERE source = \"source-1\" ORDER BY created_at DESC")
	elapsed = time.Since(start)

	require.NoError(t, err)
	assert.Equal(t, 10, result.Count) // Should find 10 documents with source-1
	assert.Less(t, elapsed.Milliseconds(), int64(25), "Filtered query should complete in under 25ms")
}

func TestGovcExecutor_InvalidQuery(t *testing.T) {
	// Create govc backend for testing
	backend, err := storage.NewGovcBackend("test-invalid", storage.NewSimpleMetricsCollector())
	require.NoError(t, err)
	defer backend.Close()

	// Create executor
	executor := NewGovcExecutor(backend)

	// Test invalid queries
	ctx := context.Background()

	tests := []struct {
		name  string
		query string
	}{
		{
			name:  "invalid syntax",
			query: "INVALID QUERY SYNTAX",
		},
		{
			name:  "missing FROM",
			query: "SELECT documents",
		},
		{
			name:  "unknown query type",
			query: "SELECT FROM unknown_type",
		},
		{
			name:  "empty query",
			query: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := executor.Execute(ctx, tt.query)
			assert.Error(t, err)
			assert.Nil(t, result)
		})
	}
}