package api

import (
	"fmt"
	"log"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"github.com/Caia-Tech/caia-library/pkg/gql"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// Handlers contains the HTTP handlers for the API
type Handlers struct {
	temporal client.Client
	repoPath string
}

// NewHandlers creates a new handlers instance
func NewHandlers(temporal client.Client, repoPath string) *Handlers {
	return &Handlers{
		temporal: temporal,
		repoPath: repoPath,
	}
}

// Health returns the service health status
func (h *Handlers) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"service":   "caia-library",
		"version":   "0.1.0",
		"timestamp": time.Now().UTC(),
	})
}

// IngestDocumentRequest represents a document ingestion request
type IngestDocumentRequest struct {
	URL      string            `json:"url" validate:"required,url"`
	Type     string            `json:"type" validate:"required"`
	Metadata map[string]string `json:"metadata"`
}

// IngestDocumentResponse represents the response for document ingestion
type IngestDocumentResponse struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
}

// IngestDocument starts a new document ingestion workflow
func (h *Handlers) IngestDocument(c *fiber.Ctx) error {
	var req IngestDocumentRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
			"details": err.Error(),
		})
	}

	// Validate request
	if req.URL == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "URL is required",
		})
	}

	if req.Type == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Document type is required",
		})
	}

	// Validate document type
	validTypes := map[string]bool{
		"text": true,
		"html": true,
		"pdf":  true,
	}
	if !validTypes[req.Type] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": fmt.Sprintf("Invalid document type: %s. Valid types are: text, html, pdf", req.Type),
		})
	}

	// Generate workflow ID
	workflowID := fmt.Sprintf("ingest-%s", uuid.New().String())

	// Start workflow
	we, err := h.temporal.ExecuteWorkflow(c.Context(), client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "caia-library",
	}, workflows.DocumentIngestionWorkflow, workflows.DocumentInput{
		URL:      req.URL,
		Type:     req.Type,
		Metadata: req.Metadata,
	})
	if err != nil {
		log.Printf("Failed to start workflow: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start document ingestion",
			"details": err.Error(),
		})
	}

	log.Printf("Started document ingestion workflow: %s for URL: %s", workflowID, req.URL)

	return c.Status(fiber.StatusAccepted).JSON(IngestDocumentResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
	})
}

// GetDocument retrieves a document by ID
func (h *Handlers) GetDocument(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Document ID is required",
		})
	}

	// TODO: Implement document retrieval from Git repository
	// This would involve:
	// 1. Opening the Git repository
	// 2. Finding the document by ID
	// 3. Reading the metadata, text, and embeddings
	// 4. Returning the document

	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{
		"error": "Document retrieval not yet implemented",
		"id":    id,
	})
}

// ListDocumentsRequest represents query parameters for listing documents
type ListDocumentsRequest struct {
	Page  int    `query:"page" validate:"min=1"`
	Limit int    `query:"limit" validate:"min=1,max=100"`
	Type  string `query:"type"`
}

// ListDocuments returns a paginated list of documents
func (h *Handlers) ListDocuments(c *fiber.Ctx) error {
	var req ListDocumentsRequest
	
	// Parse query parameters
	if err := c.QueryParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
			"details": err.Error(),
		})
	}

	// Set defaults
	if req.Page == 0 {
		req.Page = 1
	}
	if req.Limit == 0 {
		req.Limit = 20
	}

	// TODO: Implement document listing from Git repository
	// This would involve:
	// 1. Opening the Git repository
	// 2. Walking through the documents directory
	// 3. Filtering by type if specified
	// 4. Implementing pagination
	// 5. Reading metadata for each document

	return c.JSON(fiber.Map{
		"documents": []interface{}{},
		"pagination": fiber.Map{
			"page":  req.Page,
			"limit": req.Limit,
			"total": 0,
		},
	})
}

// WorkflowStatusResponse represents the workflow status
type WorkflowStatusResponse struct {
	WorkflowID string                 `json:"workflow_id"`
	Status     string                 `json:"status"`
	StartTime  time.Time              `json:"start_time"`
	CloseTime  *time.Time             `json:"close_time,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Result     map[string]interface{} `json:"result,omitempty"`
}

// GetWorkflow returns the status of a workflow
func (h *Handlers) GetWorkflow(c *fiber.Ctx) error {
	workflowID := c.Params("id")
	if workflowID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Workflow ID is required",
		})
	}

	// Describe workflow execution
	resp, err := h.temporal.DescribeWorkflowExecution(c.Context(), workflowID, "")
	if err != nil {
		log.Printf("Failed to describe workflow %s: %v", workflowID, err)
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Workflow not found",
			"workflow_id": workflowID,
		})
	}

	response := WorkflowStatusResponse{
		WorkflowID: workflowID,
		Status:     resp.WorkflowExecutionInfo.Status.String(),
		StartTime:  resp.WorkflowExecutionInfo.StartTime.AsTime(),
	}

	if resp.WorkflowExecutionInfo.CloseTime != nil {
		closeTime := resp.WorkflowExecutionInfo.CloseTime.AsTime()
		response.CloseTime = &closeTime
	}

	// Add error message if workflow failed
	if resp.WorkflowExecutionInfo.Status.String() == "Failed" {
		response.Error = "Workflow failed - check Temporal UI for details"
	}

	return c.JSON(response)
}

// ScheduledIngestionRequest represents a scheduled ingestion source
type ScheduledIngestionRequest struct {
	Name     string            `json:"name" validate:"required"`
	Type     string            `json:"type" validate:"required,oneof=rss web api"`
	URL      string            `json:"url" validate:"required,url"`
	Schedule string            `json:"schedule" validate:"required"` // Cron expression
	Filters  []string          `json:"filters"`
	Metadata map[string]string `json:"metadata"`
}

// ScheduledIngestionResponse represents the response for scheduled ingestion
type ScheduledIngestionResponse struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Schedule   string `json:"schedule"`
}

// CreateScheduledIngestion creates a new scheduled ingestion workflow
func (h *Handlers) CreateScheduledIngestion(c *fiber.Ctx) error {
	var req ScheduledIngestionRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
			"details": err.Error(),
		})
	}

	// Validate request
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name is required",
		})
	}

	// Generate workflow ID
	workflowID := fmt.Sprintf("scheduled-%s-%s", req.Name, uuid.New().String())

	// Start scheduled workflow
	we, err := h.temporal.ExecuteWorkflow(c.Context(), client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "caia-library",
	}, workflows.ScheduledIngestionWorkflow, workflows.ScheduledIngestionInput{
		Name:     req.Name,
		Type:     req.Type,
		URL:      req.URL,
		Schedule: req.Schedule,
		Filters:  req.Filters,
		Metadata: req.Metadata,
	})
	if err != nil {
		log.Printf("Failed to start scheduled workflow: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start scheduled ingestion",
			"details": err.Error(),
		})
	}

	log.Printf("Started scheduled ingestion workflow: %s for source: %s", workflowID, req.Name)

	return c.Status(fiber.StatusCreated).JSON(ScheduledIngestionResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Schedule:   req.Schedule,
	})
}

// BatchIngestionRequest represents a batch of documents to ingest
type BatchIngestionRequest struct {
	Documents []IngestDocumentRequest `json:"documents" validate:"required,min=1"`
}

// BatchIngestionResponse represents the response for batch ingestion
type BatchIngestionResponse struct {
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
	Count      int    `json:"count"`
}

// CreateBatchIngestion creates a batch ingestion workflow
func (h *Handlers) CreateBatchIngestion(c *fiber.Ctx) error {
	var req BatchIngestionRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Failed to parse request body: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
			"details": err.Error(),
		})
	}

	// Validate request
	if len(req.Documents) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "At least one document is required",
		})
	}

	// Convert to workflow input
	var documents []workflows.DocumentInput
	for _, doc := range req.Documents {
		documents = append(documents, workflows.DocumentInput{
			URL:      doc.URL,
			Type:     doc.Type,
			Metadata: doc.Metadata,
		})
	}

	// Generate workflow ID
	workflowID := fmt.Sprintf("batch-%s", uuid.New().String())

	// Start batch workflow
	we, err := h.temporal.ExecuteWorkflow(c.Context(), client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "caia-library",
	}, workflows.BatchIngestionWorkflow, documents)
	if err != nil {
		log.Printf("Failed to start batch workflow: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start batch ingestion",
			"details": err.Error(),
		})
	}

	log.Printf("Started batch ingestion workflow: %s for %d documents", workflowID, len(documents))

	return c.Status(fiber.StatusAccepted).JSON(BatchIngestionResponse{
		WorkflowID: we.GetID(),
		RunID:      we.GetRunID(),
		Count:      len(documents),
	})
}

// QueryRequest represents a GQL query request
type QueryRequest struct {
	Query string `json:"query" validate:"required"`
}

// QueryResponse represents a GQL query response
type QueryResponse struct {
	Type    string        `json:"type"`
	Count   int           `json:"count"`
	Items   []interface{} `json:"items"`
	Elapsed int64         `json:"elapsed_ms"`
	Query   string        `json:"query"`
}

// ExecuteQuery executes a Git Query Language query
func (h *Handlers) ExecuteQuery(c *fiber.Ctx) error {
	var req QueryRequest

	// Parse request body
	if err := c.BodyParser(&req); err != nil {
		log.Printf("Failed to parse query request: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
	}

	// Validate query
	if req.Query == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Query cannot be empty",
		})
	}

	log.Printf("Executing GQL query: %s", req.Query)

	// Create executor
	executor := gql.NewExecutor(h.repoPath)

	// Execute query
	result, err := executor.Execute(c.Context(), req.Query)
	if err != nil {
		log.Printf("Query execution failed: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   "Query execution failed",
			"details": err.Error(),
			"query":   req.Query,
		})
	}

	// Return results
	return c.JSON(QueryResponse{
		Type:    string(result.Type),
		Count:   result.Count,
		Items:   result.Items,
		Elapsed: result.Elapsed.Milliseconds(),
		Query:   req.Query,
	})
}

// GetQueryExamples returns example GQL queries
func (h *Handlers) GetQueryExamples(c *fiber.Ctx) error {
	examples := make([]fiber.Map, len(gql.QueryExamples))
	for i, ex := range gql.QueryExamples {
		examples[i] = fiber.Map{
			"name":        ex.Name,
			"query":       ex.Query,
			"description": ex.Description,
		}
	}

	return c.JSON(fiber.Map{
		"examples": examples,
		"syntax": fiber.Map{
			"select":    "SELECT FROM <type> WHERE <conditions> ORDER BY <field> [DESC] LIMIT <n>",
			"types":     []string{"documents", "attribution", "sources", "authors"},
			"operators": []string{"=", "!=", "~", ">", "<", "exists", "not exists"},
		},
	})
}

// GetAttributionStats returns attribution compliance statistics
func (h *Handlers) GetAttributionStats(c *fiber.Ctx) error {
	// Execute attribution query
	executor := gql.NewExecutor(h.repoPath)
	result, err := executor.Execute(c.Context(), gql.ExampleAttributionCompliance)
	if err != nil {
		log.Printf("Failed to get attribution stats: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve attribution statistics",
		})
	}

	// Calculate compliance percentage
	totalSources := result.Count
	compliantSources := 0
	
	for _, item := range result.Items {
		if attr, ok := item.(*gql.AttributionResult); ok && attr.CAIAAttribution {
			compliantSources++
		}
	}

	complianceRate := float64(0)
	if totalSources > 0 {
		complianceRate = float64(compliantSources) / float64(totalSources) * 100
	}

	return c.JSON(fiber.Map{
		"total_sources":     totalSources,
		"compliant_sources": compliantSources,
		"compliance_rate":   fmt.Sprintf("%.1f%%", complianceRate),
		"attribution_text":  "Content collected by Caia Tech (https://caiatech.com)",
		"policy":            "All documents must include proper attribution to both source and Caia Tech",
	})
}