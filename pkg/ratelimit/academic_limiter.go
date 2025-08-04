package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AcademicRateLimiter ensures ethical rate limiting for academic sources
type AcademicRateLimiter struct {
	mu       sync.Mutex
	limiters map[string]*SourceLimiter
}

// SourceLimiter tracks rate limits for a specific source
type SourceLimiter struct {
	name            string
	requestsPerSec  float64
	lastRequestTime time.Time
	minInterval     time.Duration
	backoffUntil    time.Time
	requestCount    int64
	errorCount      int64
}

// NewAcademicRateLimiter creates a rate limiter for academic sources
func NewAcademicRateLimiter() *AcademicRateLimiter {
	return &AcademicRateLimiter{
		limiters: map[string]*SourceLimiter{
			"arxiv": {
				name:           "arxiv",
				requestsPerSec: 0.33, // 1 per 3 seconds
				minInterval:    3 * time.Second,
			},
			"pubmed": {
				name:           "pubmed",
				requestsPerSec: 3,
				minInterval:    350 * time.Millisecond,
			},
			"doaj": {
				name:           "doaj",
				requestsPerSec: 1,
				minInterval:    1 * time.Second,
			},
			"plos": {
				name:           "plos",
				requestsPerSec: 1,
				minInterval:    1 * time.Second,
			},
			"semantic_scholar": {
				name:           "semantic_scholar",
				requestsPerSec: 0.33, // ~100 per 5 minutes
				minInterval:    3 * time.Second,
			},
		},
	}
}

// WaitForSource blocks until it's safe to make a request to the source
func (r *AcademicRateLimiter) WaitForSource(ctx context.Context, source string) error {
	r.mu.Lock()
	limiter, exists := r.limiters[source]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("unknown source: %s", source)
	}

	now := time.Now()

	// Check if we're in backoff
	if now.Before(limiter.backoffUntil) {
		waitTime := limiter.backoffUntil.Sub(now)
		r.mu.Unlock()
		
		select {
		case <-time.After(waitTime):
			return r.WaitForSource(ctx, source) // Retry after backoff
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Calculate time since last request
	timeSinceLastRequest := now.Sub(limiter.lastRequestTime)
	
	// If not enough time has passed, wait
	if timeSinceLastRequest < limiter.minInterval {
		waitTime := limiter.minInterval - timeSinceLastRequest
		r.mu.Unlock()
		
		select {
		case <-time.After(waitTime):
			// Update last request time after waiting
			r.mu.Lock()
			limiter.lastRequestTime = time.Now()
			limiter.requestCount++
			r.mu.Unlock()
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Safe to proceed
	limiter.lastRequestTime = now
	limiter.requestCount++
	r.mu.Unlock()
	return nil
}

// RecordError records an error and potentially triggers backoff
func (r *AcademicRateLimiter) RecordError(source string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limiter, exists := r.limiters[source]
	if !exists {
		return
	}

	limiter.errorCount++

	// Implement exponential backoff on repeated errors
	if limiter.errorCount > 3 {
		backoffDuration := time.Duration(limiter.errorCount) * 30 * time.Second
		if backoffDuration > 5*time.Minute {
			backoffDuration = 5 * time.Minute
		}
		limiter.backoffUntil = time.Now().Add(backoffDuration)
	}
}

// RecordSuccess resets error count for a source
func (r *AcademicRateLimiter) RecordSuccess(source string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	limiter, exists := r.limiters[source]
	if exists {
		limiter.errorCount = 0
	}
}

// GetStats returns statistics for all sources
func (r *AcademicRateLimiter) GetStats() map[string]SourceStats {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := make(map[string]SourceStats)
	for name, limiter := range r.limiters {
		stats[name] = SourceStats{
			RequestCount:    limiter.requestCount,
			ErrorCount:      limiter.errorCount,
			LastRequestTime: limiter.lastRequestTime,
			InBackoff:       time.Now().Before(limiter.backoffUntil),
			BackoffUntil:    limiter.backoffUntil,
		}
	}
	return stats
}

// SourceStats contains statistics for a source
type SourceStats struct {
	RequestCount    int64
	ErrorCount      int64
	LastRequestTime time.Time
	InBackoff       bool
	BackoffUntil    time.Time
}

// EthicalCollectorConfig contains configuration for ethical collection
type EthicalCollectorConfig struct {
	UserAgent           string
	CacheDir            string
	MaxRetries          int
	AttributionTemplate string
	ContactEmail        string
}

// DefaultEthicalConfig returns default ethical collection configuration
func DefaultEthicalConfig() EthicalCollectorConfig {
	return EthicalCollectorConfig{
		UserAgent:           "CAIA-Library/1.0 (https://github.com/Caia-Tech/caia-library; library@caiatech.com) Academic-Research-Bot",
		CacheDir:            "/tmp/caia-cache",
		MaxRetries:          3,
		AttributionTemplate: "Content from %s, collected by Caia Tech (https://caiatech.com) on %s",
		ContactEmail:        "library@caiatech.com",
	}
}