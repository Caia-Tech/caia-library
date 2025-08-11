package procurement_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStorage implements a simple mock storage backend for testing
type MockStorage struct {
	documents map[string]*document.Document
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		documents: make(map[string]*document.Document),
	}
}

func (ms *MockStorage) StoreDocument(ctx context.Context, doc *document.Document) (string, error) {
	ms.documents[doc.ID] = doc
	return doc.ID, nil
}

func (ms *MockStorage) GetDocument(ctx context.Context, id string) (*document.Document, error) {
	if doc, exists := ms.documents[id]; exists {
		return doc, nil
	}
	return nil, fmt.Errorf("document not found: %s", id)
}

func (ms *MockStorage) MergeBranch(ctx context.Context, branchName string) error {
	return nil // Mock implementation
}

func (ms *MockStorage) ListDocuments(ctx context.Context, filters map[string]string) ([]*document.Document, error) {
	docs := make([]*document.Document, 0)
	for _, doc := range ms.documents {
		docs = append(docs, doc)
	}
	return docs, nil
}

func (ms *MockStorage) Health(ctx context.Context) error {
	return nil // Always healthy for tests
}

func TestWebScrapingIntegration(t *testing.T) {
	// Create mock HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte(`User-agent: *
Disallow: /private/
Allow: /public/
Crawl-delay: 1`))
		case "/test-page":
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
    <meta name="description" content="A test page for scraping">
    <meta name="author" content="Test Author">
    <meta name="keywords" content="test, scraping, golang">
</head>
<body>
    <h1>Test Page Title</h1>
    <div class="content">
        <p>This is a test paragraph with some content for scraping.</p>
        <p>Another paragraph with more information about the topic.</p>
        <ul>
            <li>Item 1</li>
            <li>Item 2</li>
            <li>Item 3</li>
        </ul>
    </div>
    <footer>
        <p>Copyright 2024 Test Site</p>
    </footer>
</body>
</html>`))
		case "/private/restricted":
			http.Error(w, "Forbidden", http.StatusForbidden)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	// Create components
	complianceEngine := scraping.NewComplianceEngine(nil)
	rateLimiter := scraping.NewAdaptiveRateLimiter(nil)
	extractor := scraping.NewContentExtractor(nil)
	qualityValidator := quality.NewQualityValidator(nil)
	
	// Mock storage
	mockStorage := NewMockStorage()
	
	t.Run("Compliance Engine", func(t *testing.T) {
		ctx := context.Background()
		
		// Test allowed URL
		result, err := complianceEngine.CheckCompliance(ctx, server.URL+"/test-page")
		require.NoError(t, err)
		assert.True(t, result.RobotsCompliant) // robots.txt allows this path
		assert.Equal(t, 1*time.Second, result.RequiredDelay)
		// Note: Overall allowed may be false due to ToS compliance for unknown domains
		
		// Test disallowed URL  
		result, err = complianceEngine.CheckCompliance(ctx, server.URL+"/private/restricted")
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.False(t, result.RobotsCompliant)
		assert.Contains(t, result.Restrictions, "Blocked by robots.txt")
	})
	
	t.Run("Rate Limiter", func(t *testing.T) {
		ctx := context.Background()
		domain := "example.com"
		
		// Test basic rate limiting
		start := time.Now()
		err := rateLimiter.Wait(ctx, domain, 100*time.Millisecond)
		require.NoError(t, err)
		
		// Second request should be delayed
		err = rateLimiter.Wait(ctx, domain, 100*time.Millisecond)
		require.NoError(t, err)
		elapsed := time.Since(start)
		assert.True(t, elapsed >= 100*time.Millisecond)
		
		// Test adaptive behavior
		rateLimiter.RecordRequest(domain, scraping.RequestResult{
			Timestamp:   time.Now(),
			StatusCode:  200,
			Success:     true,
			RateLimited: false,
		})
		
		stats := rateLimiter.GetDomainStats(domain)
		assert.NotNil(t, stats)
		assert.Equal(t, int64(1), stats.SuccessCount)
	})
	
	t.Run("Content Extractor", func(t *testing.T) {
		ctx := context.Background()
		
		// Test successful extraction
		result, err := extractor.ExtractContent(ctx, server.URL+"/test-page")
		require.NoError(t, err)
		assert.True(t, result.Success)
		assert.Equal(t, 200, result.StatusCode)
		assert.NotNil(t, result.Document)
		
		// Check extracted content
		doc := result.Document
		assert.Contains(t, doc.Content.Text, "Test Page Title")
		assert.Contains(t, doc.Content.Text, "test paragraph")
		assert.Equal(t, "Test Page", doc.Content.Metadata["title"])
		assert.Equal(t, "A test page for scraping", doc.Content.Metadata["description"])
		assert.Equal(t, "Test Author", doc.Content.Metadata["author"])
		
		// Test URL validation
		err = extractor.ValidateURL("invalid-url")
		assert.Error(t, err)
		
		err = extractor.ValidateURL("ftp://example.com")
		assert.Error(t, err)
		
		err = extractor.ValidateURL("https://example.com")
		assert.NoError(t, err)
	})
	
	t.Run("Distributed Crawler", func(t *testing.T) {
		ctx := context.Background()
		
		crawler := scraping.NewDistributedCrawler(
			complianceEngine,
			rateLimiter, 
			extractor,
			qualityValidator,
			mockStorage,
			nil, // Use default config
		)
		
		// Start crawler
		err := crawler.Start(ctx)
		require.NoError(t, err)
		defer crawler.Stop()
		
		// Submit a test job
		job := &scraping.CrawlJob{
			ID:     "test-job-1",
			URL:    server.URL + "/test-page",
			Domain: "localhost",
			Depth:  0,
			Source: "test",
		}
		
		err = crawler.SubmitJob(ctx, job)
		require.NoError(t, err)
		
		// Wait for job completion
		time.Sleep(2 * time.Second)
		
		// Check job status
		status := crawler.GetJobStatus("test-job-1")
		// Job might be completed or removed from active jobs
		if status != nil {
			assert.NotEqual(t, "pending", string(status.Status))
		}
		
		// Check metrics
		metrics := crawler.GetMetrics()
		assert.True(t, metrics.JobsQueued > 0)
	})
	
	t.Run("Scraping Service", func(t *testing.T) {
		ctx := context.Background()
		
		service := scraping.NewScrapingService(
			complianceEngine,
			rateLimiter,
			extractor, 
			qualityValidator,
			mockStorage,
			nil, // Use default config
		)
		
		// Start service
		err := service.Start(ctx)
		require.NoError(t, err)
		defer service.Stop()
		
		// Add a test source
		source := &scraping.ScrapingSource{
			ID:       "test-source",
			Name:     "Test Source",
			BaseURL:  server.URL,
			StartURLs: []string{server.URL + "/test-page"},
			CrawlInterval: 1 * time.Hour,
			MaxPages: 10,
		}
		
		err = service.AddSource(source)
		require.NoError(t, err)
		
		// List sources
		sources := service.ListSources()
		assert.Len(t, sources, 1)
		assert.Equal(t, "test-source", sources[0].ID)
		
		// Get source
		retrieved := service.GetSource("test-source")
		assert.NotNil(t, retrieved)
		assert.Equal(t, "Test Source", retrieved.Name)
		
		// Manually trigger scraping
		err = service.ScrapeSource(ctx, "test-source")
		require.NoError(t, err)
		
		// Wait a bit for processing
		time.Sleep(1 * time.Second)
		
		// Check metrics
		metrics := service.GetMetrics()
		assert.True(t, metrics.ActiveSources >= 1)
		
		// Remove source
		err = service.RemoveSource("test-source")
		require.NoError(t, err)
		
		sources = service.ListSources()
		assert.Len(t, sources, 0)
	})
}

func TestScrapingCompliance(t *testing.T) {
	// Test various robots.txt scenarios
	testCases := []struct {
		name           string
		robotsTxt      string
		testPath       string
		expectedAllowed bool
	}{
		{
			name: "Simple Disallow",
			robotsTxt: `User-agent: *
Disallow: /admin/`,
			testPath:        "/admin/users",
			expectedAllowed: false,
		},
		{
			name: "Wildcard Allow",
			robotsTxt: `User-agent: *
Disallow: /
Allow: /public/`,
			testPath:        "/public/page",
			expectedAllowed: true,
		},
		{
			name: "Empty Robots.txt",
			robotsTxt:       ``,
			testPath:        "/any/path",
			expectedAllowed: true,
		},
		{
			name: "Crawl Delay",
			robotsTxt: `User-agent: *
Crawl-delay: 5`,
			testPath:        "/page",
			expectedAllowed: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test server with specific robots.txt
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/robots.txt" {
					w.Header().Set("Content-Type", "text/plain")
					w.Write([]byte(tc.robotsTxt))
				} else {
					w.Write([]byte("<html><body>Test page</body></html>"))
				}
			}))
			defer server.Close()
			
			complianceEngine := scraping.NewComplianceEngine(nil)
			ctx := context.Background()
			
			result, err := complianceEngine.CheckCompliance(ctx, server.URL+tc.testPath)
			require.NoError(t, err)
			assert.Equal(t, tc.expectedAllowed, result.RobotsCompliant)
		})
	}
}

func TestContentExtractionVariations(t *testing.T) {
	testCases := []struct {
		name        string
		html        string
		expectTitle string
		expectText  string
	}{
		{
			name: "Article with Metadata",
			html: `<!DOCTYPE html>
<html>
<head>
    <title>Article Title</title>
    <meta name="description" content="Article description">
    <meta property="og:title" content="OG Title">
</head>
<body>
    <article>
        <h1>Main Article Title</h1>
        <p>Article content goes here.</p>
    </article>
</body>
</html>`,
			expectTitle: "Article Title",
			expectText:  "Main Article Title",
		},
		{
			name: "Documentation Page",
			html: `<!DOCTYPE html>
<html>
<head>
    <title>API Documentation</title>
</head>
<body>
    <main>
        <h1>API Reference</h1>
        <div class="content">
            <h2>Getting Started</h2>
            <p>This API allows you to interact with our service.</p>
            <pre><code>curl -X GET https://api.example.com</code></pre>
        </div>
    </main>
</body>
</html>`,
			expectTitle: "API Documentation",
			expectText:  "API Reference",
		},
		{
			name: "Blog Post",
			html: `<!DOCTYPE html>
<html>
<head>
    <title>Blog Post Title</title>
    <meta name="author" content="Blog Author">
</head>
<body>
    <div class="post-content">
        <h1>Understanding Web Scraping</h1>
        <p class="intro">Web scraping is the process of extracting data from websites.</p>
        <p>It involves making HTTP requests and parsing HTML content.</p>
    </div>
</body>
</html>`,
			expectTitle: "Blog Post Title",
			expectText:  "Understanding Web Scraping",
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/html")
				w.Write([]byte(tc.html))
			}))
			defer server.Close()
			
			extractor := scraping.NewContentExtractor(nil)
			ctx := context.Background()
			
			result, err := extractor.ExtractContent(ctx, server.URL)
			require.NoError(t, err)
			assert.True(t, result.Success)
			
			assert.Equal(t, tc.expectTitle, result.Document.Content.Metadata["title"])
			assert.Contains(t, result.Document.Content.Text, tc.expectText)
		})
	}
}

func BenchmarkContentExtraction(b *testing.B) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>Benchmark Page</title>
    <meta name="description" content="A page for benchmarking content extraction">
</head>
<body>
    <h1>Main Title</h1>
    <div class="content">
        <p>Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.</p>
        <p>Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.</p>
        <ul>
            <li>First item</li>
            <li>Second item</li>
            <li>Third item</li>
        </ul>
    </div>
</body>
</html>`
	
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	}))
	defer server.Close()
	
	extractor := scraping.NewContentExtractor(nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		_, err := extractor.ExtractContent(ctx, server.URL)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRateLimiter(b *testing.B) {
	rateLimiter := scraping.NewAdaptiveRateLimiter(nil)
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := rateLimiter.Wait(ctx, "example.com", 1*time.Millisecond)
		if err != nil {
			b.Fatal(err)
		}
	}
}