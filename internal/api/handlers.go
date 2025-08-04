package api

import (
	"fmt"
	"log"
	"time"

	"github.com/caiatech/caia-library/internal/temporal/workflows"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// Handlers contains the HTTP handlers for the API
type Handlers struct {
	temporal client.Client
}

// NewHandlers creates a new handlers instance
func NewHandlers(temporal client.Client) *Handlers {
	return &Handlers{
		temporal: temporal,
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