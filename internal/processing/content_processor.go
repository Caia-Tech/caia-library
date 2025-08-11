package processing

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Caia-Tech/caia-library/internal/pipeline"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/rs/zerolog/log"
)

// ContentProcessorConfig configures the content processor
type ContentProcessorConfig struct {
	Enabled             bool          `json:"enabled"`
	ProcessInBackground bool          `json:"process_in_background"`
	BatchSize           int           `json:"batch_size"`
	ProcessingTimeout   time.Duration `json:"processing_timeout"`
	StrictMode          bool          `json:"strict_mode"`
	PreserveStructure   bool          `json:"preserve_structure"`
	EnabledRules        []string      `json:"enabled_rules,omitempty"`
	DisabledRules       []string      `json:"disabled_rules,omitempty"`
}

// DefaultContentProcessorConfig returns default configuration
func DefaultContentProcessorConfig() *ContentProcessorConfig {
	return &ContentProcessorConfig{
		Enabled:             true,
		ProcessInBackground: true,
		BatchSize:           10,
		ProcessingTimeout:   30 * time.Second,
		StrictMode:          false,
		PreserveStructure:   true,
	}
}

// ContentProcessorStats tracks processing statistics
type ContentProcessorStats struct {
	DocumentsProcessed  int64         `json:"documents_processed"`
	DocumentsFailed     int64         `json:"documents_failed"`
	TotalBytesProcessed int64         `json:"total_bytes_processed"`
	TotalBytesRemoved   int64         `json:"total_bytes_removed"`
	AverageProcessTime  time.Duration `json:"average_process_time"`
	LastProcessed       time.Time     `json:"last_processed"`
	QueueSize           int           `json:"queue_size"`
}

// ContentProcessor automatically processes and cleans document content
type ContentProcessor struct {
	config       *ContentProcessorConfig
	cleaner      *ContentCleaner
	storage      storage.StorageBackend
	eventBus     *pipeline.EventBus
	subscription *pipeline.Subscription
	
	// Processing queue and workers
	processQueue chan *document.Document
	workers      int
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	
	// Statistics
	mu    sync.RWMutex
	stats ContentProcessorStats
}

// NewContentProcessor creates a new content processor
func NewContentProcessor(storage storage.StorageBackend, eventBus *pipeline.EventBus, config *ContentProcessorConfig) (*ContentProcessor, error) {
	if config == nil {
		config = DefaultContentProcessorConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	processor := &ContentProcessor{
		config:       config,
		cleaner:      NewContentCleaner(),
		storage:      storage,
		eventBus:     eventBus,
		processQueue: make(chan *document.Document, config.BatchSize*2),
		workers:      4, // Default 4 workers
		ctx:          ctx,
		cancel:       cancel,
		stats:        ContentProcessorStats{},
	}
	
	// Configure cleaner based on config
	processor.cleaner.SetStrictMode(config.StrictMode)
	processor.cleaner.SetPreserveStructure(config.PreserveStructure)
	
	// Enable/disable specific rules
	if len(config.DisabledRules) > 0 {
		for _, ruleName := range config.DisabledRules {
			processor.cleaner.DisableRule(ruleName)
		}
	}
	
	if len(config.EnabledRules) > 0 {
		// If specific rules are enabled, disable all others first
		for _, ruleName := range processor.cleaner.GetEnabledRules() {
			processor.cleaner.DisableRule(ruleName)
		}
		for _, ruleName := range config.EnabledRules {
			processor.cleaner.EnableRule(ruleName)
		}
	}
	
	// Subscribe to document events if enabled
	if config.Enabled {
		if err := processor.subscribeToEvents(); err != nil {
			return nil, err
		}
	}
	
	// Start worker goroutines
	processor.startWorkers()
	
	log.Info().
		Bool("enabled", config.Enabled).
		Int("workers", processor.workers).
		Int("batch_size", config.BatchSize).
		Msg("Content processor started")
	
	return processor, nil
}

// subscribeToEvents subscribes to document events
func (cp *ContentProcessor) subscribeToEvents() error {
	handler := func(ctx context.Context, event *pipeline.DocumentEvent) error {
		defer func() {
			if r := recover(); r != nil {
				log.Error().
					Interface("panic", r).
					Interface("event", event).
					Msg("Event handler panic recovered")
			}
		}()
		
		// Only process newly added documents
		if event.Type != pipeline.EventDocumentAdded {
			return nil
		}
		
		if event.Document == nil {
			log.Warn().Msg("Received document event with nil document")
			return nil
		}
		
		// Validate document before queuing
		if event.Document.ID == "" {
			log.Warn().Msg("Received document event with empty document ID")
			return nil
		}
		
		// Queue document for processing with timeout
		queueTimeout := 5 * time.Second
		queueCtx, cancel := context.WithTimeout(ctx, queueTimeout)
		defer cancel()
		
		select {
		case cp.processQueue <- event.Document:
			log.Debug().
				Str("document_id", event.Document.ID).
				Msg("Document queued for processing")
			return nil
		case <-queueCtx.Done():
			if queueCtx.Err() == context.DeadlineExceeded {
				log.Warn().
					Str("document_id", event.Document.ID).
					Dur("timeout", queueTimeout).
					Int("queue_size", len(cp.processQueue)).
					Msg("Failed to queue document - queue full or timeout")
				
				// Update failed stats
				cp.updateStats(0, 0, 0, false)
			}
			return queueCtx.Err()
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	subscription, err := cp.eventBus.Subscribe(
		[]pipeline.EventType{pipeline.EventDocumentAdded},
		handler,
		cp.config.BatchSize,
	)
	if err != nil {
		return err
	}
	
	cp.subscription = subscription
	return nil
}

// startWorkers starts the worker goroutines
func (cp *ContentProcessor) startWorkers() {
	for i := 0; i < cp.workers; i++ {
		cp.wg.Add(1)
		go cp.worker(i)
	}
}

// worker processes documents from the queue
func (cp *ContentProcessor) worker(workerID int) {
	defer cp.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Int("worker_id", workerID).
				Interface("panic", r).
				Msg("Content processor worker panic recovered")
			
			// Update failed document count
			cp.updateStats(0, 0, 0, false)
		}
	}()
	
	log.Debug().Int("worker_id", workerID).Msg("Content processor worker started")
	
	for {
		select {
		case doc := <-cp.processQueue:
			if doc != nil {
				cp.processDocumentSafely(doc, workerID)
			}
		case <-cp.ctx.Done():
			log.Debug().Int("worker_id", workerID).Msg("Content processor worker stopping")
			return
		}
	}
}

// processDocumentSafely processes a document with error recovery
func (cp *ContentProcessor) processDocumentSafely(doc *document.Document, workerID int) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Str("document_id", doc.ID).
				Int("worker_id", workerID).
				Interface("panic", r).
				Msg("Document processing panic recovered")
			
			// Update stats for failed processing
			cp.updateStats(0, 0, 0, false)
			
			// Publish error event
			errorEvent := pipeline.NewDocumentEvent(pipeline.EventProcessingFailed, doc)
			errorEvent.Metadata["error"] = fmt.Sprintf("Processing panic: %v", r)
			errorEvent.Metadata["worker_id"] = workerID
			
			if err := cp.eventBus.Publish(errorEvent); err != nil {
				log.Warn().Err(err).Str("document_id", doc.ID).Msg("Failed to publish error event")
			}
		}
	}()
	
	cp.processDocument(doc)
}

// processDocument processes a single document
func (cp *ContentProcessor) processDocument(doc *document.Document) {
	start := time.Now()
	
	// Create processing context with timeout
	ctx, cancel := context.WithTimeout(cp.ctx, cp.config.ProcessingTimeout)
	defer cancel()
	
	log.Debug().
		Str("document_id", doc.ID).
		Str("document_type", doc.Source.Type).
		Int("content_length", len(doc.Content.Text)).
		Msg("Processing document content")
	
	// Clean document content with timeout handling
	result, err := cp.cleanDocumentWithTimeout(ctx, doc)
	if err != nil {
		cp.updateStats(0, 0, time.Since(start), false)
		
		// Check if error is due to timeout
		if ctx.Err() == context.DeadlineExceeded {
			log.Error().
				Str("document_id", doc.ID).
				Dur("timeout", cp.config.ProcessingTimeout).
				Msg("Document cleaning timed out")
		} else {
			log.Error().
				Err(err).
				Str("document_id", doc.ID).
				Msg("Failed to clean document content")
		}
		
		// Publish error event
		errorEvent := pipeline.NewDocumentEvent(pipeline.EventProcessingFailed, doc)
		errorEvent.Metadata["error"] = err.Error()
		errorEvent.Metadata["stage"] = "cleaning"
		
		if publishErr := cp.eventBus.Publish(errorEvent); publishErr != nil {
			log.Warn().Err(publishErr).Str("document_id", doc.ID).Msg("Failed to publish error event")
		}
		return
	}
	
	// Update document in storage if content changed
	if result.OriginalLength != result.CleanedLength {
		if err := cp.updateDocumentInStorageWithRetry(ctx, doc, 3); err != nil {
			cp.updateStats(0, 0, time.Since(start), false)
			
			log.Error().
				Err(err).
				Str("document_id", doc.ID).
				Msg("Failed to update cleaned document in storage after retries")
			
			// Publish error event for storage failure
			errorEvent := pipeline.NewDocumentEvent(pipeline.EventProcessingFailed, doc)
			errorEvent.Metadata["error"] = err.Error()
			errorEvent.Metadata["stage"] = "storage_update"
			
			if publishErr := cp.eventBus.Publish(errorEvent); publishErr != nil {
				log.Warn().Err(publishErr).Str("document_id", doc.ID).Msg("Failed to publish error event")
			}
			return
		}
	}
	
	// Publish processing complete event with error handling
	event := pipeline.NewDocumentEvent(pipeline.EventDocumentCleaned, doc)
	event.Metadata["original_length"] = result.OriginalLength
	event.Metadata["cleaned_length"] = result.CleanedLength
	event.Metadata["bytes_removed"] = result.BytesRemoved
	event.Metadata["rules_applied"] = result.RulesApplied
	event.Metadata["processing_time_ms"] = result.ProcessingTime.Milliseconds()
	
	// Try to publish event with timeout
	publishCtx, publishCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer publishCancel()
	
	publishDone := make(chan error, 1)
	go func() {
		publishDone <- cp.eventBus.Publish(event)
	}()
	
	select {
	case err := <-publishDone:
		if err != nil {
			log.Warn().Err(err).Str("document_id", doc.ID).Msg("Failed to publish cleaning event")
		}
	case <-publishCtx.Done():
		log.Warn().Str("document_id", doc.ID).Msg("Publishing cleaning event timed out")
	}
	
	cp.updateStats(result.OriginalLength, result.BytesRemoved, time.Since(start), true)
	
	log.Debug().
		Str("document_id", doc.ID).
		Int("bytes_removed", result.BytesRemoved).
		Dur("processing_time", result.ProcessingTime).
		Interface("rules_applied", result.RulesApplied).
		Msg("Document content cleaned successfully")
}

// cleanDocumentWithTimeout cleans document with proper timeout handling
func (cp *ContentProcessor) cleanDocumentWithTimeout(ctx context.Context, doc *document.Document) (*CleaningResult, error) {
	// Create a channel to receive the result
	resultChan := make(chan *CleaningResult, 1)
	errorChan := make(chan error, 1)
	
	// Run cleaning in a goroutine to handle timeouts properly
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errorChan <- fmt.Errorf("cleaning panic: %v", r)
			}
		}()
		
		result, err := cp.cleaner.CleanDocument(ctx, doc)
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- result
	}()
	
	// Wait for result or timeout
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errorChan:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// updateDocumentInStorageWithRetry updates the document in storage with retry logic
func (cp *ContentProcessor) updateDocumentInStorageWithRetry(ctx context.Context, doc *document.Document, maxRetries int) error {
	var lastErr error
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Create context with timeout for each attempt
		attemptCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		
		// For now, we'll store it as a new version
		// In a real system, you might want to update the existing document
		_, err := cp.storage.StoreDocument(attemptCtx, doc)
		cancel()
		
		if err == nil {
			return nil // Success
		}
		
		lastErr = err
		
		// Log retry attempt
		if attempt < maxRetries-1 {
			log.Warn().
				Err(err).
				Str("document_id", doc.ID).
				Int("attempt", attempt+1).
				Int("max_retries", maxRetries).
				Msg("Storage update failed, retrying")
			
			// Exponential backoff
			backoffDuration := time.Duration(attempt+1) * 100 * time.Millisecond
			select {
			case <-time.After(backoffDuration):
				// Continue to next attempt
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	
	return fmt.Errorf("storage update failed after %d attempts: %w", maxRetries, lastErr)
}

// updateStats updates processing statistics
func (cp *ContentProcessor) updateStats(bytesProcessed, bytesRemoved int, processingTime time.Duration, success bool) {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	
	if success {
		cp.stats.DocumentsProcessed++
		cp.stats.TotalBytesProcessed += int64(bytesProcessed)
		cp.stats.TotalBytesRemoved += int64(bytesRemoved)
		cp.stats.LastProcessed = time.Now()
		
		// Update average processing time
		if cp.stats.DocumentsProcessed > 0 {
			totalTime := cp.stats.AverageProcessTime*time.Duration(cp.stats.DocumentsProcessed-1) + processingTime
			cp.stats.AverageProcessTime = totalTime / time.Duration(cp.stats.DocumentsProcessed)
		}
	} else {
		cp.stats.DocumentsFailed++
	}
	
	cp.stats.QueueSize = len(cp.processQueue)
}

// GetStats returns current processing statistics
func (cp *ContentProcessor) GetStats() ContentProcessorStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	
	stats := cp.stats
	stats.QueueSize = len(cp.processQueue)
	return stats
}

// Close shuts down the content processor gracefully
func (cp *ContentProcessor) Close() {
	log.Info().Msg("Content processor shutting down...")
	
	// Unsubscribe from events first to stop new work
	if cp.subscription != nil {
		if err := cp.eventBus.Unsubscribe(cp.subscription.ID); err != nil {
			log.Warn().Err(err).Msg("Failed to unsubscribe from event bus")
		}
	}
	
	// Cancel context to signal workers to stop
	cp.cancel()
	
	// Wait for workers to finish with timeout
	workersDone := make(chan struct{})
	go func() {
		cp.wg.Wait()
		close(workersDone)
	}()
	
	// Wait for workers or timeout
	select {
	case <-workersDone:
		log.Debug().Msg("All workers stopped gracefully")
	case <-time.After(30 * time.Second):
		log.Warn().Msg("Timeout waiting for workers to stop")
	}
	
	// Close the processing queue safely
	if cp.processQueue != nil {
		// Drain remaining items
		remaining := len(cp.processQueue)
		if remaining > 0 {
			log.Warn().Int("remaining_items", remaining).Msg("Discarding unprocessed documents")
			for i := 0; i < remaining; i++ {
				select {
				case <-cp.processQueue:
					// Discard item
				default:
					break
				}
			}
		}
		
		close(cp.processQueue)
	}
	
	log.Info().Msg("Content processor shut down successfully")
}