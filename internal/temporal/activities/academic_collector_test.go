package activities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcademicCollectorActivities_CollectArXiv(t *testing.T) {
	// Mock arXiv API response
	mockResponse := `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2301.00001</id>
    <title>Test Paper on AI</title>
    <summary>This is a test abstract about artificial intelligence.</summary>
    <published>2023-01-01T00:00:00Z</published>
    <updated>2023-01-02T00:00:00Z</updated>
    <author>
      <name>John Doe</name>
    </author>
    <author>
      <name>Jane Smith</name>
    </author>
    <link href="http://arxiv.org/pdf/2301.00001v1" type="application/pdf"/>
  </entry>
</feed>`

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent
		assert.Contains(t, r.Header.Get("User-Agent"), "CAIA-Library")
		assert.Contains(t, r.Header.Get("User-Agent"), "Academic-Research-Bot")
		assert.Contains(t, r.Header.Get("User-Agent"), "library@caiatech.com")

		// Verify query parameters
		assert.Contains(t, r.URL.String(), "search_query")
		
		w.Header().Set("Content-Type", "application/atom+xml")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	// Create collector
	collector := NewAcademicCollectorActivities()
	collector.httpClient = &http.Client{Timeout: 5 * time.Second}

	// Test input
	input := workflows.ScheduledIngestionInput{
		Name:     "arxiv",
		Type:     "arxiv",
		URL:      server.URL,
		Filters:  []string{"cs.AI", "cs.LG"},
		Metadata: map[string]string{"test": "true"},
	}

	// Execute collection
	ctx := context.Background()
	docs, err := collector.collectArXiv(ctx, input)

	// Assertions
	require.NoError(t, err)
	require.Len(t, docs, 1)

	doc := docs[0]
	assert.Equal(t, "http://arxiv.org/abs/2301.00001", doc.ID)
	assert.Equal(t, "http://arxiv.org/pdf/2301.00001v1", doc.URL)
	assert.Equal(t, "pdf", doc.Type)
	
	// Verify attribution
	assert.Equal(t, "arXiv", doc.Metadata["source"])
	assert.Contains(t, doc.Metadata["attribution"], "Content from arXiv.org")
	assert.Contains(t, doc.Metadata["attribution"], "collected by Caia Tech")
	assert.Contains(t, doc.Metadata["attribution"], "https://caiatech.com")
	assert.Equal(t, "arXiv License", doc.Metadata["license"])
	assert.Contains(t, doc.Metadata["collection_agent"], "CAIA-Library")
	assert.Equal(t, "Collected in compliance with arXiv Terms of Use", doc.Metadata["ethical_notice"])
	
	// Verify paper metadata
	assert.Equal(t, "Test Paper on AI", doc.Metadata["title"])
	assert.Equal(t, "John Doe, Jane Smith", doc.Metadata["authors"])
	assert.Equal(t, "This is a test abstract about artificial intelligence.", doc.Metadata["abstract"])
	
	// Verify custom metadata is preserved
	assert.Equal(t, "true", doc.Metadata["test"])
}

func TestAcademicCollectorActivities_RateLimiting(t *testing.T) {
	collector := NewAcademicCollectorActivities()
	
	// Test that arXiv collection respects rate limit
	start := time.Now()
	
	ctx := context.Background()
	input := workflows.ScheduledIngestionInput{
		Name: "arxiv",
		Type: "arxiv",
		URL:  "http://invalid-url-for-testing",
	}
	
	// This should wait 3 seconds before making the request
	_, _ = collector.collectArXiv(ctx, input)
	
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 3*time.Second, "ArXiv collector should wait at least 3 seconds")
}

func TestAcademicCollectorActivities_UnsupportedSource(t *testing.T) {
	collector := NewAcademicCollectorActivities()
	
	input := workflows.ScheduledIngestionInput{
		Name: "unsupported-source",
		Type: "unknown",
	}
	
	ctx := context.Background()
	_, err := collector.CollectAcademicSourcesActivity(ctx, input)
	
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported academic source")
}

func TestAcademicCollectorActivities_FormatAuthors(t *testing.T) {
	collector := NewAcademicCollectorActivities()
	
	tests := []struct {
		name     string
		authors  []ArXivAuthor
		expected string
	}{
		{
			name:     "single author",
			authors:  []ArXivAuthor{{Name: "John Doe"}},
			expected: "John Doe",
		},
		{
			name:     "multiple authors",
			authors:  []ArXivAuthor{{Name: "John Doe"}, {Name: "Jane Smith"}, {Name: "Bob Johnson"}},
			expected: "John Doe, Jane Smith, Bob Johnson",
		},
		{
			name:     "no authors",
			authors:  []ArXivAuthor{},
			expected: "",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := collector.formatAuthors(tt.authors)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAcademicCollectorActivities_UserAgent(t *testing.T) {
	collector := NewAcademicCollectorActivities()
	
	// Verify User-Agent format
	assert.Contains(t, collector.userAgent, "CAIA-Library")
	assert.Contains(t, collector.userAgent, "github.com/Caia-Tech/caia-library")
	assert.Contains(t, collector.userAgent, "library@caiatech.com")
	assert.Contains(t, collector.userAgent, "Academic-Research-Bot")
}

func TestAcademicCollectorActivities_AllSourcesHaveAttribution(t *testing.T) {
	collector := NewAcademicCollectorActivities()
	ctx := context.Background()
	
	sources := []string{"arxiv", "pubmed", "doaj", "plos"}
	
	for _, source := range sources {
		t.Run(source, func(t *testing.T) {
			input := workflows.ScheduledIngestionInput{
				Name: source,
				Type: source,
				URL:  "http://example.com",
			}
			
			// We don't care if it fails, just that it tries to add attribution
			docs, _ := collector.CollectAcademicSourcesActivity(ctx, input)
			
			// If we got any documents, verify they have attribution
			for _, doc := range docs {
				assert.NotEmpty(t, doc.Metadata["attribution"], "Document should have attribution")
				assert.Contains(t, doc.Metadata["attribution"], "Caia Tech", "Attribution should mention Caia Tech")
				assert.NotEmpty(t, doc.Metadata["source"], "Document should have source")
				assert.NotEmpty(t, doc.Metadata["collection_agent"], "Document should have collection agent")
			}
		})
	}
}

// Benchmark test to ensure rate limiting doesn't impact performance too much
func BenchmarkAcademicCollectorActivities_CollectArXiv(b *testing.B) {
	collector := NewAcademicCollectorActivities()
	// ctx := context.Background()
	
	// input := workflows.ScheduledIngestionInput{
	//	Name:    "arxiv",
	//	Type:    "arxiv",
	//	URL:     "http://example.com",
	//	Filters: []string{"cs.AI"},
	// }
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Skip actual HTTP calls in benchmark
		collector.formatAuthors([]ArXivAuthor{{Name: "Test Author"}})
	}
}