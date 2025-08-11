package scraping

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/rs/zerolog/log"
)

// DistributedCrawler manages distributed web crawling operations
type DistributedCrawler struct {
	complianceEngine *ComplianceEngine
	rateLimiter      *AdaptiveRateLimiter
	extractor        *ContentExtractor
	qualityValidator procurement.QualityValidator
	storage          storage.StorageBackend
	config           *CrawlerConfig
	
	// State management
	activeJobs    map[string]*CrawlJob
	jobsMu        sync.RWMutex
	workers       []*CrawlWorker
	jobQueue      chan *CrawlJob
	resultQueue   chan *CrawlResult
	
	// Metrics
	metrics   *CrawlMetrics
	metricsMu sync.RWMutex
}

// CrawlerConfig configures crawler behavior
type CrawlerConfig struct {
	MaxWorkers           int           `json:"max_workers"`
	MaxConcurrentDomains int           `json:"max_concurrent_domains"`
	JobQueueSize         int           `json:"job_queue_size"`
	ResultQueueSize      int           `json:"result_queue_size"`
	JobTimeout           time.Duration `json:"job_timeout"`
	RetryAttempts        int           `json:"retry_attempts"`
	RetryDelay           time.Duration `json:"retry_delay"`
	EnableDeduplication  bool          `json:"enable_deduplication"`
	QualityThreshold     float64       `json:"quality_threshold"`
	MaxPagesPerDomain    int           `json:"max_pages_per_domain"`
	CrawlDepth           int           `json:"crawl_depth"`
	RespectSitemaps      bool          `json:"respect_sitemaps"`
}

// CrawlJob represents a crawling job
type CrawlJob struct {
	ID          string                 `json:"id"`
	URL         string                 `json:"url"`
	Domain      string                 `json:"domain"`
	Depth       int                    `json:"depth"`
	Priority    int                    `json:"priority"`
	Source      string                 `json:"source"`
	Metadata    map[string]string      `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at"`
	Status      procurement.ProcessingStatus `json:"status"`
	AttemptCount int                   `json:"attempt_count"`
	LastError   string                 `json:"last_error,omitempty"`
}

// CrawlResult represents the result of a crawling job
type CrawlResult struct {
	JobID           string               `json:"job_id"`
	URL             string               `json:"url"`
	Success         bool                 `json:"success"`
	Document        *document.Document   `json:"document,omitempty"`
	QualityScore    float64              `json:"quality_score"`
	Error           string               `json:"error,omitempty"`
	StatusCode      int                  `json:"status_code"`
	ProcessingTime  time.Duration        `json:"processing_time"`
	ExtractedLinks  []string             `json:"extracted_links"`
	ComplianceResult *ComplianceResult   `json:"compliance_result"`
	CreatedAt       time.Time            `json:"created_at"`
}

// CrawlWorker represents a worker that processes crawl jobs
type CrawlWorker struct {
	ID       int
	crawler  *DistributedCrawler
	stopCh   chan struct{}
	stopped  bool
	mu       sync.Mutex
}

// CrawlMetrics tracks crawling performance and statistics
type CrawlMetrics struct {
	JobsQueued       int64             `json:"jobs_queued"`
	JobsCompleted    int64             `json:"jobs_completed"`
	JobsFailed       int64             `json:"jobs_failed"`
	DocumentsStored  int64             `json:"documents_stored"`
	BytesProcessed   int64             `json:"bytes_processed"`
	AverageQuality   float64           `json:"average_quality"`
	DomainStats      map[string]*DomainStats `json:"domain_stats"`
	WorkerStats      map[int]*WorkerStats    `json:"worker_stats"`
	LastUpdated      time.Time         `json:"last_updated"`
}

// DomainStats tracks statistics for a specific domain
type DomainStats struct {
	Domain          string    `json:"domain"`
	PagesProcessed  int64     `json:"pages_processed"`
	PagesSuccessful int64     `json:"pages_successful"`
	AverageQuality  float64   `json:"average_quality"`
	LastCrawled     time.Time `json:"last_crawled"`
}

// WorkerStats tracks statistics for a specific worker
type WorkerStats struct {
	WorkerID        int       `json:"worker_id"`
	JobsProcessed   int64     `json:"jobs_processed"`
	AverageTime     time.Duration `json:"average_time"`
	LastJobTime     time.Time `json:"last_job_time"`
}

// NewDistributedCrawler creates a new distributed crawler
func NewDistributedCrawler(
	complianceEngine *ComplianceEngine,
	rateLimiter *AdaptiveRateLimiter,
	extractor *ContentExtractor,
	qualityValidator procurement.QualityValidator,
	storage storage.StorageBackend,
	config *CrawlerConfig,
) *DistributedCrawler {
	if config == nil {
		config = DefaultCrawlerConfig()
	}
	
	dc := &DistributedCrawler{
		complianceEngine: complianceEngine,
		rateLimiter:     rateLimiter,
		extractor:       extractor,
		qualityValidator: qualityValidator,
		storage:         storage,
		config:          config,
		activeJobs:      make(map[string]*CrawlJob),
		workers:         make([]*CrawlWorker, 0, config.MaxWorkers),
		jobQueue:        make(chan *CrawlJob, config.JobQueueSize),
		resultQueue:     make(chan *CrawlResult, config.ResultQueueSize),
		metrics: &CrawlMetrics{
			DomainStats: make(map[string]*DomainStats),
			WorkerStats: make(map[int]*WorkerStats),
			LastUpdated: time.Now(),
		},
	}
	
	return dc
}

// DefaultCrawlerConfig returns default crawler configuration
func DefaultCrawlerConfig() *CrawlerConfig {
	return &CrawlerConfig{
		MaxWorkers:           10,
		MaxConcurrentDomains: 50,
		JobQueueSize:         1000,
		ResultQueueSize:      500,
		JobTimeout:           5 * time.Minute,
		RetryAttempts:        3,
		RetryDelay:           30 * time.Second,
		EnableDeduplication:  true,
		QualityThreshold:     0.6,
		MaxPagesPerDomain:    1000,
		CrawlDepth:           3,
		RespectSitemaps:      true,
	}
}

// Start starts the crawler with workers and result processor
func (dc *DistributedCrawler) Start(ctx context.Context) error {
	log.Info().
		Int("max_workers", dc.config.MaxWorkers).
		Int("job_queue_size", dc.config.JobQueueSize).
		Msg("Starting distributed crawler")
	
	// Start workers
	for i := 0; i < dc.config.MaxWorkers; i++ {
		worker := &CrawlWorker{
			ID:      i,
			crawler: dc,
			stopCh:  make(chan struct{}),
		}
		dc.workers = append(dc.workers, worker)
		go worker.Start(ctx)
		
		// Initialize worker stats
		dc.metricsMu.Lock()
		dc.metrics.WorkerStats[i] = &WorkerStats{
			WorkerID: i,
		}
		dc.metricsMu.Unlock()
	}
	
	// Start result processor
	go dc.processResults(ctx)
	
	log.Info().
		Int("workers_started", len(dc.workers)).
		Msg("Distributed crawler started")
	
	return nil
}

// Stop stops the crawler and all workers
func (dc *DistributedCrawler) Stop() error {
	log.Info().Msg("Stopping distributed crawler")
	
	// Stop all workers
	for _, worker := range dc.workers {
		worker.Stop()
	}
	
	// Close channels
	close(dc.jobQueue)
	close(dc.resultQueue)
	
	log.Info().Msg("Distributed crawler stopped")
	return nil
}

// SubmitJob submits a new crawl job
func (dc *DistributedCrawler) SubmitJob(ctx context.Context, job *CrawlJob) error {
	job.Status = procurement.StatusPending
	job.CreatedAt = time.Now()
	
	// Track job
	dc.jobsMu.Lock()
	dc.activeJobs[job.ID] = job
	dc.jobsMu.Unlock()
	
	// Update metrics
	dc.metricsMu.Lock()
	dc.metrics.JobsQueued++
	dc.metricsMu.Unlock()
	
	// Submit to queue
	select {
	case dc.jobQueue <- job:
		log.Debug().
			Str("job_id", job.ID).
			Str("url", job.URL).
			Msg("Job submitted to queue")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("job queue full")
	}
}

// SubmitBatch submits multiple crawl jobs
func (dc *DistributedCrawler) SubmitBatch(ctx context.Context, jobs []*CrawlJob) error {
	for _, job := range jobs {
		if err := dc.SubmitJob(ctx, job); err != nil {
			return fmt.Errorf("failed to submit job %s: %w", job.ID, err)
		}
	}
	return nil
}

// GetJobStatus returns the status of a crawl job
func (dc *DistributedCrawler) GetJobStatus(jobID string) *CrawlJob {
	dc.jobsMu.RLock()
	defer dc.jobsMu.RUnlock()
	
	if job, exists := dc.activeJobs[jobID]; exists {
		// Return a copy to prevent race conditions
		jobCopy := *job
		return &jobCopy
	}
	
	return nil
}

// Start starts a crawl worker
func (cw *CrawlWorker) Start(ctx context.Context) {
	log.Debug().Int("worker_id", cw.ID).Msg("Starting crawl worker")
	
	for {
		select {
		case job := <-cw.crawler.jobQueue:
			if job == nil {
				return // Channel closed
			}
			cw.processJob(ctx, job)
			
		case <-cw.stopCh:
			log.Debug().Int("worker_id", cw.ID).Msg("Crawl worker stopping")
			return
			
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops a crawl worker
func (cw *CrawlWorker) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()
	
	if !cw.stopped {
		close(cw.stopCh)
		cw.stopped = true
	}
}

// processJob processes a single crawl job
func (cw *CrawlWorker) processJob(ctx context.Context, job *CrawlJob) {
	start := time.Now()
	
	log.Debug().
		Int("worker_id", cw.ID).
		Str("job_id", job.ID).
		Str("url", job.URL).
		Msg("Processing crawl job")
	
	// Update job status
	job.Status = procurement.StatusProcessing
	job.StartedAt = time.Now()
	job.AttemptCount++
	
	// Create job context with timeout
	jobCtx, cancel := context.WithTimeout(ctx, cw.crawler.config.JobTimeout)
	defer cancel()
	
	// Process the job
	result := cw.crawlURL(jobCtx, job)
	
	// Update job status
	job.CompletedAt = time.Now()
	if result.Success {
		job.Status = procurement.StatusCompleted
	} else {
		job.Status = procurement.StatusFailed
		job.LastError = result.Error
	}
	
	// Send result
	select {
	case cw.crawler.resultQueue <- result:
		// Sent successfully
	case <-ctx.Done():
		return
	default:
		log.Warn().Str("job_id", job.ID).Msg("Result queue full, dropping result")
	}
	
	// Update worker stats
	cw.crawler.metricsMu.Lock()
	if stats, exists := cw.crawler.metrics.WorkerStats[cw.ID]; exists {
		stats.JobsProcessed++
		totalTime := time.Duration(int64(stats.AverageTime)*stats.JobsProcessed-1) + time.Since(start)
		stats.AverageTime = totalTime / time.Duration(stats.JobsProcessed)
		stats.LastJobTime = time.Now()
	}
	cw.crawler.metricsMu.Unlock()
	
	log.Debug().
		Int("worker_id", cw.ID).
		Str("job_id", job.ID).
		Bool("success", result.Success).
		Dur("processing_time", time.Since(start)).
		Msg("Crawl job completed")
}

// crawlURL crawls a single URL
func (cw *CrawlWorker) crawlURL(ctx context.Context, job *CrawlJob) *CrawlResult {
	result := &CrawlResult{
		JobID:     job.ID,
		URL:       job.URL,
		CreatedAt: time.Now(),
	}
	
	// Check compliance
	compliance, err := cw.crawler.complianceEngine.CheckCompliance(ctx, job.URL)
	if err != nil {
		result.Error = fmt.Sprintf("Compliance check failed: %v", err)
		return result
	}
	
	result.ComplianceResult = compliance
	
	if !compliance.Allowed {
		result.Error = fmt.Sprintf("URL not allowed: %s", strings.Join(compliance.Restrictions, ", "))
		return result
	}
	
	// Wait for rate limiting
	if err := cw.crawler.rateLimiter.Wait(ctx, job.Domain, compliance.RequiredDelay); err != nil {
		result.Error = fmt.Sprintf("Rate limiting failed: %v", err)
		return result
	}
	
	// Extract content
	extractionResult, err := cw.crawler.extractor.ExtractContent(ctx, job.URL)
	if err != nil {
		result.Error = fmt.Sprintf("Content extraction failed: %v", err)
		// Record rate limiting result
		cw.crawler.rateLimiter.RecordRequest(job.Domain, RequestResult{
			Timestamp:   time.Now(),
			StatusCode:  0,
			Success:     false,
			RateLimited: false,
		})
		return result
	}
	
	result.StatusCode = extractionResult.StatusCode
	result.ProcessingTime = extractionResult.ProcessingTime
	
	if !extractionResult.Success {
		result.Error = extractionResult.Error
		// Record result
		cw.crawler.rateLimiter.RecordRequest(job.Domain, RequestResult{
			Timestamp:   time.Now(),
			StatusCode:  extractionResult.StatusCode,
			Duration:    extractionResult.ProcessingTime,
			Success:     false,
			RateLimited: extractionResult.StatusCode == 429,
		})
		return result
	}
	
	// Validate quality
	if cw.crawler.qualityValidator != nil {
		validation, err := cw.crawler.qualityValidator.ValidateContent(
			ctx, 
			extractionResult.Document.Content.Text, 
			extractionResult.Document.Content.Metadata,
		)
		if err != nil {
			log.Warn().Err(err).Str("url", job.URL).Msg("Quality validation failed")
			result.QualityScore = 0.5 // Default score
		} else {
			result.QualityScore = validation.OverallScore
		}
		
		// Check quality threshold
		if result.QualityScore < cw.crawler.config.QualityThreshold {
			result.Error = fmt.Sprintf("Quality score %.2f below threshold %.2f", 
				result.QualityScore, cw.crawler.config.QualityThreshold)
			result.Document = extractionResult.Document // Still include document
		} else {
			result.Document = extractionResult.Document
			result.Success = true
		}
	} else {
		result.Document = extractionResult.Document
		result.QualityScore = 0.8 // Default score when no validator
		result.Success = true
	}
	
	// Record successful request
	cw.crawler.rateLimiter.RecordRequest(job.Domain, RequestResult{
		Timestamp:   time.Now(),
		StatusCode:  extractionResult.StatusCode,
		Duration:    extractionResult.ProcessingTime,
		Success:     result.Success,
		RateLimited: false,
	})
	
	return result
}

// processResults processes crawl results
func (dc *DistributedCrawler) processResults(ctx context.Context) {
	log.Debug().Msg("Starting result processor")
	
	for {
		select {
		case result := <-dc.resultQueue:
			if result == nil {
				return // Channel closed
			}
			dc.handleResult(ctx, result)
			
		case <-ctx.Done():
			return
		}
	}
}

// handleResult handles a single crawl result
func (dc *DistributedCrawler) handleResult(ctx context.Context, result *CrawlResult) {
	log.Debug().
		Str("job_id", result.JobID).
		Str("url", result.URL).
		Bool("success", result.Success).
		Float64("quality_score", result.QualityScore).
		Msg("Processing crawl result")
	
	// Update metrics
	dc.metricsMu.Lock()
	if result.Success {
		dc.metrics.JobsCompleted++
		if result.Document != nil {
			dc.metrics.DocumentsStored++
			dc.metrics.BytesProcessed += int64(len(result.Document.Content.Text))
			
			// Update average quality
			totalQuality := dc.metrics.AverageQuality * float64(dc.metrics.DocumentsStored-1)
			dc.metrics.AverageQuality = (totalQuality + result.QualityScore) / float64(dc.metrics.DocumentsStored)
		}
	} else {
		dc.metrics.JobsFailed++
	}
	
	// Update domain stats
	if result.ComplianceResult != nil {
		domain := result.ComplianceResult.Domain
		if stats, exists := dc.metrics.DomainStats[domain]; exists {
			stats.PagesProcessed++
			if result.Success {
				stats.PagesSuccessful++
				totalQuality := stats.AverageQuality * float64(stats.PagesSuccessful-1)
				stats.AverageQuality = (totalQuality + result.QualityScore) / float64(stats.PagesSuccessful)
			}
			stats.LastCrawled = time.Now()
		} else {
			dc.metrics.DomainStats[domain] = &DomainStats{
				Domain:          domain,
				PagesProcessed:  1,
				PagesSuccessful: func() int64 { if result.Success { return 1 } else { return 0 } }(),
				AverageQuality:  result.QualityScore,
				LastCrawled:     time.Now(),
			}
		}
	}
	
	dc.metrics.LastUpdated = time.Now()
	dc.metricsMu.Unlock()
	
	// Store document if successful
	if result.Success && result.Document != nil {
		if _, err := dc.storage.StoreDocument(ctx, result.Document); err != nil {
			log.Error().
				Err(err).
				Str("job_id", result.JobID).
				Str("url", result.URL).
				Msg("Failed to store document")
		} else {
			log.Debug().
				Str("job_id", result.JobID).
				Str("document_id", result.Document.ID).
				Msg("Document stored successfully")
		}
	}
	
	// Remove job from active jobs
	dc.jobsMu.Lock()
	delete(dc.activeJobs, result.JobID)
	dc.jobsMu.Unlock()
}

// GetMetrics returns current crawling metrics
func (dc *DistributedCrawler) GetMetrics() *CrawlMetrics {
	dc.metricsMu.RLock()
	defer dc.metricsMu.RUnlock()
	
	// Return a deep copy to prevent race conditions
	metrics := *dc.metrics
	
	// Copy domain stats
	metrics.DomainStats = make(map[string]*DomainStats)
	for domain, stats := range dc.metrics.DomainStats {
		statsCopy := *stats
		metrics.DomainStats[domain] = &statsCopy
	}
	
	// Copy worker stats
	metrics.WorkerStats = make(map[int]*WorkerStats)
	for id, stats := range dc.metrics.WorkerStats {
		statsCopy := *stats
		metrics.WorkerStats[id] = &statsCopy
	}
	
	return &metrics
}

// GetActiveJobs returns currently active jobs
func (dc *DistributedCrawler) GetActiveJobs() []*CrawlJob {
	dc.jobsMu.RLock()
	defer dc.jobsMu.RUnlock()
	
	jobs := make([]*CrawlJob, 0, len(dc.activeJobs))
	for _, job := range dc.activeJobs {
		jobCopy := *job
		jobs = append(jobs, &jobCopy)
	}
	
	return jobs
}

// GetQueueStatus returns the current queue status
func (dc *DistributedCrawler) GetQueueStatus() map[string]int {
	return map[string]int{
		"job_queue_length":    len(dc.jobQueue),
		"result_queue_length": len(dc.resultQueue),
		"active_jobs":         len(dc.activeJobs),
		"active_workers":      len(dc.workers),
	}
}