package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/api"
	"github.com/Caia-Tech/caia-library/pkg/gql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/client"
)

// TestEndToEnd performs comprehensive integration testing of the Caia Library system
func TestEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end test in short mode")
	}

	// Setup test environment
	ctx := context.Background()
	baseURL := "http://localhost:8080"
	
	// Ensure services are running
	t.Log("Checking service health...")
	err := waitForService(baseURL + "/health", 30*time.Second)
	require.NoError(t, err, "API service not healthy")

	// Test 1: Health Check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	// Test 2: Ingest a document with proper Caia Tech attribution
	t.Run("DocumentIngestion", func(t *testing.T) {
		// Create test document request
		docReq := api.IngestRequest{
			URL:  "https://arxiv.org/pdf/2301.00001.pdf",
			Type: "pdf",
			Metadata: map[string]string{
				"source":             "arXiv",
				"attribution":        "Content from arXiv.org, collected by Caia Tech (https://caiatech.com)",
				"ethical_compliance": "true",
				"title":              "Test Paper on Neural Networks",
				"author":             "Jane Smith",
			},
		}

		// Send ingestion request
		body, _ := json.Marshal(docReq)
		resp, err := http.Post(baseURL+"/api/v1/documents", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()

		// Check response
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		
		var result api.IngestResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotEmpty(t, result.WorkflowID)
		assert.NotEmpty(t, result.RunID)

		// Wait for workflow to complete
		t.Logf("Waiting for workflow %s to complete...", result.WorkflowID)
		err = waitForWorkflow(baseURL, result.WorkflowID, 2*time.Minute)
		require.NoError(t, err)
	})

	// Test 3: Query documents using Git Query Language
	t.Run("GQLQueries", func(t *testing.T) {
		testQueries := []struct {
			name  string
			query string
			check func(t *testing.T, resp api.QueryResponse)
		}{
			{
				name:  "FindArxivDocuments",
				query: `SELECT FROM documents WHERE source = "arXiv" LIMIT 10`,
				check: func(t *testing.T, resp api.QueryResponse) {
					assert.Equal(t, "documents", resp.Type)
					assert.Greater(t, resp.Count, 0)
					if resp.Count > 0 {
						doc := resp.Items[0].(map[string]interface{})
						assert.Equal(t, "arXiv", doc["source"])
						metadata := doc["metadata"].(map[string]interface{})
						assert.Contains(t, metadata["attribution"], "Caia Tech")
					}
				},
			},
			{
				name:  "CheckAttributionCompliance",
				query: `SELECT FROM attribution WHERE caia_attribution = true`,
				check: func(t *testing.T, resp api.QueryResponse) {
					assert.Equal(t, "attribution", resp.Type)
					// All sources should have Caia attribution
					for _, item := range resp.Items {
						source := item.(map[string]interface{})
						assert.True(t, source["caia_attribution"].(bool))
					}
				},
			},
			{
				name:  "FindMissingAttribution",
				query: `SELECT FROM attribution WHERE caia_attribution = false`,
				check: func(t *testing.T, resp api.QueryResponse) {
					assert.Equal(t, "attribution", resp.Type)
					assert.Equal(t, 0, resp.Count, "Found sources without Caia attribution!")
				},
			},
			{
				name:  "ListSources",
				query: `SELECT FROM sources`,
				check: func(t *testing.T, resp api.QueryResponse) {
					assert.Equal(t, "sources", resp.Type)
					assert.Greater(t, resp.Count, 0)
				},
			},
			{
				name:  "SearchByTitle",
				query: `SELECT FROM documents WHERE title ~ "neural" LIMIT 5`,
				check: func(t *testing.T, resp api.QueryResponse) {
					assert.Equal(t, "documents", resp.Type)
					// Check that results contain "neural" in title
					for _, item := range resp.Items {
						doc := item.(map[string]interface{})
						title := doc["title"].(string)
						assert.Contains(t, title, "neural", "Title should contain 'neural'")
					}
				},
			},
		}

		for _, tc := range testQueries {
			t.Run(tc.name, func(t *testing.T) {
				// Execute query
				queryReq := api.QueryRequest{Query: tc.query}
				body, _ := json.Marshal(queryReq)
				
				resp, err := http.Post(baseURL+"/api/v1/query", "application/json", bytes.NewReader(body))
				require.NoError(t, err)
				defer resp.Body.Close()
				
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				
				var result api.QueryResponse
				err = json.NewDecoder(resp.Body).Decode(&result)
				require.NoError(t, err)
				
				// Run test-specific checks
				tc.check(t, result)
			})
		}
	})

	// Test 4: Attribution Statistics
	t.Run("AttributionStats", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/stats/attribution")
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var stats map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&stats)
		require.NoError(t, err)
		
		// Check compliance
		assert.Equal(t, "100.0%", stats["compliance_rate"], "Attribution compliance should be 100%")
		assert.Contains(t, stats["attribution_text"], "Caia Tech")
	})

	// Test 5: Scheduled Ingestion
	t.Run("ScheduledIngestion", func(t *testing.T) {
		// Create scheduled ingestion
		schedReq := api.ScheduledIngestionRequest{
			Name:     "test-arxiv-daily",
			Type:     "arxiv",
			URL:      "http://export.arxiv.org/api/query",
			Schedule: "0 2 * * *", // 2 AM daily
			Filters:  []string{"cs.AI", "cs.LG"},
			Metadata: map[string]string{
				"attribution": "Caia Tech",
			},
		}
		
		body, _ := json.Marshal(schedReq)
		resp, err := http.Post(baseURL+"/api/v1/ingestion/scheduled", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		
		var result api.ScheduledIngestionResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotEmpty(t, result.ScheduleID)
	})

	// Test 6: Batch Ingestion
	t.Run("BatchIngestion", func(t *testing.T) {
		batchReq := api.BatchIngestionRequest{
			Documents: []api.BatchDocument{
				{
					URL:  "https://arxiv.org/pdf/2301.00002.pdf",
					Type: "pdf",
					Metadata: map[string]string{
						"source":      "arXiv",
						"attribution": "Content from arXiv.org, collected by Caia Tech",
						"title":       "Advances in Machine Learning",
						"author":      "John Doe",
					},
				},
				{
					URL:  "https://arxiv.org/pdf/2301.00003.pdf",
					Type: "pdf",
					Metadata: map[string]string{
						"source":      "arXiv",
						"attribution": "Content from arXiv.org, collected by Caia Tech",
						"title":       "Deep Learning Applications",
						"author":      "Alice Johnson",
					},
				},
			},
		}
		
		body, _ := json.Marshal(batchReq)
		resp, err := http.Post(baseURL+"/api/v1/ingestion/batch", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
		
		var result api.BatchIngestionResponse
		err = json.NewDecoder(resp.Body).Decode(&result)
		require.NoError(t, err)
		assert.NotEmpty(t, result.WorkflowID)
		assert.Equal(t, 2, result.TotalDocuments)
	})

	// Test 7: Git Repository Verification
	t.Run("GitRepositoryIntegrity", func(t *testing.T) {
		repoPath := os.Getenv("CAIA_REPO_PATH")
		if repoPath == "" {
			repoPath = "./data/repo"
		}
		
		// Check Git repository exists
		_, err := os.Stat(repoPath + "/.git")
		require.NoError(t, err, "Git repository not found")
		
		// Verify Git history
		cmd := exec.Command("git", "-C", repoPath, "log", "--oneline", "-n", "5")
		output, err := cmd.Output()
		require.NoError(t, err)
		t.Logf("Recent Git commits:\n%s", output)
		
		// Check for attribution in commits
		cmd = exec.Command("git", "-C", repoPath, "log", "--grep=Caia Tech", "--oneline")
		output, err = cmd.Output()
		require.NoError(t, err)
		assert.NotEmpty(t, output, "No commits with Caia Tech attribution found")
	})

	// Test 8: Query Examples
	t.Run("QueryExamples", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/query/examples")
		require.NoError(t, err)
		defer resp.Body.Close()
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		
		var examples map[string][]map[string]string
		err = json.NewDecoder(resp.Body).Decode(&examples)
		require.NoError(t, err)
		
		// Verify examples exist
		assert.NotEmpty(t, examples["examples"])
		assert.Greater(t, len(examples["examples"]), 5)
	})

	// Test 9: Performance Test
	t.Run("PerformanceTest", func(t *testing.T) {
		// Query with limit to test performance
		query := `SELECT FROM documents WHERE source = "arXiv" ORDER BY created_at DESC LIMIT 100`
		
		start := time.Now()
		
		queryReq := api.QueryRequest{Query: query}
		body, _ := json.Marshal(queryReq)
		
		resp, err := http.Post(baseURL+"/api/v1/query", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		
		duration := time.Since(start)
		
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Less(t, duration, 5*time.Second, "Query took too long")
		
		t.Logf("Query completed in %v", duration)
	})

	// Test 10: Academic Source Compliance
	t.Run("AcademicSourceCompliance", func(t *testing.T) {
		// Test academic collector with rate limiting
		sources := []string{"arxiv", "pubmed", "doaj", "plos"}
		
		for _, source := range sources {
			t.Run(source, func(t *testing.T) {
				// Query for documents from this source
				query := fmt.Sprintf(`SELECT FROM documents WHERE source = "%s" LIMIT 5`, source)
				
				queryReq := api.QueryRequest{Query: query}
				body, _ := json.Marshal(queryReq)
				
				resp, err := http.Post(baseURL+"/api/v1/query", "application/json", bytes.NewReader(body))
				require.NoError(t, err)
				defer resp.Body.Close()
				
				if resp.StatusCode == http.StatusOK {
					var result api.QueryResponse
					err = json.NewDecoder(resp.Body).Decode(&result)
					require.NoError(t, err)
					
					// Check all documents have proper attribution
					for _, item := range result.Items {
						doc := item.(map[string]interface{})
						metadata := doc["metadata"].(map[string]interface{})
						attribution := metadata["attribution"].(string)
						
						assert.Contains(t, attribution, "Caia Tech", "Missing Caia Tech attribution")
						assert.Contains(t, attribution, source, "Missing source attribution")
					}
				}
			})
		}
	})
}

// Helper functions

func waitForService(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	
	return fmt.Errorf("service at %s not ready after %v", url, timeout)
}

func waitForWorkflow(baseURL, workflowID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/v1/workflows/" + workflowID)
		if err != nil {
			return err
		}
		
		var status map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&status)
		resp.Body.Close()
		
		if err != nil {
			return err
		}
		
		if status["status"] == "Completed" {
			return nil
		}
		
		if status["status"] == "Failed" || status["status"] == "Terminated" {
			return fmt.Errorf("workflow failed with status: %s", status["status"])
		}
		
		time.Sleep(2 * time.Second)
	}
	
	return fmt.Errorf("workflow %s did not complete within %v", workflowID, timeout)
}