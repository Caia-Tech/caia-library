package activities

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/stretchr/testify/assert"
)

// TestFetchActivityEdgeCases tests document fetching with various edge cases
func TestFetchActivityEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		setupServer    func() *httptest.Server
		url            string
		expectError    bool
		expectedStatus int
	}{
		{
			name: "successful PDF fetch",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/pdf")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("%PDF-1.4\nTest PDF content"))
				}))
			},
			expectError: false,
		},
		{
			name: "large document fetch",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(http.StatusOK)
					// Write 1MB of content
					data := make([]byte, 1024*1024)
					for i := range data {
						data[i] = 'A'
					}
					w.Write(data)
				}))
			},
			expectError: false,
		},
		{
			name: "server timeout",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					time.Sleep(3 * time.Second)
					w.WriteHeader(http.StatusOK)
				}))
			},
			expectError: true,
		},
		{
			name: "redirect chain",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/redirect" {
						http.Redirect(w, r, "/final", http.StatusMovedPermanently)
						return
					}
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte("Final content"))
				}))
			},
			url:         "/redirect",
			expectError: false,
		},
		{
			name: "rate limited",
			setupServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte("Rate limited"))
				}))
			},
			expectedStatus: http.StatusTooManyRequests,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := tt.setupServer()
			defer server.Close()

			url := server.URL
			if tt.url != "" {
				url = server.URL + tt.url
			}

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			result, err := FetchDocumentActivity(ctx, url)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, result.Content)
			}
		})
	}
}

// TestExtractActivityWithDifferentContentTypes tests text extraction with various content types
func TestExtractActivityWithDifferentContentTypes(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		contentType string
		expectError bool
		expectText  string
	}{
		{
			name:        "plain text",
			content:     []byte("This is plain text content for testing."),
			contentType: "text/plain",
			expectError: false,
			expectText:  "This is plain text content for testing.",
		},
		{
			name:        "HTML content",
			content:     []byte("<html><body><h1>Title</h1><p>Paragraph content</p></body></html>"),
			contentType: "text/html",
			expectError: false,
		},
		{
			name:        "empty text",
			content:     []byte(""),
			contentType: "text/plain",
			expectError: true,
		},
		{
			name:        "very large text",
			content:     make([]byte, 10*1024*1024), // 10MB
			contentType: "text/plain",
			expectError: false,
		},
		{
			name:        "unicode content",
			content:     []byte("Hello 世界 🌍 Ελληνικά"),
			contentType: "text/plain; charset=utf-8",
			expectError: false,
			expectText:  "Hello 世界 🌍 Ελληνικά",
		},
		{
			name:        "malformed PDF",
			content:     []byte("Not actually a PDF file"),
			contentType: "application/pdf",
			expectError: true,
		},
	}

	// Fill large content with readable text
	for i := range tests[3].content {
		tests[3].content[i] = byte('A' + (i % 26))
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := workflows.ExtractInput{
				Content: tt.content,
				Type:    tt.contentType,
			}

			result, err := ExtractTextActivity(ctx, input)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, result.Text)
			} else {
				assert.NoError(t, err)
				if tt.expectText != "" {
					assert.Equal(t, tt.expectText, result.Text)
				} else {
					assert.NotEmpty(t, result.Text)
				}
				assert.NotEmpty(t, result.Metadata)
			}
		})
	}
}

// TestEmbeddingsActivityPerformance tests embedding generation performance
func TestEmbeddingsActivityPerformance(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		contentSize int
		maxDuration time.Duration
	}{
		{
			name:        "small content",
			contentSize: 100,
			maxDuration: 1 * time.Second,
		},
		{
			name:        "medium content",
			contentSize: 1000,
			maxDuration: 2 * time.Second,
		},
		{
			name:        "large content",
			contentSize: 10000,
			maxDuration: 5 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			content := make([]byte, tt.contentSize)
			for i := range content {
				content[i] = byte('A' + (i % 26))
			}

			start := time.Now()
			embeddings, err := GenerateEmbeddingsActivity(ctx, content)
			duration := time.Since(start)

			assert.NoError(t, err)
			assert.Len(t, embeddings, 384) // Expected embedding dimension
			assert.Less(t, duration, tt.maxDuration, "Embedding generation took too long")

			// Verify embeddings are not all zeros
			nonZeroCount := 0
			for _, val := range embeddings {
				if val != 0 {
					nonZeroCount++
				}
			}
			assert.Greater(t, nonZeroCount, 0, "All embeddings are zero")
		})
	}
}

// TestActivityErrorPropagation tests that errors are properly propagated
func TestActivityErrorPropagation(t *testing.T) {
	ctx := context.Background()

	t.Run("fetch activity error", func(t *testing.T) {
		_, err := FetchDocumentActivity(ctx, "http://non-existent-domain-12345.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no such host")
	})

	t.Run("extract activity error", func(t *testing.T) {
		input := workflows.ExtractInput{
			Content: nil,
			Type:    "",
		}
		_, err := ExtractTextActivity(ctx, input)
		assert.Error(t, err)
	})

	t.Run("embeddings activity error", func(t *testing.T) {
		_, err := GenerateEmbeddingsActivity(ctx, nil)
		assert.Error(t, err)
	})
}

// TestConcurrentActivityExecution tests concurrent activity execution
func TestConcurrentActivityExecution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Concurrent test content"))
	}))
	defer server.Close()

	ctx := context.Background()
	numConcurrent := 10
	errChan := make(chan error, numConcurrent)
	resultChan := make(chan workflows.FetchResult, numConcurrent)

	// Launch concurrent fetch activities
	for i := 0; i < numConcurrent; i++ {
		go func() {
			result, err := FetchDocumentActivity(ctx, server.URL)
			errChan <- err
			resultChan <- result
		}()
	}

	// Collect results
	for i := 0; i < numConcurrent; i++ {
		err := <-errChan
		result := <-resultChan
		
		assert.NoError(t, err)
		assert.Equal(t, "Concurrent test content", string(result.Content))
		assert.Equal(t, "text/plain", result.ContentType)
	}
}

// BenchmarkActivities benchmarks activity performance
func BenchmarkFetchActivity(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Benchmark content"))
	}))
	defer server.Close()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := FetchDocumentActivity(ctx, server.URL)
		if err != nil {
			b.Fatalf("Failed to fetch: %v", err)
		}
	}
}

func BenchmarkExtractActivity(b *testing.B) {
	content := []byte("This is benchmark content for text extraction testing")
	input := workflows.ExtractInput{
		Content: content,
		Type:    "text/plain",
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ExtractTextActivity(ctx, input)
		if err != nil {
			b.Fatalf("Failed to extract: %v", err)
		}
	}
}

func BenchmarkEmbeddingsActivity(b *testing.B) {
	content := []byte("This is benchmark content for embeddings generation testing")
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateEmbeddingsActivity(ctx, content)
		if err != nil {
			b.Fatalf("Failed to generate embeddings: %v", err)
		}
	}
}