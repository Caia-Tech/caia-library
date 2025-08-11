package scraping

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/rs/zerolog/log"
)

// ScrapingService provides high-level web scraping functionality
type ScrapingService struct {
	crawler          *DistributedCrawler
	complianceEngine *ComplianceEngine
	rateLimiter     *AdaptiveRateLimiter
	extractor       *ContentExtractor
	qualityValidator procurement.QualityValidator
	storage         storage.StorageBackend
	config          *ServiceConfig
	
	// Source management
	sources       map[string]*ScrapingSource
	sourcesMu     sync.RWMutex
	
	// Metrics
	serviceMetrics *ServiceMetrics
	metricsMu     sync.RWMutex
	
	// State
	running  bool
	stopCh   chan struct{}
	mu       sync.Mutex
}

// ServiceConfig configures the scraping service
type ServiceConfig struct {
	MaxConcurrentSources int           `json:"max_concurrent_sources"`
	DefaultCrawlInterval time.Duration `json:"default_crawl_interval"`
	MaxRetryAttempts     int           `json:"max_retry_attempts"`
	HealthCheckInterval  time.Duration `json:"health_check_interval"`
	MetricsUpdateInterval time.Duration `json:"metrics_update_interval"`
	EnableAutoDiscovery   bool          `json:"enable_auto_discovery"`
	QualityThreshold      float64       `json:"quality_threshold"`
	MaxDocumentsPerSource int           `json:"max_documents_per_source"`
	ArchiveResults        bool          `json:"archive_results"`
}

// ScrapingSource represents a configured web scraping source
type ScrapingSource struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	BaseURL         string            `json:"base_url"`
	Domain          string            `json:"domain"`
	SourceType      string            `json:"source_type"`
	CrawlInterval   time.Duration     `json:"crawl_interval"`
	MaxPages        int               `json:"max_pages"`
	StartURLs       []string          `json:"start_urls"`
	URLPatterns     []string          `json:"url_patterns"`
	ExcludePatterns []string          `json:"exclude_patterns"`
	Metadata        map[string]string `json:"metadata"`
	Config          *SourceConfig     `json:"config"`
	Status          SourceStatus      `json:"status"`
	LastCrawl       time.Time         `json:"last_crawl"`
	NextCrawl       time.Time         `json:"next_crawl"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// SourceConfig provides source-specific configuration
type SourceConfig struct {
	UserAgent       string            `json:"user_agent"`
	CrawlDelay      time.Duration     `json:"crawl_delay"`
	RespectRobots   bool              `json:"respect_robots"`
	MaxDepth        int               `json:"max_depth"`
	MaxConcurrency  int               `json:"max_concurrency"`
	ContentSelectors map[string]string `json:"content_selectors"`
	MetadataRules   map[string]string `json:"metadata_rules"`
	FilterRules     []FilterRule      `json:"filter_rules"`
}

// FilterRule defines content filtering rules
type FilterRule struct {
	Type      string `json:"type"`      // "include", "exclude", "transform"
	Field     string `json:"field"`     // "url", "content", "title", etc.
	Pattern   string `json:"pattern"`   // regex pattern
	Action    string `json:"action"`    // "skip", "modify", "tag"
	Value     string `json:"value,omitempty"`
}

// SourceStatus represents the status of a scraping source
type SourceStatus string

const (
	SourceStatusActive   SourceStatus = "active"
	SourceStatusPaused   SourceStatus = "paused"
	SourceStatusError    SourceStatus = "error"
	SourceStatusDisabled SourceStatus = "disabled"
)

// ServiceMetrics tracks overall service performance
type ServiceMetrics struct {
	ActiveSources      int                     `json:"active_sources"`
	TotalDocuments     int64                   `json:"total_documents"`
	DocumentsToday     int64                   `json:"documents_today"`
	AverageQuality     float64                 `json:"average_quality"`
	SuccessRate        float64                 `json:"success_rate"`
	SourceMetrics      map[string]*SourceMetrics `json:"source_metrics"`
	QualityDistribution map[string]int         `json:"quality_distribution"`
	ErrorsByType       map[string]int64        `json:"errors_by_type"`
	LastUpdated        time.Time               `json:"last_updated"`
}

// SourceMetrics tracks per-source performance
type SourceMetrics struct {
	SourceID         string    `json:"source_id"`
	DocumentsScraped int64     `json:"documents_scraped"`
	SuccessfulScrapes int64    `json:"successful_scrapes"`
	FailedScrapes    int64     `json:"failed_scrapes"`
	AverageQuality   float64   `json:"average_quality"`
	LastSuccess      time.Time `json:"last_success"`
	LastError        string    `json:"last_error,omitempty"`
	ErrorCount       int64     `json:"error_count"`
}

// NewScrapingService creates a new scraping service
func NewScrapingService(
	complianceEngine *ComplianceEngine,
	rateLimiter *AdaptiveRateLimiter,
	extractor *ContentExtractor,
	qualityValidator procurement.QualityValidator,
	storage storage.StorageBackend,
	config *ServiceConfig,
) *ScrapingService {
	if config == nil {
		config = DefaultServiceConfig()
	}
	
	// Create distributed crawler
	crawlerConfig := DefaultCrawlerConfig()
	crawlerConfig.QualityThreshold = config.QualityThreshold
	crawler := NewDistributedCrawler(
		complianceEngine,
		rateLimiter,
		extractor,
		qualityValidator,
		storage,
		crawlerConfig,
	)
	
	return &ScrapingService{
		crawler:          crawler,
		complianceEngine: complianceEngine,
		rateLimiter:     rateLimiter,
		extractor:       extractor,
		qualityValidator: qualityValidator,
		storage:         storage,
		config:          config,
		sources:         make(map[string]*ScrapingSource),
		stopCh:          make(chan struct{}),
		serviceMetrics: &ServiceMetrics{
			SourceMetrics:       make(map[string]*SourceMetrics),
			QualityDistribution: make(map[string]int),
			ErrorsByType:       make(map[string]int64),
			LastUpdated:        time.Now(),
		},
	}
}

// DefaultServiceConfig returns default service configuration
func DefaultServiceConfig() *ServiceConfig {
	return &ServiceConfig{
		MaxConcurrentSources:  10,
		DefaultCrawlInterval:  24 * time.Hour,
		MaxRetryAttempts:      3,
		HealthCheckInterval:   5 * time.Minute,
		MetricsUpdateInterval: 1 * time.Minute,
		EnableAutoDiscovery:   false,
		QualityThreshold:      0.6,
		MaxDocumentsPerSource: 10000,
		ArchiveResults:        true,
	}
}

// Start starts the scraping service
func (ss *ScrapingService) Start(ctx context.Context) error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	
	if ss.running {
		return fmt.Errorf("scraping service already running")
	}
	
	log.Info().Msg("Starting scraping service")
	
	// Start crawler
	if err := ss.crawler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start crawler: %w", err)
	}
	
	// Start service routines
	go ss.schedulerLoop(ctx)
	go ss.healthCheckLoop(ctx)
	go ss.metricsUpdateLoop(ctx)
	
	ss.running = true
	
	log.Info().
		Int("max_concurrent_sources", ss.config.MaxConcurrentSources).
		Dur("default_crawl_interval", ss.config.DefaultCrawlInterval).
		Msg("Scraping service started")
	
	return nil
}

// Stop stops the scraping service
func (ss *ScrapingService) Stop() error {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	
	if !ss.running {
		return nil
	}
	
	log.Info().Msg("Stopping scraping service")
	
	// Signal stop
	close(ss.stopCh)
	
	// Stop crawler
	if err := ss.crawler.Stop(); err != nil {
		log.Error().Err(err).Msg("Error stopping crawler")
	}
	
	ss.running = false
	
	log.Info().Msg("Scraping service stopped")
	return nil
}

// AddSource adds a new scraping source
func (ss *ScrapingService) AddSource(source *ScrapingSource) error {
	ss.sourcesMu.Lock()
	defer ss.sourcesMu.Unlock()
	
	// Validate source
	if err := ss.validateSource(source); err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}
	
	// Set defaults
	if source.CrawlInterval == 0 {
		source.CrawlInterval = ss.config.DefaultCrawlInterval
	}
	
	if source.Config == nil {
		source.Config = DefaultSourceConfig()
	}
	
	source.Status = SourceStatusActive
	source.CreatedAt = time.Now()
	source.UpdatedAt = time.Now()
	source.NextCrawl = time.Now().Add(source.CrawlInterval)
	
	// Parse domain from base URL
	if parsedURL, err := url.Parse(source.BaseURL); err == nil {
		source.Domain = parsedURL.Host
	}
	
	ss.sources[source.ID] = source
	
	// Initialize source metrics
	ss.metricsMu.Lock()
	ss.serviceMetrics.SourceMetrics[source.ID] = &SourceMetrics{
		SourceID: source.ID,
	}
	ss.serviceMetrics.ActiveSources++
	ss.metricsMu.Unlock()
	
	log.Info().
		Str("source_id", source.ID).
		Str("source_name", source.Name).
		Str("base_url", source.BaseURL).
		Msg("Added new scraping source")
	
	return nil
}

// DefaultSourceConfig returns default source configuration
func DefaultSourceConfig() *SourceConfig {
	return &SourceConfig{
		UserAgent:      "CAIA-Library/1.0 (+https://caia.tech/bot)",
		CrawlDelay:     1 * time.Second,
		RespectRobots:  true,
		MaxDepth:       3,
		MaxConcurrency: 3,
		ContentSelectors: map[string]string{
			"title":   "h1, .title, .entry-title",
			"content": ".content, .post-content, .entry-content, main, article",
			"author":  ".author, .byline, .post-author",
			"date":    ".date, .published, .post-date, time",
		},
		MetadataRules: map[string]string{
			"category": "meta[name='category']",
			"tags":     "meta[name='keywords']",
		},
		FilterRules: []FilterRule{
			{
				Type:    "exclude",
				Field:   "url",
				Pattern: `\.(jpg|jpeg|png|gif|pdf|doc|docx)$`,
				Action:  "skip",
			},
		},
	}
}

// RemoveSource removes a scraping source
func (ss *ScrapingService) RemoveSource(sourceID string) error {
	ss.sourcesMu.Lock()
	defer ss.sourcesMu.Unlock()
	
	if _, exists := ss.sources[sourceID]; !exists {
		return fmt.Errorf("source not found: %s", sourceID)
	}
	
	delete(ss.sources, sourceID)
	
	// Update metrics
	ss.metricsMu.Lock()
	delete(ss.serviceMetrics.SourceMetrics, sourceID)
	ss.serviceMetrics.ActiveSources--
	ss.metricsMu.Unlock()
	
	log.Info().Str("source_id", sourceID).Msg("Removed scraping source")
	return nil
}

// GetSource retrieves a scraping source
func (ss *ScrapingService) GetSource(sourceID string) *ScrapingSource {
	ss.sourcesMu.RLock()
	defer ss.sourcesMu.RUnlock()
	
	if source, exists := ss.sources[sourceID]; exists {
		// Return a copy
		sourceCopy := *source
		return &sourceCopy
	}
	
	return nil
}

// ListSources returns all configured sources
func (ss *ScrapingService) ListSources() []*ScrapingSource {
	ss.sourcesMu.RLock()
	defer ss.sourcesMu.RUnlock()
	
	sources := make([]*ScrapingSource, 0, len(ss.sources))
	for _, source := range ss.sources {
		sourceCopy := *source
		sources = append(sources, &sourceCopy)
	}
	
	return sources
}

// ScrapeSource manually triggers scraping for a specific source
func (ss *ScrapingService) ScrapeSource(ctx context.Context, sourceID string) error {
	source := ss.GetSource(sourceID)
	if source == nil {
		return fmt.Errorf("source not found: %s", sourceID)
	}
	
	if source.Status != SourceStatusActive {
		return fmt.Errorf("source is not active: %s", source.Status)
	}
	
	return ss.crawlSource(ctx, source)
}

// schedulerLoop runs the main scheduling loop
func (ss *ScrapingService) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute) // Check every minute
	defer ticker.Stop()
	
	log.Debug().Msg("Starting scheduler loop")
	
	for {
		select {
		case <-ticker.C:
			ss.scheduleReadySources(ctx)
		case <-ss.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// scheduleReadySources schedules sources that are ready to be crawled
func (ss *ScrapingService) scheduleReadySources(ctx context.Context) {
	now := time.Now()
	
	ss.sourcesMu.RLock()
	readySources := make([]*ScrapingSource, 0)
	
	for _, source := range ss.sources {
		if source.Status == SourceStatusActive && now.After(source.NextCrawl) {
			readySources = append(readySources, source)
		}
	}
	ss.sourcesMu.RUnlock()
	
	if len(readySources) == 0 {
		return
	}
	
	log.Debug().Int("ready_sources", len(readySources)).Msg("Scheduling ready sources")
	
	// Limit concurrent source crawling
	semaphore := make(chan struct{}, ss.config.MaxConcurrentSources)
	
	for _, source := range readySources {
		semaphore <- struct{}{}
		
		go func(src *ScrapingSource) {
			defer func() { <-semaphore }()
			
			if err := ss.crawlSource(ctx, src); err != nil {
				log.Error().
					Err(err).
					Str("source_id", src.ID).
					Msg("Failed to crawl source")
				
				// Update source status
				ss.updateSourceError(src.ID, err.Error())
			} else {
				// Update last crawl time and schedule next
				ss.updateSourceSuccess(src.ID)
			}
		}(source)
	}
}

// crawlSource performs crawling for a specific source
func (ss *ScrapingService) crawlSource(ctx context.Context, source *ScrapingSource) error {
	log.Info().
		Str("source_id", source.ID).
		Str("source_name", source.Name).
		Int("start_urls", len(source.StartURLs)).
		Msg("Starting source crawl")
	
	// Generate crawl jobs
	jobs := ss.generateCrawlJobs(source)
	
	log.Debug().
		Str("source_id", source.ID).
		Int("jobs_generated", len(jobs)).
		Msg("Generated crawl jobs")
	
	// Submit jobs to crawler
	if err := ss.crawler.SubmitBatch(ctx, jobs); err != nil {
		return fmt.Errorf("failed to submit crawl jobs: %w", err)
	}
	
	return nil
}

// generateCrawlJobs generates crawl jobs for a source
func (ss *ScrapingService) generateCrawlJobs(source *ScrapingSource) []*CrawlJob {
	jobs := make([]*CrawlJob, 0)
	
	for i, startURL := range source.StartURLs {
		job := &CrawlJob{
			ID:       fmt.Sprintf("%s-%d-%d", source.ID, time.Now().Unix(), i),
			URL:      startURL,
			Domain:   source.Domain,
			Depth:    0,
			Priority: 1,
			Source:   source.ID,
			Metadata: map[string]string{
				"source_id":   source.ID,
				"source_name": source.Name,
				"source_type": source.SourceType,
			},
		}
		
		// Add source-specific metadata
		for k, v := range source.Metadata {
			job.Metadata[k] = v
		}
		
		jobs = append(jobs, job)
	}
	
	return jobs
}

// healthCheckLoop performs periodic health checks
func (ss *ScrapingService) healthCheckLoop(ctx context.Context) {
	ticker := time.NewTicker(ss.config.HealthCheckInterval)
	defer ticker.Stop()
	
	log.Debug().Msg("Starting health check loop")
	
	for {
		select {
		case <-ticker.C:
			ss.performHealthCheck()
		case <-ss.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// performHealthCheck checks the health of sources and the service
func (ss *ScrapingService) performHealthCheck() {
	log.Debug().Msg("Performing health check")
	
	// Check crawler health
	queueStatus := ss.crawler.GetQueueStatus()
	
	if queueStatus["job_queue_length"] > 500 {
		log.Warn().Int("queue_length", queueStatus["job_queue_length"]).Msg("Job queue is getting full")
	}
	
	// Check source health
	ss.sourcesMu.RLock()
	errorSources := 0
	for _, source := range ss.sources {
		if source.Status == SourceStatusError {
			errorSources++
		}
	}
	ss.sourcesMu.RUnlock()
	
	if errorSources > 0 {
		log.Warn().Int("error_sources", errorSources).Msg("Sources in error state detected")
	}
	
	// Cleanup inactive domain limiters
	ss.rateLimiter.CleanupInactive(24 * time.Hour)
	
	// Clear expired compliance cache
	ss.complianceEngine.ClearExpiredCache()
}

// metricsUpdateLoop updates service metrics
func (ss *ScrapingService) metricsUpdateLoop(ctx context.Context) {
	ticker := time.NewTicker(ss.config.MetricsUpdateInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			ss.updateMetrics()
		case <-ss.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// updateMetrics updates service metrics
func (ss *ScrapingService) updateMetrics() {
	crawlMetrics := ss.crawler.GetMetrics()
	
	ss.metricsMu.Lock()
	defer ss.metricsMu.Unlock()
	
	ss.serviceMetrics.TotalDocuments = crawlMetrics.DocumentsStored
	ss.serviceMetrics.AverageQuality = crawlMetrics.AverageQuality
	
	if crawlMetrics.JobsCompleted > 0 {
		ss.serviceMetrics.SuccessRate = float64(crawlMetrics.JobsCompleted) / float64(crawlMetrics.JobsQueued)
	}
	
	ss.serviceMetrics.LastUpdated = time.Now()
}

// Helper methods

func (ss *ScrapingService) validateSource(source *ScrapingSource) error {
	if source.ID == "" {
		return fmt.Errorf("source ID is required")
	}
	
	if source.Name == "" {
		return fmt.Errorf("source name is required")
	}
	
	if source.BaseURL == "" {
		return fmt.Errorf("base URL is required")
	}
	
	if _, err := url.Parse(source.BaseURL); err != nil {
		return fmt.Errorf("invalid base URL: %w", err)
	}
	
	if len(source.StartURLs) == 0 {
		return fmt.Errorf("at least one start URL is required")
	}
	
	return nil
}

func (ss *ScrapingService) updateSourceError(sourceID, errorMsg string) {
	ss.sourcesMu.Lock()
	if source, exists := ss.sources[sourceID]; exists {
		source.Status = SourceStatusError
		source.UpdatedAt = time.Now()
		source.NextCrawl = time.Now().Add(source.CrawlInterval * 2) // Backoff
	}
	ss.sourcesMu.Unlock()
	
	ss.metricsMu.Lock()
	if metrics, exists := ss.serviceMetrics.SourceMetrics[sourceID]; exists {
		metrics.ErrorCount++
		metrics.LastError = errorMsg
	}
	ss.metricsMu.Unlock()
}

func (ss *ScrapingService) updateSourceSuccess(sourceID string) {
	ss.sourcesMu.Lock()
	if source, exists := ss.sources[sourceID]; exists {
		source.Status = SourceStatusActive
		source.LastCrawl = time.Now()
		source.NextCrawl = time.Now().Add(source.CrawlInterval)
		source.UpdatedAt = time.Now()
	}
	ss.sourcesMu.Unlock()
	
	ss.metricsMu.Lock()
	if metrics, exists := ss.serviceMetrics.SourceMetrics[sourceID]; exists {
		metrics.LastSuccess = time.Now()
	}
	ss.metricsMu.Unlock()
}

// GetMetrics returns current service metrics
func (ss *ScrapingService) GetMetrics() *ServiceMetrics {
	ss.metricsMu.RLock()
	defer ss.metricsMu.RUnlock()
	
	// Return a copy
	metrics := *ss.serviceMetrics
	
	// Deep copy source metrics
	metrics.SourceMetrics = make(map[string]*SourceMetrics)
	for id, srcMetrics := range ss.serviceMetrics.SourceMetrics {
		metricsCopy := *srcMetrics
		metrics.SourceMetrics[id] = &metricsCopy
	}
	
	return &metrics
}

// GetCrawlerStatus returns crawler status information
func (ss *ScrapingService) GetCrawlerStatus() map[string]interface{} {
	return map[string]interface{}{
		"queue_status": ss.crawler.GetQueueStatus(),
		"active_jobs":  len(ss.crawler.GetActiveJobs()),
		"metrics":      ss.crawler.GetMetrics(),
	}
}