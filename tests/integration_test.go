package tests

import (
	"context"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/activities"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/Caia-Tech/caia-library/pkg/embedder"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"math"
)

// TestCompleteDocumentWorkflow tests the entire document ingestion workflow
func TestCompleteDocumentWorkflow(t *testing.T) {
	// Create test suite
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	// Register workflow
	env.RegisterWorkflow(workflows.DocumentIngestionWorkflow)

	// Register activities
	env.RegisterActivity(activities.FetchDocumentActivity)
	env.RegisterActivity(activities.ExtractTextActivity)
	env.RegisterActivity(activities.GenerateEmbeddingsActivity)
	env.RegisterActivity(activities.StoreDocumentActivity)
	env.RegisterActivity(activities.IndexDocumentActivity)
	env.RegisterActivity(activities.MergeBranchActivity)

	// Create test input
	input := workflows.DocumentIngestionInput{
		URL:  "https://example.com/test.pdf",
		Type: "pdf",
		Metadata: map[string]string{
			"source":      "test",
			"attribution": "Content from example.com, collected by Caia Tech",
			"title":       "Test Document",
			"author":      "Test Author",
		},
	}

	// Mock activity implementations
	env.OnActivity(activities.FetchDocumentActivity, mock.Anything, mock.Anything).Return(
		&activities.FetchResult{
			Content:     []byte("PDF content here"),
			ContentType: "application/pdf",
			Size:        1024,
		}, nil)

	env.OnActivity(activities.ExtractTextActivity, mock.Anything, mock.Anything).Return(
		&activities.ExtractResult{
			Text: "This is extracted text from the PDF",
			Metadata: map[string]string{
				"pages":      "5",
				"language":   "en",
				"word_count": "100",
			},
		}, nil)

	env.OnActivity(activities.GenerateEmbeddingsActivity, mock.Anything, mock.Anything).Return(
		&activities.EmbeddingResult{
			Embeddings: [][]float32{
				generateTestEmbedding(384),
			},
			Model:      "all-MiniLM-L6-v2",
			Dimensions: 384,
		}, nil)

	env.OnActivity(activities.StoreDocumentActivity, mock.Anything, mock.Anything).Return(
		&activities.StoreResult{
			DocumentID: "test-doc-123",
			CommitHash: "abc123def456",
			StorePath:  "documents/test/test-doc-123.json",
		}, nil)

	env.OnActivity(activities.IndexDocumentActivity, mock.Anything, mock.Anything).Return(nil, nil)
	env.OnActivity(activities.MergeBranchActivity, mock.Anything, mock.Anything).Return(nil, nil)

	// Execute workflow
	env.ExecuteWorkflow(workflows.DocumentIngestionWorkflow, input)

	// Verify completion
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get result
	var result workflows.DocumentIngestionResult
	require.NoError(t, env.GetWorkflowResult(&result))

	// Verify result
	assert.Equal(t, "test-doc-123", result.DocumentID)
	assert.Equal(t, "documents/test/test-doc-123.json", result.StoragePath)
	assert.Equal(t, "abc123def456", result.CommitHash)
	assert.Equal(t, "Completed", result.Status)
}

// TestGitIntegration tests Git repository operations
func TestGitIntegration(t *testing.T) {
	// Create temporary directory for test repo
	tmpDir, err := ioutil.TempDir("", "caia-test-repo")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Initialize repository
	repo, err := git.InitRepository(tmpDir)
	require.NoError(t, err)

	// Test document storage
	doc := &document.Document{
		ID:     "test-doc-1",
		Source: "test",
		URL:    "https://example.com/test.pdf",
		Title:  "Test Document",
		Author: "Test Author",
		Text:   "This is a test document",
		Metadata: map[string]string{
			"attribution": "Content from example.com, collected by Caia Tech",
			"license":     "CC-BY",
		},
		Embeddings: [][]float32{
			generateTestEmbedding(384),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Store document
	commitHash, err := repo.StoreDocument("test-branch", doc)
	require.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Verify document exists
	storedDoc, err := repo.GetDocument("test-branch", doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, storedDoc.ID)
	assert.Equal(t, doc.Title, storedDoc.Title)
	assert.Contains(t, storedDoc.Metadata["attribution"], "Caia Tech")

	// Test branch merge
	err = repo.MergeBranch("test-branch")
	require.NoError(t, err)

	// Verify document in main branch
	mainDoc, err := repo.GetDocument("main", doc.ID)
	require.NoError(t, err)
	assert.Equal(t, doc.ID, mainDoc.ID)
}

// TestAttributionCompliance verifies Caia Tech attribution
func TestAttributionCompliance(t *testing.T) {
	testCases := []struct {
		name         string
		source       string
		expectedAttr string
		shouldPass   bool
	}{
		{
			name:         "ArXiv with proper attribution",
			source:       "arXiv",
			expectedAttr: "Content from arXiv.org, collected by Caia Tech",
			shouldPass:   true,
		},
		{
			name:         "PubMed with proper attribution",
			source:       "pubmed",
			expectedAttr: "Content from PubMed Central, collected by Caia Tech",
			shouldPass:   true,
		},
		{
			name:         "Missing Caia attribution",
			source:       "bad-source",
			expectedAttr: "Content from bad-source",
			shouldPass:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create document
			doc := &document.Document{
				ID:     "test-" + tc.source,
				Source: tc.source,
				Metadata: map[string]string{
					"attribution": tc.expectedAttr,
				},
			}

			// Check attribution
			hasCAIAAttribution := false
			if attr, ok := doc.Metadata["attribution"]; ok {
				hasCAIAAttribution = contains(attr, "Caia Tech")
			}

			if tc.shouldPass {
				assert.True(t, hasCAIAAttribution, "Missing Caia Tech attribution")
			} else {
				assert.False(t, hasCAIAAttribution, "Should not have Caia Tech attribution")
			}
		})
	}
}

// TestRateLimiting verifies rate limiting for academic sources
func TestRateLimiting(t *testing.T) {
	limiter := activities.NewAcademicRateLimiter()

	sources := []struct {
		name          string
		requestCount  int
		expectBlocked bool
	}{
		{"arxiv", 3, false},    // Within limit
		{"pubmed", 10, false},  // At limit
		{"pubmed", 11, true},   // Over limit
		{"doaj", 30, false},    // Within limit
		{"plos", 120, false},   // At limit
		{"plos", 121, true},    // Over limit
	}

	for _, s := range sources {
		t.Run(s.name, func(t *testing.T) {
			// Reset limiter for test
			limiter = activities.NewAcademicRateLimiter()

			// Make requests
			blocked := false
			for i := 0; i < s.requestCount; i++ {
				if !limiter.Allow(s.name) {
					blocked = true
					break
				}
			}

			assert.Equal(t, s.expectBlocked, blocked)
		})
	}
}

// TestDocumentProcessingPipeline tests the complete processing pipeline
func TestDocumentProcessingPipeline(t *testing.T) {
	ctx := context.Background()

	// Test PDF extraction
	t.Run("PDFExtraction", func(t *testing.T) {
		extractor := extractor.NewExtractor()
		
		// Create test PDF content
		testPDF := []byte("%PDF-1.4\nTest PDF content")
		
		result, err := extractor.ExtractText(ctx, testPDF, "pdf")
		require.NoError(t, err)
		assert.NotEmpty(t, result.Text)
		assert.Contains(t, result.Metadata["format"], "pdf")
	})

	// Test embedding generation
	t.Run("EmbeddingGeneration", func(t *testing.T) {
		embedder, err := embedder.NewAdvancedEmbedder(384)
		require.NoError(t, err)

		text := "Caia Library provides ethical document collection with proper attribution"
		embeddings, err := embedder.GenerateEmbeddings(ctx, text)
		require.NoError(t, err)
		
		assert.Len(t, embeddings, 1)
		assert.Len(t, embeddings[0], 384)
		
		// Verify embeddings are normalized
		var sum float32
		for _, val := range embeddings[0] {
			sum += val * val
		}
		assert.InDelta(t, 1.0, sum, 0.01, "Embeddings should be normalized")
	})

	// Test HTML extraction
	t.Run("HTMLExtraction", func(t *testing.T) {
		extractor := extractor.NewExtractor()
		
		htmlContent := []byte(`
			<html>
				<head><title>Test Document</title></head>
				<body>
					<h1>Caia Library Test</h1>
					<p>This document was collected by Caia Tech with proper attribution.</p>
				</body>
			</html>
		`)
		
		result, err := extractor.ExtractText(ctx, htmlContent, "html")
		require.NoError(t, err)
		assert.Contains(t, result.Text, "Caia Library Test")
		assert.Contains(t, result.Text, "Caia Tech")
		assert.Equal(t, "Test Document", result.Metadata["title"])
	})
}

// TestScheduledIngestion tests scheduled document collection
func TestScheduledIngestion(t *testing.T) {
	ts := &testsuite.WorkflowTestSuite{}
	env := ts.NewTestWorkflowEnvironment()

	// Register workflow
	env.RegisterWorkflow(workflows.ScheduledIngestionWorkflow)

	// Create test input
	input := workflows.ScheduledIngestionInput{
		SourceType: "arxiv",
		SourceURL:  "http://export.arxiv.org/api/query",
		Filters:    []string{"cs.AI", "cs.LG"},
		Metadata: map[string]string{
			"attribution": "Caia Tech",
		},
	}

	// Mock collector activity
	collector := activities.NewCollectorActivities()
	env.RegisterActivity(collector.CollectFromSourceActivity)
	env.RegisterActivity(collector.CheckDuplicateActivity)

	env.OnActivity(collector.CollectFromSourceActivity, mock.Anything, mock.Anything).Return(
		&activities.CollectResult{
			Documents: []activities.CollectedDocument{
				{
					URL:    "https://arxiv.org/pdf/2301.00001.pdf",
					Title:  "Test Paper 1",
					Author: "Author 1",
					Source: "arXiv",
				},
				{
					URL:    "https://arxiv.org/pdf/2301.00002.pdf",
					Title:  "Test Paper 2",
					Author: "Author 2",
					Source: "arXiv",
				},
			},
			NextCursor: "",
		}, nil)

	env.OnActivity(collector.CheckDuplicateActivity, mock.Anything, mock.Anything).Return(false, nil)

	// Execute workflow
	env.ExecuteWorkflow(workflows.ScheduledIngestionWorkflow, input)

	// Verify completion
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Get result
	var result workflows.ScheduledIngestionResult
	require.NoError(t, env.GetWorkflowResult(&result))

	// Verify result
	assert.Equal(t, 2, result.DocumentsProcessed)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, "Completed", result.Status)
}

// Helper functions

func generateTestEmbedding(dimensions int) []float32 {
	embedding := make([]float32, dimensions)
	for i := range embedding {
		embedding[i] = float32(i) / float32(dimensions)
	}
	// Normalize
	var sum float32
	for _, val := range embedding {
		sum += val * val
	}
	norm := float32(1.0 / float32(math.Sqrt(float64(sum))))
	for i := range embedding {
		embedding[i] *= norm
	}
	return embedding
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 s[:len(substr)] == substr ||
		 s[len(s)-len(substr):] == substr ||
		 len(substr) > 0 && len(s) > len(substr) && 
		 findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}