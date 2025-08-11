package api

import (
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/gofiber/fiber/v2"
)

// StorageHandler provides HTTP endpoints for storage system monitoring and control
type StorageHandler struct {
	hybridStorage *storage.HybridStorage
	metrics       *storage.SimpleMetricsCollector
}

// NewStorageHandler creates a new storage handler
func NewStorageHandler(hybridStorage *storage.HybridStorage, metrics *storage.SimpleMetricsCollector) *StorageHandler {
	return &StorageHandler{
		hybridStorage: hybridStorage,
		metrics:       metrics,
	}
}

// GetStorageStats returns current storage system statistics
func (h *StorageHandler) GetStorageStats(c *fiber.Ctx) error {
	stats := h.hybridStorage.GetStats()
	return c.JSON(fiber.Map{
		"storage_stats": stats,
	})
}

// GetStorageMetrics returns detailed performance metrics
func (h *StorageHandler) GetStorageMetrics(c *fiber.Ctx) error {
	summary := h.metrics.GetMetricsSummary()
	return c.JSON(fiber.Map{
		"metrics_summary": summary,
		"total_operations": len(h.metrics.GetMetrics()),
	})
}

// GetStorageHealth checks the health of both storage backends
func (h *StorageHandler) GetStorageHealth(c *fiber.Ctx) error {
	ctx := c.Context()
	err := h.hybridStorage.Health(ctx)
	
	if err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"healthy": false,
			"error":   err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"healthy": true,
		"status":  "All storage backends are healthy",
	})
}

// ClearMetrics clears all collected metrics (useful for testing)
func (h *StorageHandler) ClearMetrics(c *fiber.Ctx) error {
	h.metrics.ClearMetrics()
	return c.JSON(fiber.Map{
		"message": "Metrics cleared successfully",
	})
}