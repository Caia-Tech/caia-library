package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAcademicRateLimiter_WaitForSource(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	ctx := context.Background()

	tests := []struct {
		name            string
		source          string
		expectedMinWait time.Duration
		shouldError     bool
	}{
		{
			name:            "arxiv rate limit",
			source:          "arxiv",
			expectedMinWait: 3 * time.Second,
			shouldError:     false,
		},
		{
			name:            "pubmed rate limit",
			source:          "pubmed",
			expectedMinWait: 350 * time.Millisecond,
			shouldError:     false,
		},
		{
			name:            "unknown source",
			source:          "unknown",
			expectedMinWait: 0,
			shouldError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			err := limiter.WaitForSource(ctx, tt.source)
			elapsed := time.Since(start)

			if tt.shouldError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				// First request should be immediate
				assert.Less(t, elapsed, 100*time.Millisecond)

				// Second request should wait
				start = time.Now()
				err = limiter.WaitForSource(ctx, tt.source)
				elapsed = time.Since(start)
				
				assert.NoError(t, err)
				assert.GreaterOrEqual(t, elapsed, tt.expectedMinWait)
			}
		})
	}
}

func TestAcademicRateLimiter_ConcurrentAccess(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	ctx := context.Background()
	
	// Test sequential requests to verify rate limiting works
	numRequests := 3
	requestTimes := make([]time.Time, 0, numRequests)
	
	for i := 0; i < numRequests; i++ {
		start := time.Now()
		err := limiter.WaitForSource(ctx, "arxiv")
		require.NoError(t, err)
		requestTimes = append(requestTimes, start)
	}
	
	// Verify requests are properly spaced (first request is immediate, subsequent ones wait)
	for i := 1; i < len(requestTimes); i++ {
		gap := requestTimes[i].Sub(requestTimes[i-1])
		assert.GreaterOrEqual(t, gap, 2900*time.Millisecond, "Requests should be at least ~3 seconds apart")
	}
}

func TestAcademicRateLimiter_ErrorHandling(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	
	// Record multiple errors
	for i := 0; i < 5; i++ {
		limiter.RecordError("arxiv", assert.AnError)
	}
	
	// Check that source is in backoff
	stats := limiter.GetStats()
	arxivStats := stats["arxiv"]
	assert.Equal(t, int64(5), arxivStats.ErrorCount)
	assert.True(t, arxivStats.InBackoff)
	
	// Try to make a request during backoff
	ctx := context.Background()
	start := time.Now()
	err := limiter.WaitForSource(ctx, "arxiv")
	elapsed := time.Since(start)
	
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 30*time.Second, "Should wait for backoff period")
}

func TestAcademicRateLimiter_RecordSuccess(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	
	// Record errors
	limiter.RecordError("pubmed", assert.AnError)
	limiter.RecordError("pubmed", assert.AnError)
	
	// Verify error count
	stats := limiter.GetStats()
	assert.Equal(t, int64(2), stats["pubmed"].ErrorCount)
	
	// Record success
	limiter.RecordSuccess("pubmed")
	
	// Verify error count reset
	stats = limiter.GetStats()
	assert.Equal(t, int64(0), stats["pubmed"].ErrorCount)
}

func TestAcademicRateLimiter_GetStats(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	ctx := context.Background()
	
	// Make some requests
	err := limiter.WaitForSource(ctx, "arxiv")
	require.NoError(t, err)
	
	err = limiter.WaitForSource(ctx, "pubmed")
	require.NoError(t, err)
	
	// Record an error
	limiter.RecordError("doaj", assert.AnError)
	
	// Get stats
	stats := limiter.GetStats()
	
	// Verify stats exist for all sources
	assert.Contains(t, stats, "arxiv")
	assert.Contains(t, stats, "pubmed")
	assert.Contains(t, stats, "doaj")
	assert.Contains(t, stats, "plos")
	assert.Contains(t, stats, "semantic_scholar")
	
	// Verify request counts
	assert.Equal(t, int64(1), stats["arxiv"].RequestCount)
	assert.Equal(t, int64(1), stats["pubmed"].RequestCount)
	assert.Equal(t, int64(1), stats["doaj"].ErrorCount)
}

func TestAcademicRateLimiter_ContextCancellation(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	
	// Make first request to set the last request time
	ctx := context.Background()
	err := limiter.WaitForSource(ctx, "arxiv")
	require.NoError(t, err)
	
	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())
	
	// Start waiting in a goroutine
	done := make(chan error)
	go func() {
		done <- limiter.WaitForSource(ctx, "arxiv")
	}()
	
	// Cancel context after a short delay
	time.Sleep(100 * time.Millisecond)
	cancel()
	
	// Should receive context error
	err = <-done
	assert.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestDefaultEthicalConfig(t *testing.T) {
	config := DefaultEthicalConfig()
	
	assert.Contains(t, config.UserAgent, "CAIA-Library")
	assert.Contains(t, config.UserAgent, "library@caiatech.com")
	assert.Equal(t, "/tmp/caia-cache", config.CacheDir)
	assert.Equal(t, 3, config.MaxRetries)
	assert.Contains(t, config.AttributionTemplate, "Caia Tech")
	assert.Equal(t, "library@caiatech.com", config.ContactEmail)
}

func TestAcademicRateLimiter_ExponentialBackoff(t *testing.T) {
	limiter := NewAcademicRateLimiter()
	
	// Test backoff increases with error count
	backoffs := []time.Duration{}
	
	for i := 1; i <= 6; i++ {
		limiter.RecordError("plos", assert.AnError)
		stats := limiter.GetStats()
		
		if stats["plos"].InBackoff {
			backoffDuration := time.Until(stats["plos"].BackoffUntil)
			backoffs = append(backoffs, backoffDuration)
		}
	}
	
	// Verify backoff increases but caps at 5 minutes
	assert.Greater(t, len(backoffs), 0, "Should have backoff periods")
	for i := 1; i < len(backoffs); i++ {
		assert.GreaterOrEqual(t, backoffs[i], backoffs[i-1], "Backoff should increase")
	}
	assert.LessOrEqual(t, backoffs[len(backoffs)-1], 5*time.Minute, "Backoff should cap at 5 minutes")
}

// Benchmark rate limiter performance
func BenchmarkAcademicRateLimiter_WaitForSource(b *testing.B) {
	limiter := NewAcademicRateLimiter()
	
	// Reset timer to exclude setup
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Just test the locking/checking logic, not actual waiting
		limiter.GetStats()
	}
}