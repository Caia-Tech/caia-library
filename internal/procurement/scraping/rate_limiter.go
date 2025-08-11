package scraping

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AdaptiveRateLimiter manages request rate limiting with adaptive behavior
type AdaptiveRateLimiter struct {
	domainLimiters map[string]*DomainLimiter
	mu             sync.RWMutex
	config         *RateLimiterConfig
	globalLimiter  *TokenBucket
}

// RateLimiterConfig configures rate limiting behavior
type RateLimiterConfig struct {
	DefaultDelay          time.Duration `json:"default_delay"`
	MaxConcurrentDomains  int           `json:"max_concurrent_domains"`
	MaxConcurrentPerDomain int          `json:"max_concurrent_per_domain"`
	BackoffMultiplier     float64       `json:"backoff_multiplier"`
	MaxBackoffDelay       time.Duration `json:"max_backoff_delay"`
	AdaptiveAdjustment    bool          `json:"adaptive_adjustment"`
	RespectRetryAfter     bool          `json:"respect_retry_after"`
	MinDelay              time.Duration `json:"min_delay"`
	MaxDelay              time.Duration `json:"max_delay"`
}

// DomainLimiter manages rate limiting for a specific domain
type DomainLimiter struct {
	Domain           string        `json:"domain"`
	CurrentDelay     time.Duration `json:"current_delay"`
	LastRequest      time.Time     `json:"last_request"`
	RequestCount     int64         `json:"request_count"`
	ErrorCount       int64         `json:"error_count"`
	SuccessCount     int64         `json:"success_count"`
	TokenBucket      *TokenBucket  `json:"token_bucket"`
	Semaphore        chan struct{} `json:"-"`
	AdaptiveHistory  []RequestResult `json:"adaptive_history"`
	mu               sync.Mutex    `json:"-"`
}

// RequestResult represents the result of a request for adaptive learning
type RequestResult struct {
	Timestamp   time.Time     `json:"timestamp"`
	StatusCode  int           `json:"status_code"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	RateLimited bool          `json:"rate_limited"`
}

// TokenBucket implements a token bucket rate limiter
type TokenBucket struct {
	capacity    int64
	tokens      int64
	refillRate  time.Duration
	lastRefill  time.Time
	mu          sync.Mutex
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(config *RateLimiterConfig) *AdaptiveRateLimiter {
	if config == nil {
		config = DefaultRateLimiterConfig()
	}
	
	return &AdaptiveRateLimiter{
		domainLimiters: make(map[string]*DomainLimiter),
		config:         config,
		globalLimiter:  NewTokenBucket(int64(config.MaxConcurrentDomains), time.Second/10), // 10 domains per second max
	}
}

// DefaultRateLimiterConfig returns default rate limiter configuration
func DefaultRateLimiterConfig() *RateLimiterConfig {
	return &RateLimiterConfig{
		DefaultDelay:           1 * time.Second,
		MaxConcurrentDomains:   50,
		MaxConcurrentPerDomain: 3,
		BackoffMultiplier:      2.0,
		MaxBackoffDelay:        30 * time.Second,
		AdaptiveAdjustment:     true,
		RespectRetryAfter:      true,
		MinDelay:               100 * time.Millisecond,
		MaxDelay:               60 * time.Second,
	}
}

// NewTokenBucket creates a new token bucket
func NewTokenBucket(capacity int64, refillRate time.Duration) *TokenBucket {
	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait blocks until a request can be made to the specified domain
func (arl *AdaptiveRateLimiter) Wait(ctx context.Context, domain string, requiredDelay time.Duration) error {
	// Check global rate limiter first
	if !arl.globalLimiter.TryAcquire() {
		select {
		case <-time.After(arl.globalLimiter.refillRate):
			// Try again after refill interval
			if !arl.globalLimiter.TryAcquire() {
				return fmt.Errorf("global rate limit exceeded")
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	// Get or create domain limiter
	limiter := arl.getDomainLimiter(domain)
	
	// Apply domain-specific rate limiting
	return limiter.Wait(ctx, requiredDelay, arl.config)
}

// RecordRequest records the result of a request for adaptive learning
func (arl *AdaptiveRateLimiter) RecordRequest(domain string, result RequestResult) {
	limiter := arl.getDomainLimiter(domain)
	limiter.RecordRequest(result, arl.config)
}

// GetDomainStats returns statistics for a domain
func (arl *AdaptiveRateLimiter) GetDomainStats(domain string) *DomainLimiter {
	arl.mu.RLock()
	defer arl.mu.RUnlock()
	
	if limiter, exists := arl.domainLimiters[domain]; exists {
		limiter.mu.Lock()
		defer limiter.mu.Unlock()
		
		// Return a copy to prevent race conditions
		return &DomainLimiter{
			Domain:          limiter.Domain,
			CurrentDelay:    limiter.CurrentDelay,
			LastRequest:     limiter.LastRequest,
			RequestCount:    limiter.RequestCount,
			ErrorCount:      limiter.ErrorCount,
			SuccessCount:    limiter.SuccessCount,
			AdaptiveHistory: append([]RequestResult{}, limiter.AdaptiveHistory...),
		}
	}
	
	return nil
}

// getDomainLimiter gets or creates a domain limiter
func (arl *AdaptiveRateLimiter) getDomainLimiter(domain string) *DomainLimiter {
	arl.mu.Lock()
	defer arl.mu.Unlock()
	
	if limiter, exists := arl.domainLimiters[domain]; exists {
		return limiter
	}
	
	// Create new domain limiter
	limiter := &DomainLimiter{
		Domain:          domain,
		CurrentDelay:    arl.config.DefaultDelay,
		TokenBucket:     NewTokenBucket(int64(arl.config.MaxConcurrentPerDomain), arl.config.DefaultDelay),
		Semaphore:       make(chan struct{}, arl.config.MaxConcurrentPerDomain),
		AdaptiveHistory: make([]RequestResult, 0),
	}
	
	arl.domainLimiters[domain] = limiter
	
	log.Debug().
		Str("domain", domain).
		Dur("default_delay", arl.config.DefaultDelay).
		Int("max_concurrent", arl.config.MaxConcurrentPerDomain).
		Msg("Created new domain limiter")
	
	return limiter
}

// Wait blocks until a request can be made for this domain
func (dl *DomainLimiter) Wait(ctx context.Context, requiredDelay time.Duration, config *RateLimiterConfig) error {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	// Acquire semaphore for concurrent requests
	select {
	case dl.Semaphore <- struct{}{}:
		defer func() { <-dl.Semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}
	
	// Calculate actual delay to use
	actualDelay := dl.CurrentDelay
	if requiredDelay > actualDelay {
		actualDelay = requiredDelay
	}
	
	// Apply minimum delay constraint
	if actualDelay < config.MinDelay {
		actualDelay = config.MinDelay
	}
	
	// Check if we need to wait based on last request time
	if !dl.LastRequest.IsZero() {
		elapsed := time.Since(dl.LastRequest)
		if elapsed < actualDelay {
			waitTime := actualDelay - elapsed
			
			log.Debug().
				Str("domain", dl.Domain).
				Dur("wait_time", waitTime).
				Dur("actual_delay", actualDelay).
				Dur("elapsed", elapsed).
				Msg("Waiting for rate limit")
			
			select {
			case <-time.After(waitTime):
				// Continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	
	// Acquire token from bucket
	if !dl.TokenBucket.TryAcquire() {
		select {
		case <-time.After(dl.TokenBucket.refillRate):
			if !dl.TokenBucket.TryAcquire() {
				return fmt.Errorf("domain rate limit exceeded for %s", dl.Domain)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	
	dl.LastRequest = time.Now()
	dl.RequestCount++
	
	return nil
}

// RecordRequest records the result of a request
func (dl *DomainLimiter) RecordRequest(result RequestResult, config *RateLimiterConfig) {
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	// Add to adaptive history
	dl.AdaptiveHistory = append(dl.AdaptiveHistory, result)
	
	// Keep only recent history (last 100 requests)
	if len(dl.AdaptiveHistory) > 100 {
		dl.AdaptiveHistory = dl.AdaptiveHistory[len(dl.AdaptiveHistory)-100:]
	}
	
	// Update counters
	if result.Success {
		dl.SuccessCount++
	} else {
		dl.ErrorCount++
	}
	
	// Adaptive delay adjustment
	if config.AdaptiveAdjustment {
		dl.adjustDelay(result, config)
	}
	
	log.Debug().
		Str("domain", dl.Domain).
		Int("status_code", result.StatusCode).
		Bool("success", result.Success).
		Bool("rate_limited", result.RateLimited).
		Dur("current_delay", dl.CurrentDelay).
		Int64("success_count", dl.SuccessCount).
		Int64("error_count", dl.ErrorCount).
		Msg("Recorded request result")
}

// adjustDelay adjusts the delay based on request results
func (dl *DomainLimiter) adjustDelay(result RequestResult, config *RateLimiterConfig) {
	if result.RateLimited || result.StatusCode == 429 {
		// Increase delay on rate limiting
		newDelay := time.Duration(float64(dl.CurrentDelay) * config.BackoffMultiplier)
		if newDelay > config.MaxBackoffDelay {
			newDelay = config.MaxBackoffDelay
		}
		dl.CurrentDelay = newDelay
		
		log.Debug().
			Str("domain", dl.Domain).
			Dur("old_delay", dl.CurrentDelay).
			Dur("new_delay", newDelay).
			Msg("Increased delay due to rate limiting")
		
	} else if result.Success && len(dl.AdaptiveHistory) >= 10 {
		// Analyze recent success rate
		recentRequests := dl.AdaptiveHistory
		if len(recentRequests) > 20 {
			recentRequests = recentRequests[len(recentRequests)-20:]
		}
		
		successCount := 0
		rateLimitedCount := 0
		
		for _, req := range recentRequests {
			if req.Success {
				successCount++
			}
			if req.RateLimited || req.StatusCode == 429 {
				rateLimitedCount++
			}
		}
		
		successRate := float64(successCount) / float64(len(recentRequests))
		rateLimitedRate := float64(rateLimitedCount) / float64(len(recentRequests))
		
		// Decrease delay if we have high success rate and low rate limiting
		if successRate > 0.9 && rateLimitedRate < 0.1 && dl.CurrentDelay > config.MinDelay {
			newDelay := time.Duration(float64(dl.CurrentDelay) / 1.2) // Reduce by 20%
			if newDelay < config.MinDelay {
				newDelay = config.MinDelay
			}
			dl.CurrentDelay = newDelay
			
			log.Debug().
				Str("domain", dl.Domain).
				Float64("success_rate", successRate).
				Dur("new_delay", newDelay).
				Msg("Decreased delay due to good performance")
		}
	}
	
	// Ensure delay stays within bounds
	if dl.CurrentDelay < config.MinDelay {
		dl.CurrentDelay = config.MinDelay
	}
	if dl.CurrentDelay > config.MaxDelay {
		dl.CurrentDelay = config.MaxDelay
	}
}

// TryAcquire attempts to acquire a token from the bucket
func (tb *TokenBucket) TryAcquire() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	tb.refill()
	
	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	
	return false
}

// refill adds tokens to the bucket based on elapsed time
func (tb *TokenBucket) refill() {
	now := time.Now()
	elapsed := now.Sub(tb.lastRefill)
	
	if elapsed >= tb.refillRate {
		tokensToAdd := int64(elapsed / tb.refillRate)
		tb.tokens += tokensToAdd
		
		if tb.tokens > tb.capacity {
			tb.tokens = tb.capacity
		}
		
		tb.lastRefill = now
	}
}

// GetTokens returns the current number of tokens
func (tb *TokenBucket) GetTokens() int64 {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	
	tb.refill()
	return tb.tokens
}

// GetAllDomainStats returns statistics for all domains
func (arl *AdaptiveRateLimiter) GetAllDomainStats() map[string]*DomainLimiter {
	arl.mu.RLock()
	defer arl.mu.RUnlock()
	
	stats := make(map[string]*DomainLimiter)
	
	for domain := range arl.domainLimiters {
		stats[domain] = arl.GetDomainStats(domain)
	}
	
	return stats
}

// CleanupInactive removes domain limiters that haven't been used recently
func (arl *AdaptiveRateLimiter) CleanupInactive(maxIdleTime time.Duration) int {
	arl.mu.Lock()
	defer arl.mu.Unlock()
	
	cleaned := 0
	now := time.Now()
	
	for domain, limiter := range arl.domainLimiters {
		limiter.mu.Lock()
		idle := now.Sub(limiter.LastRequest)
		limiter.mu.Unlock()
		
		if idle > maxIdleTime {
			delete(arl.domainLimiters, domain)
			cleaned++
		}
	}
	
	if cleaned > 0 {
		log.Info().
			Int("cleaned_domains", cleaned).
			Dur("max_idle_time", maxIdleTime).
			Msg("Cleaned up inactive domain limiters")
	}
	
	return cleaned
}

// UpdateDomainDelay manually updates the delay for a specific domain
func (arl *AdaptiveRateLimiter) UpdateDomainDelay(domain string, delay time.Duration) {
	domainLimiter := arl.getDomainLimiter(domain)
	
	domainLimiter.mu.Lock()
	defer domainLimiter.mu.Unlock()
	
	if delay < arl.config.MinDelay {
		delay = arl.config.MinDelay
	}
	if delay > arl.config.MaxDelay {
		delay = arl.config.MaxDelay
	}
	
	domainLimiter.CurrentDelay = delay
	
	log.Info().
		Str("domain", domain).
		Dur("new_delay", delay).
		Msg("Manually updated domain delay")
}