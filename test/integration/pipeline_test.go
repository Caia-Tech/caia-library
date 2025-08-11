package integration

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
)

// Mock activity implementations
func mockFetchDocument(ctx context.Context, url string) (workflows.FetchResult, error) {
	return workflows.FetchResult{Content: []byte("test content")}, nil
}

func mockExtractText(ctx context.Context, input workflows.ExtractInput) (workflows.ExtractResult, error) {
	return workflows.ExtractResult{
		Text:     "extracted text",
		Metadata: map[string]string{"pages": "1"},
	}, nil
}

func mockGenerateEmbeddings(ctx context.Context, content []byte) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func mockStoreDocument(ctx context.Context, input workflows.StoreInput) (string, error) {
	return "abc123", nil
}

func mockIndexDocument(ctx context.Context, commitHash string) error {
	return nil
}

func mockMergeBranch(ctx context.Context, commitHash string) error {
	return nil
}

type PipelineIntegrationTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	client client.Client
}

func TestPipelineIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PipelineIntegrationTestSuite))
}

func (s *PipelineIntegrationTestSuite) SetupSuite() {
	// Skip if not integration test
	if os.Getenv("INTEGRATION_TEST") != "true" {
		s.T().Skip("Skipping integration test")
	}

	// Connect to Temporal
	c, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	require.NoError(s.T(), err)
	s.client = c
}

func (s *PipelineIntegrationTestSuite) TearDownSuite() {
	if s.client != nil {
		s.client.Close()
	}
}

func (s *PipelineIntegrationTestSuite) TestFullDocumentIngestionPipeline() {
	ctx := context.Background()
	taskQueue := "test-ingestion"

	// Create worker
	w := worker.New(s.client, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	
	// Register mock activities for now
	w.RegisterActivityWithOptions(mockFetchDocument, activity.RegisterOptions{Name: workflows.FetchDocumentActivityName})
	w.RegisterActivityWithOptions(mockExtractText, activity.RegisterOptions{Name: workflows.ExtractTextActivityName})
	w.RegisterActivityWithOptions(mockGenerateEmbeddings, activity.RegisterOptions{Name: workflows.GenerateEmbeddingsActivityName})
	w.RegisterActivityWithOptions(mockStoreDocument, activity.RegisterOptions{Name: workflows.StoreDocumentActivityName})
	w.RegisterActivityWithOptions(mockIndexDocument, activity.RegisterOptions{Name: workflows.IndexDocumentActivityName})
	w.RegisterActivityWithOptions(mockMergeBranch, activity.RegisterOptions{Name: workflows.MergeBranchActivityName})

	// Start worker
	err := w.Start()
	require.NoError(s.T(), err)
	defer w.Stop()

	// Execute workflow
	input := workflows.DocumentInput{
		URL:  "https://example.com/test.pdf",
		Type: "pdf",
		Metadata: map[string]string{
			"source": "test",
		},
	}

	we, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "test-ingestion-1",
		TaskQueue: taskQueue,
	}, workflows.DocumentIngestionWorkflow, input)
	require.NoError(s.T(), err)

	// Wait for completion
	err = we.Get(ctx, nil)
	require.NoError(s.T(), err)
}

func (s *PipelineIntegrationTestSuite) TestParallelProcessing() {
	// Test that text extraction and embedding generation happen in parallel
	ctx := context.Background()
	taskQueue := "test-parallel"

	// Create worker with instrumented activities
	w := worker.New(s.client, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	
	// Track activity execution times
	extractStarted := make(chan time.Time, 1)
	embedStarted := make(chan time.Time, 1)
	
	// Mock activities that track timing
	extractTextActivity := func(ctx context.Context, input workflows.ExtractInput) (workflows.ExtractResult, error) {
		extractStarted <- time.Now()
		time.Sleep(100 * time.Millisecond) // Simulate work
		return workflows.ExtractResult{Text: "test", Metadata: map[string]string{}}, nil
	}
	w.RegisterActivityWithOptions(extractTextActivity, activity.RegisterOptions{Name: "ExtractTextActivity"})
	
	generateEmbeddingsActivity := func(ctx context.Context, content []byte) ([]float32, error) {
		embedStarted <- time.Now()
		time.Sleep(100 * time.Millisecond) // Simulate work
		return []float32{0.1, 0.2, 0.3}, nil
	}
	w.RegisterActivityWithOptions(generateEmbeddingsActivity, activity.RegisterOptions{Name: "GenerateEmbeddingsActivity"})
	
	// Register other activities
	w.RegisterActivityWithOptions(mockFetchDocument, activity.RegisterOptions{Name: workflows.FetchDocumentActivityName})
	w.RegisterActivityWithOptions(mockStoreDocument, activity.RegisterOptions{Name: workflows.StoreDocumentActivityName})
	w.RegisterActivityWithOptions(mockIndexDocument, activity.RegisterOptions{Name: workflows.IndexDocumentActivityName})
	w.RegisterActivityWithOptions(mockMergeBranch, activity.RegisterOptions{Name: workflows.MergeBranchActivityName})

	// Start worker
	err := w.Start()
	require.NoError(s.T(), err)
	defer w.Stop()

	// Execute workflow
	we, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "test-parallel-1",
		TaskQueue: taskQueue,
	}, workflows.DocumentIngestionWorkflow, workflows.DocumentInput{
		URL:  "https://example.com/test.pdf",
		Type: "pdf",
	})
	require.NoError(s.T(), err)

	// Wait for completion
	err = we.Get(ctx, nil)
	require.NoError(s.T(), err)

	// Verify activities started close to each other (parallel)
	extractTime := <-extractStarted
	embedTime := <-embedStarted
	
	timeDiff := extractTime.Sub(embedTime).Abs()
	require.Less(s.T(), timeDiff, 50*time.Millisecond, "Activities should start nearly simultaneously")
}

func (s *PipelineIntegrationTestSuite) TestWorkflowRetry() {
	// Test that workflow retries failed activities
	ctx := context.Background()
	taskQueue := "test-retry"

	w := worker.New(s.client, taskQueue, worker.Options{})
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	
	// Activity that fails first time
	attemptCount := 0
	fetchDocumentActivity := func(ctx context.Context, url string) (workflows.FetchResult, error) {
		attemptCount++
		if attemptCount == 1 {
			return workflows.FetchResult{}, errors.New("mock failure")
		}
		return workflows.FetchResult{Content: []byte("test")}, nil
	}
	w.RegisterActivityWithOptions(fetchDocumentActivity, activity.RegisterOptions{Name: "FetchDocumentActivity"})
	
	// Register other activities
	w.RegisterActivityWithOptions(mockExtractText, activity.RegisterOptions{Name: workflows.ExtractTextActivityName})
	w.RegisterActivityWithOptions(mockGenerateEmbeddings, activity.RegisterOptions{Name: workflows.GenerateEmbeddingsActivityName})
	w.RegisterActivityWithOptions(mockStoreDocument, activity.RegisterOptions{Name: workflows.StoreDocumentActivityName})
	w.RegisterActivityWithOptions(mockIndexDocument, activity.RegisterOptions{Name: workflows.IndexDocumentActivityName})
	w.RegisterActivityWithOptions(mockMergeBranch, activity.RegisterOptions{Name: workflows.MergeBranchActivityName})

	err := w.Start()
	require.NoError(s.T(), err)
	defer w.Stop()

	we, err := s.client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        "test-retry-1",
		TaskQueue: taskQueue,
	}, workflows.DocumentIngestionWorkflow, workflows.DocumentInput{
		URL:  "https://example.com/test.pdf",
		Type: "pdf",
	})
	require.NoError(s.T(), err)

	err = we.Get(ctx, nil)
	require.NoError(s.T(), err)
	require.Equal(s.T(), 2, attemptCount, "Activity should have been retried")
}