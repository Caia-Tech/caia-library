package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/rs/zerolog/log"
)

// HybridStorageConfig defines configuration for the hybrid storage system
type HybridStorageConfig struct {
	// Primary backend to try first (govc or git)
	PrimaryBackend string `json:"primary_backend"`
	
	// Enable fallback to secondary backend on failure
	EnableFallback bool `json:"enable_fallback"`
	
	// Timeout for operations before falling back
	OperationTimeout time.Duration `json:"operation_timeout"`
	
	// Sync documents between backends
	EnableSync bool `json:"enable_sync"`
	
	// Sync interval for background synchronization
	SyncInterval time.Duration `json:"sync_interval"`
}

// DefaultHybridConfig returns sensible defaults for hybrid storage
func DefaultHybridConfig() *HybridStorageConfig {
	return &HybridStorageConfig{
		PrimaryBackend:   "govc",
		EnableFallback:   true,
		OperationTimeout: 5 * time.Second,
		EnableSync:       true,
		SyncInterval:     5 * time.Minute,
	}
}

// HybridStorage provides a storage layer that can use both govc and git backends
type HybridStorage struct {
	govcBackend      StorageBackend
	gitBackend       StorageBackend
	config           *HybridStorageConfig
	metricsCollector MetricsCollector
	
	// Background sync control
	syncTicker *time.Ticker
	syncStop   chan bool
	syncMutex  sync.RWMutex
}

// NewHybridStorage creates a new hybrid storage system
func NewHybridStorage(gitRepoPath, govcRepoName string, config *HybridStorageConfig, metrics MetricsCollector) (*HybridStorage, error) {
	if config == nil {
		config = DefaultHybridConfig()
	}

	gitBackend, err := NewGitBackend(gitRepoPath, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize git backend: %w", err)
	}

	govcBackend, err := NewGovcBackend(govcRepoName, metrics)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize govc backend: %w", err)
	}

	hs := &HybridStorage{
		govcBackend:      govcBackend,
		gitBackend:       gitBackend,
		config:           config,
		metricsCollector: metrics,
		syncStop:         make(chan bool, 1),
	}

	// Start background sync if enabled
	if config.EnableSync {
		hs.startBackgroundSync()
	}

	return hs, nil
}

// StoreDocument stores a document using the hybrid strategy
func (h *HybridStorage) StoreDocument(ctx context.Context, doc *document.Document) (string, error) {
	start := time.Now()
	
	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.OperationTimeout)
	defer cancel()

	// Try primary backend first
	primaryBackend := h.getPrimaryBackend()
	secondaryBackend := h.getSecondaryBackend()

	commitHash, err := primaryBackend.StoreDocument(timeoutCtx, doc)
	if err != nil && h.config.EnableFallback {
		log.Warn().
			Err(err).
			Str("primary", h.config.PrimaryBackend).
			Msg("Primary backend failed, trying fallback")
		
		// Try secondary backend
		commitHash, err = secondaryBackend.StoreDocument(timeoutCtx, doc)
		if err == nil {
			h.recordHybridMetric("store", start, true, "fallback_success")
		} else {
			h.recordHybridMetric("store", start, false, "both_failed")
		}
	} else if err == nil {
		h.recordHybridMetric("store", start, true, "primary_success")
	} else {
		h.recordHybridMetric("store", start, false, "primary_failed_no_fallback")
	}

	// For successful stores, try to sync to other backend in background if enabled
	if err == nil && h.config.EnableSync {
		go h.backgroundStoreSync(doc, primaryBackend, secondaryBackend)
	}

	return commitHash, err
}

// GetDocument retrieves a document using the hybrid strategy
func (h *HybridStorage) GetDocument(ctx context.Context, id string) (*document.Document, error) {
	start := time.Now()
	
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.OperationTimeout)
	defer cancel()

	primaryBackend := h.getPrimaryBackend()
	secondaryBackend := h.getSecondaryBackend()

	doc, err := primaryBackend.GetDocument(timeoutCtx, id)
	if err != nil && h.config.EnableFallback {
		log.Debug().
			Err(err).
			Str("id", id).
			Str("primary", h.config.PrimaryBackend).
			Msg("Primary backend failed, trying fallback")
		
		doc, err = secondaryBackend.GetDocument(timeoutCtx, id)
		if err == nil {
			h.recordHybridMetric("get", start, true, "fallback_success")
		} else {
			h.recordHybridMetric("get", start, false, "both_failed")
		}
	} else if err == nil {
		h.recordHybridMetric("get", start, true, "primary_success")
	} else {
		h.recordHybridMetric("get", start, false, "primary_failed_no_fallback")
	}

	return doc, err
}

// MergeBranch merges a branch using the hybrid strategy
func (h *HybridStorage) MergeBranch(ctx context.Context, branchName string) error {
	start := time.Now()
	
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.OperationTimeout)
	defer cancel()

	primaryBackend := h.getPrimaryBackend()
	secondaryBackend := h.getSecondaryBackend()

	err := primaryBackend.MergeBranch(timeoutCtx, branchName)
	if err != nil && h.config.EnableFallback {
		log.Warn().
			Err(err).
			Str("branch", branchName).
			Str("primary", h.config.PrimaryBackend).
			Msg("Primary backend merge failed, trying fallback")
		
		err = secondaryBackend.MergeBranch(timeoutCtx, branchName)
		if err == nil {
			h.recordHybridMetric("merge", start, true, "fallback_success")
		} else {
			h.recordHybridMetric("merge", start, false, "both_failed")
		}
	} else if err == nil {
		h.recordHybridMetric("merge", start, true, "primary_success")
	} else {
		h.recordHybridMetric("merge", start, false, "primary_failed_no_fallback")
	}

	return err
}

// ListDocuments lists documents using the primary backend
func (h *HybridStorage) ListDocuments(ctx context.Context, filters map[string]string) ([]*document.Document, error) {
	start := time.Now()
	
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.OperationTimeout)
	defer cancel()

	primaryBackend := h.getPrimaryBackend()
	
	documents, err := primaryBackend.ListDocuments(timeoutCtx, filters)
	if err == nil {
		h.recordHybridMetric("list", start, true, "primary_success")
	} else {
		h.recordHybridMetric("list", start, false, "primary_failed")
	}

	return documents, err
}

// Health checks the health of both backends
func (h *HybridStorage) Health(ctx context.Context) error {
	start := time.Now()
	
	timeoutCtx, cancel := context.WithTimeout(ctx, h.config.OperationTimeout)
	defer cancel()

	govcErr := h.govcBackend.Health(timeoutCtx)
	gitErr := h.gitBackend.Health(timeoutCtx)

	if govcErr == nil && gitErr == nil {
		h.recordHybridMetric("health", start, true, "both_healthy")
		return nil
	} else if govcErr == nil {
		h.recordHybridMetric("health", start, true, "govc_healthy_git_failed")
		return nil // At least one backend is healthy
	} else if gitErr == nil {
		h.recordHybridMetric("health", start, true, "git_healthy_govc_failed")
		return nil // At least one backend is healthy
	} else {
		h.recordHybridMetric("health", start, false, "both_failed")
		return fmt.Errorf("both backends unhealthy - govc: %v, git: %v", govcErr, gitErr)
	}
}

// GetStats returns statistics about the hybrid storage system
func (h *HybridStorage) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"config": h.config,
	}

	// Add govc-specific stats if available
	if govcBackend, ok := h.govcBackend.(*GovcBackend); ok {
		stats["govc"] = govcBackend.GetMemoryStats()
	}

	return stats
}

// Close stops background sync and cleans up resources
func (h *HybridStorage) Close() error {
	if h.syncTicker != nil {
		h.syncTicker.Stop()
		h.syncStop <- true
		close(h.syncStop)
	}
	return nil
}

// Internal helper methods

func (h *HybridStorage) getPrimaryBackend() StorageBackend {
	if h.config.PrimaryBackend == "govc" {
		return h.govcBackend
	}
	return h.gitBackend
}

func (h *HybridStorage) getSecondaryBackend() StorageBackend {
	if h.config.PrimaryBackend == "govc" {
		return h.gitBackend
	}
	return h.govcBackend
}

func (h *HybridStorage) recordHybridMetric(operation string, start time.Time, success bool, result string) {
	if h.metricsCollector != nil {
		h.metricsCollector.RecordMetric(StorageMetrics{
			OperationType: operation,
			Duration:      time.Since(start).Nanoseconds(),
			Success:       success,
			Backend:       fmt.Sprintf("hybrid_%s", result),
			Error:         nil,
		})
	}
}

func (h *HybridStorage) startBackgroundSync() {
	h.syncTicker = time.NewTicker(h.config.SyncInterval)
	
	go func() {
		for {
			select {
			case <-h.syncTicker.C:
				h.performBackgroundSync()
			case <-h.syncStop:
				return
			}
		}
	}()
}

func (h *HybridStorage) performBackgroundSync() {
	h.syncMutex.Lock()
	defer h.syncMutex.Unlock()
	
	log.Debug().Msg("Performing background sync between storage backends")
	// TODO: Implement actual sync logic between govc and git
	// This would involve comparing documents in both backends and syncing differences
}

func (h *HybridStorage) backgroundStoreSync(doc *document.Document, primary, secondary StorageBackend) {
	ctx, cancel := context.WithTimeout(context.Background(), h.config.OperationTimeout)
	defer cancel()
	
	// Try to store in secondary backend as well
	_, err := secondary.StoreDocument(ctx, doc)
	if err != nil {
		log.Warn().
			Err(err).
			Str("document_id", doc.ID).
			Msg("Failed to sync document to secondary backend")
	} else {
		log.Debug().
			Str("document_id", doc.ID).
			Msg("Successfully synced document to secondary backend")
	}
}