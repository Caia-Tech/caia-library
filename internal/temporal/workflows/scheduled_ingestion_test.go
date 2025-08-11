package workflows

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
)

func TestScheduledIngestionWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock activities
	env.OnActivity("CollectAcademicSourcesActivity", mock.Anything, mock.Anything).Return(
		[]CollectedDocument{
			{
				ID:   "arxiv-2301.00001",
				URL:  "https://arxiv.org/pdf/2301.00001.pdf",
				Type: "pdf",
				Metadata: map[string]string{
					"source":      "arXiv",
					"attribution": "Content from arXiv.org, collected by Caia Tech",
				},
			},
			{
				ID:   "arxiv-2301.00002",
				URL:  "https://arxiv.org/pdf/2301.00002.pdf",
				Type: "pdf",
				Metadata: map[string]string{
					"source":      "arXiv",
					"attribution": "Content from arXiv.org, collected by Caia Tech",
				},
			},
		}, nil)

	env.OnActivity("CheckDuplicateActivity", mock.Anything, "arxiv-2301.00001").Return(false, nil)
	env.OnActivity("CheckDuplicateActivity", mock.Anything, "arxiv-2301.00002").Return(true, nil) // Second doc is duplicate

	// Mock child workflow
	env.OnWorkflow(DocumentIngestionWorkflow, mock.Anything, mock.Anything).Return(nil)

	// Execute workflow
	input := ScheduledIngestionInput{
		Name:     "arxiv",
		Type:     "arxiv",
		URL:      "http://export.arxiv.org/api/query",
		Schedule: "0 2 * * *",
		Filters:  []string{"cs.AI", "cs.LG"},
		Metadata: map[string]string{
			"attribution": "Caia Tech",
		},
	}

	env.ExecuteWorkflow(ScheduledIngestionWorkflow, input)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	// Verify activities were called
	env.AssertExpectations(t)

	// Verify only non-duplicate document was processed
	calls := env.GetWorkflowHistory().Events
	childWorkflowCount := 0
	for _, event := range calls {
		if event.GetEventType().String() == "EVENT_TYPE_START_CHILD_WORKFLOW_EXECUTION_INITIATED" {
			childWorkflowCount++
		}
	}
	assert.Equal(t, 1, childWorkflowCount, "Only one child workflow should be started (duplicate skipped)")
}

func TestScheduledIngestionWorkflow_AcademicSource(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Test that academic sources use the correct activity
	academicSources := []string{"arxiv", "pubmed", "doaj", "plos"}

	for _, source := range academicSources {
		t.Run(source, func(t *testing.T) {
			env := testSuite.NewTestWorkflowEnvironment()

			// Should use academic collector for these sources
			env.OnActivity("CollectAcademicSourcesActivity", mock.Anything, mock.Anything).Return(
				[]CollectedDocument{}, nil).Once()

			input := ScheduledIngestionInput{
				Name: source,
				Type: source,
			}

			env.ExecuteWorkflow(ScheduledIngestionWorkflow, input)
			require.True(t, env.IsWorkflowCompleted())
			env.AssertExpectations(t)
		})
	}
}

func TestScheduledIngestionWorkflow_NonAcademicSource(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Should use regular collector for non-academic sources
	env.OnActivity("CollectFromSourceActivity", mock.Anything, mock.Anything).Return(
		[]CollectedDocument{}, nil).Once()

	input := ScheduledIngestionInput{
		Name: "generic-rss",
		Type: "rss",
	}

	env.ExecuteWorkflow(ScheduledIngestionWorkflow, input)
	require.True(t, env.IsWorkflowCompleted())
	env.AssertExpectations(t)
}

func TestBatchIngestionWorkflow(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock child workflows
	env.OnWorkflow(DocumentIngestionWorkflow, mock.Anything, mock.Anything).Return(nil).Times(3)

	// Execute workflow
	documents := []DocumentInput{
		{URL: "https://example.com/doc1.pdf", Type: "pdf"},
		{URL: "https://example.com/doc2.pdf", Type: "pdf"},
		{URL: "https://example.com/doc3.pdf", Type: "pdf"},
	}

	env.ExecuteWorkflow(BatchIngestionWorkflow, documents)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	env.AssertExpectations(t)
}

func TestBatchIngestionWorkflow_WithErrors(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Mock child workflows - one fails
	env.OnWorkflow(DocumentIngestionWorkflow, mock.Anything, mock.Anything).Return(nil).Once()
	env.OnWorkflow(DocumentIngestionWorkflow, mock.Anything, mock.Anything).Return(assert.AnError).Once()
	env.OnWorkflow(DocumentIngestionWorkflow, mock.Anything, mock.Anything).Return(nil).Once()

	documents := []DocumentInput{
		{URL: "https://example.com/doc1.pdf", Type: "pdf"},
		{URL: "https://example.com/doc2.pdf", Type: "pdf"},
		{URL: "https://example.com/doc3.pdf", Type: "pdf"},
	}

	env.ExecuteWorkflow(BatchIngestionWorkflow, documents)

	require.True(t, env.IsWorkflowCompleted())
	err := env.GetWorkflowError()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "batch ingestion had 1 errors")
}

func TestIsAcademicSource(t *testing.T) {
	tests := []struct {
		source   string
		expected bool
	}{
		{"arxiv", true},
		{"pubmed", true},
		{"doaj", true},
		{"plos", true},
		{"semantic_scholar", true},
		{"core", true},
		{"rss", false},
		{"web", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := isAcademicSource(tt.source)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScheduledIngestionWorkflow_CronSchedule(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Set up cron schedule
	env.SetStartTime(time.Date(2024, 3, 14, 0, 0, 0, 0, time.UTC))

	env.OnActivity("CollectAcademicSourcesActivity", mock.Anything, mock.Anything).Return(
		[]CollectedDocument{}, nil)

	input := ScheduledIngestionInput{
		Name:     "arxiv",
		Type:     "arxiv",
		Schedule: "0 2 * * *", // Daily at 2 AM
	}

	env.ExecuteWorkflow(ScheduledIngestionWorkflow, input)
	
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
}