package activities

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

// TestStoreDocumentActivityWithHybridStorage tests the StoreDocumentActivity with hybrid storage
func TestStoreDocumentActivityWithHybridStorage(t *testing.T) {
	// Set up test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-storage-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepoPath := filepath.Join(tempDir, "test-repo")
	err = os.MkdirAll(gitRepoPath, 0755)
	require.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(gitRepoPath, false)
	require.NoError(t, err)

	// Create an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(gitRepoPath, "README.md")
	err = os.WriteFile(readmePath, []byte("# Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create metrics collector
	metrics := storage.NewSimpleMetricsCollector()

	// Configure hybrid storage with govc primary
	config := storage.DefaultHybridConfig()
	config.PrimaryBackend = "govc"
	config.EnableFallback = true
	config.EnableSync = false

	// Initialize hybrid storage
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	// Set global storage for activities
	SetGlobalStorage(hybridStorage, metrics)

	// Register activity
	env.RegisterActivity(StoreDocumentActivity)

	// Create test input
	input := workflows.StoreInput{
		URL:     "https://example.com/test.txt",
		Type:    "text",
		Content: []byte("Test document content"),
		Text:    "Test document content",
		Metadata: map[string]string{
			"title":  "Test Document",
			"source": "unit test",
		},
		Embeddings: []float32{0.1, 0.2, 0.3, 0.4},
	}

	// Execute activity
	var commitHash string
	future, err := env.ExecuteActivity(StoreDocumentActivity, input)
	require.NoError(t, err)
	err = future.Get(&commitHash)

	// Verify results
	assert.NoError(t, err)
	assert.NotEmpty(t, commitHash)

	// Verify metrics were recorded
	summary := metrics.GetMetricsSummary()
	assert.NotNil(t, summary)

	// Verify stats
	stats := hybridStorage.GetStats()
	assert.NotNil(t, stats)

	t.Logf("Commit hash: %s", commitHash)
	t.Logf("Metrics: %+v", summary)
	t.Logf("Storage stats: %+v", stats)
}

// TestMergeBranchActivityWithHybridStorage tests the MergeBranchActivity with hybrid storage
func TestMergeBranchActivityWithHybridStorage(t *testing.T) {
	// Set up test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-merge-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepoPath := filepath.Join(tempDir, "test-repo")
	err = os.MkdirAll(gitRepoPath, 0755)
	require.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(gitRepoPath, false)
	require.NoError(t, err)

	// Create an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(gitRepoPath, "README.md")
	err = os.WriteFile(readmePath, []byte("# Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create metrics collector
	metrics := storage.NewSimpleMetricsCollector()

	// Configure hybrid storage
	config := storage.DefaultHybridConfig()
	config.PrimaryBackend = "govc"
	config.EnableFallback = true
	config.EnableSync = false

	// Initialize hybrid storage
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	// Set global storage for activities
	SetGlobalStorage(hybridStorage, metrics)

	// Register activity
	env.RegisterActivity(MergeBranchActivity)

	// Test branch merge
	branchName := "test-branch"
	future, err := env.ExecuteActivity(MergeBranchActivity, branchName)
	require.NoError(t, err)
	
	// For merge activities that return no data, we just check if they complete successfully
	// The activity itself will log success/failure
	_ = future

	t.Logf("Successfully merged branch: %s", branchName)
}

// TestStorageWorkflowIntegration tests the complete workflow with hybrid storage
func TestStorageWorkflowIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Set up test environment
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestActivityEnvironment()

	// Create temporary directory for git repository
	tempDir, err := os.MkdirTemp("", "caia-workflow-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize git repository
	gitRepoPath := filepath.Join(tempDir, "test-repo")
	err = os.MkdirAll(gitRepoPath, 0755)
	require.NoError(t, err)

	// Initialize git repo
	repo, err := git.PlainInit(gitRepoPath, false)
	require.NoError(t, err)

	// Create an initial commit
	worktree, err := repo.Worktree()
	require.NoError(t, err)

	readmePath := filepath.Join(gitRepoPath, "README.md")
	err = os.WriteFile(readmePath, []byte("# Test Repository\n"), 0644)
	require.NoError(t, err)

	_, err = worktree.Add("README.md")
	require.NoError(t, err)

	_, err = worktree.Commit("Initial commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Test User",
			Email: "test@caiatech.com",
			When:  time.Now(),
		},
	})
	require.NoError(t, err)

	// Create metrics collector
	metrics := storage.NewSimpleMetricsCollector()

	// Configure hybrid storage
	config := storage.DefaultHybridConfig()
	config.PrimaryBackend = "govc"
	config.EnableFallback = true
	config.EnableSync = false

	// Initialize hybrid storage
	hybridStorage, err := storage.NewHybridStorage(gitRepoPath, "test-repo", config, metrics)
	require.NoError(t, err)
	defer hybridStorage.Close()

	// Set global storage for activities
	SetGlobalStorage(hybridStorage, metrics)

	// Register activities
	env.RegisterActivity(StoreDocumentActivity)
	env.RegisterActivity(MergeBranchActivity)

	// Test complete workflow: Store -> Merge
	storeInput := workflows.StoreInput{
		URL:     "https://example.com/workflow-test.txt",
		Type:    "text",
		Content: []byte("Workflow integration test content"),
		Text:    "Workflow integration test content",
		Metadata: map[string]string{
			"title":       "Workflow Test",
			"document_id": "workflow-test-001",
			"test_type":   "integration",
		},
		Embeddings: []float32{0.5, 0.6, 0.7, 0.8},
	}

	// Step 1: Store document
	var commitHash string
	future1, err := env.ExecuteActivity(StoreDocumentActivity, storeInput)
	require.NoError(t, err)
	err = future1.Get(&commitHash)
	require.NoError(t, err)
	require.NotEmpty(t, commitHash)

	// Step 2: Merge branch
	branchName := "ingest/workflow-test-001"
	future2, err := env.ExecuteActivity(MergeBranchActivity, branchName)
	require.NoError(t, err)
	
	// For merge activities that return no data, we just verify they executed without error
	_ = future2

	// Verify final state
	summary := metrics.GetMetricsSummary()
	stats := hybridStorage.GetStats()

	assert.NotNil(t, summary)
	assert.NotNil(t, stats)

	// Log results
	t.Logf("Workflow completed successfully")
	t.Logf("Commit hash: %s", commitHash)
	t.Logf("Metrics summary: %+v", summary)
	t.Logf("Storage stats: %+v", stats)

	// Check that operations were recorded
	if operations, ok := summary["total_operations"]; ok {
		assert.Greater(t, operations, 0)
	}
}