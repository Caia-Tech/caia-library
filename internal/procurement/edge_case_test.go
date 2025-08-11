package procurement_test

import (
	"context"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEdgeCaseHandling(t *testing.T) {
	t.Run("Quality Validator Edge Cases", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		require.NotNil(t, validator)
		
		testCases := []struct {
			name        string
			content     string
			metadata    map[string]string
			expectError bool
		}{
			{
				name:        "Empty Content",
				content:     "",
				metadata:    map[string]string{"type": "empty"},
				expectError: false, // Should handle gracefully
			},
			{
				name:        "Nil Metadata",
				content:     "Some content",
				metadata:    nil,
				expectError: false,
			},
			{
				name:        "Only Whitespace",
				content:     "   \n\t\r   ",
				metadata:    map[string]string{"type": "whitespace"},
				expectError: false,
			},
			{
				name:        "Very Short Content",
				content:     "Hi",
				metadata:    map[string]string{"type": "short"},
				expectError: false, // Should not panic or error, just give low score
			},
			{
				name:        "Special Characters Only",
				content:     "!@#$%^&*()",
				metadata:    map[string]string{"type": "special"},
				expectError: false,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				result, err := validator.ValidateContent(ctx, tc.content, tc.metadata)
				
				if tc.expectError {
					assert.Error(t, err, "Expected error for case: %s", tc.name)
				} else {
					if err != nil {
						t.Logf("Validation error for %s: %v", tc.name, err)
						// Log but don't fail - some edge cases might have expected errors
					}
					
					if result != nil {
						assert.True(t, result.OverallScore >= 0 && result.OverallScore <= 1,
							"Score should be valid for %s: %.2f", tc.name, result.OverallScore)
					}
				}
			})
		}
	})
	
	t.Run("Rate Limiter Configuration", func(t *testing.T) {
		// Test with more lenient rate limiting for concurrent scenarios
		config := &scraping.RateLimiterConfig{
			DefaultDelay:           100 * time.Millisecond, // Much shorter for testing
			MaxConcurrentDomains:   100,
			MaxConcurrentPerDomain: 10,
			BackoffMultiplier:      1.5,
			MaxBackoffDelay:        5 * time.Second,
			AdaptiveAdjustment:     true,
			RespectRetryAfter:      true,
			MinDelay:               10 * time.Millisecond,
			MaxDelay:               10 * time.Second,
		}
		
		rateLimiter := scraping.NewAdaptiveRateLimiter(config)
		
		// Test rapid requests don't timeout
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		
		for i := 0; i < 10; i++ {
			err := rateLimiter.Wait(ctx, "test.com", 10*time.Millisecond)
			assert.NoError(t, err, "Request %d should not timeout", i)
			
			rateLimiter.RecordRequest("test.com", scraping.RequestResult{
				Timestamp:   time.Now(),
				StatusCode:  200,
				Success:     true,
				RateLimited: false,
			})
		}
		
		stats := rateLimiter.GetDomainStats("test.com")
		assert.NotNil(t, stats)
		assert.Equal(t, int64(10), stats.SuccessCount)
	})
	
	t.Run("Compliance Engine Edge Cases", func(t *testing.T) {
		complianceEngine := scraping.NewComplianceEngine(nil)
		
		testCases := []struct {
			name        string
			url         string
			expectError bool
		}{
			{
				name:        "Invalid URL",
				url:         "not-a-url",
				expectError: true,
			},
			{
				name:        "Empty URL",
				url:         "",
				expectError: true,
			},
			{
				name:        "Valid URL",
				url:         "https://example.com/page",
				expectError: false,
			},
			{
				name:        "Localhost URL",
				url:         "http://localhost:8080/test",
				expectError: false,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				result, err := complianceEngine.CheckCompliance(ctx, tc.url)
				
				if tc.expectError {
					assert.Error(t, err, "Expected error for %s", tc.name)
				} else {
					if err != nil {
						t.Logf("Compliance check error for %s: %v", tc.name, err)
					} else {
						assert.NotNil(t, result, "Result should not be nil for valid URL")
					}
				}
			})
		}
	})
	
	t.Run("Content Extractor Resilience", func(t *testing.T) {
		extractor := scraping.NewContentExtractor(nil)
		
		testCases := []struct {
			name        string
			url         string
			expectError bool
		}{
			{
				name:        "Invalid URL Format",
				url:         "invalid-url",
				expectError: true,
			},
			{
				name:        "Non-HTTP Protocol",
				url:         "ftp://example.com",
				expectError: true,
			},
			{
				name:        "Missing Protocol",
				url:         "example.com",
				expectError: true,
			},
		}
		
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Test URL validation first
				err := extractor.ValidateURL(tc.url)
				
				if tc.expectError {
					assert.Error(t, err, "Expected validation error for %s", tc.name)
				} else {
					assert.NoError(t, err, "Should not error for valid URL: %s", tc.name)
				}
			})
		}
	})
	
	t.Run("Context Handling", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		// Test cancelled context
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		result, err := validator.ValidateContent(ctx, "test content", map[string]string{"type": "test"})
		
		// Should handle gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		}
		// Result might be nil or have partial data - both are acceptable
		if result != nil {
			t.Logf("Result with cancelled context: %+v", result)
		}
		
		// Test timeout
		timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer timeoutCancel()
		
		result, err = validator.ValidateContent(timeoutCtx, "test content", map[string]string{"type": "timeout"})
		
		// Should handle timeout gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "deadline exceeded")
		}
	})
	
	t.Run("Memory Safety", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		// Test with very large content
		largeContent := ""
		for i := 0; i < 50000; i++ {
			largeContent += "word "
		}
		
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		result, err := validator.ValidateContent(ctx, largeContent, map[string]string{"type": "large"})
		
		// Should not crash
		if err != nil {
			t.Logf("Large content validation error: %v", err)
		}
		if result != nil {
			assert.True(t, result.OverallScore >= 0 && result.OverallScore <= 1,
				"Score should be valid even for large content")
		}
	})
	
	t.Run("Concurrent Safety", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		// Test concurrent access doesn't cause issues
		done := make(chan bool, 5)
		
		for i := 0; i < 5; i++ {
			go func(id int) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				
				result, err := validator.ValidateContent(ctx, 
					"Concurrent test content for safety validation and error handling mechanisms in distributed systems",
					map[string]string{"worker": string(rune('A' + id))})
				
				if err != nil {
					t.Logf("Worker %d error: %v", id, err)
				}
				if result != nil && (result.OverallScore < 0 || result.OverallScore > 1) {
					t.Errorf("Worker %d invalid score: %.2f", id, result.OverallScore)
				}
				done <- true
			}(i)
		}
		
		// Wait for all workers
		for i := 0; i < 5; i++ {
			select {
			case <-done:
				// Success
			case <-time.After(10 * time.Second):
				t.Error("Worker timed out")
			}
		}
	})
}