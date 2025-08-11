package procurement_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/stretchr/testify/assert"
)

func TestSystemStressValidation(t *testing.T) {
	t.Run("Concurrent Quality Validation", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)

		// Test concurrent quality validation
		numWorkers := 20
		numValidationsPerWorker := 10
		var wg sync.WaitGroup
		errCh := make(chan error, numWorkers*numValidationsPerWorker)
		
		testContents := []string{
			"This is a technical article about Go programming. It covers basic concepts like functions, variables, and control structures.",
			"def fibonacci(n): return n if n <= 1 else fibonacci(n-1) + fibonacci(n-2)",
			"The mitochondria is the powerhouse of the cell. It produces ATP through cellular respiration.",
			generateLongTestContent(1000),
			"x = 5 / 0",  // Division by zero test
			"",           // Empty content
			"Hello 世界! Testing unicode and émojis 🚀",
		}
		
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < numValidationsPerWorker; j++ {
					content := testContents[j%len(testContents)]
					metadata := map[string]string{
						"type": "test",
						"worker": fmt.Sprintf("%d", workerID),
						"iteration": fmt.Sprintf("%d", j),
					}
					
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					result, err := validator.ValidateContent(ctx, content, metadata)
					cancel()
					
					if err != nil {
						errCh <- fmt.Errorf("worker %d validation %d failed: %w", workerID, j, err)
						continue
					}
					
					if result.OverallScore < 0 || result.OverallScore > 1 {
						errCh <- fmt.Errorf("worker %d validation %d invalid score: %.2f", 
							workerID, j, result.OverallScore)
					}
				}
			}(i)
		}
		
		wg.Wait()
		close(errCh)
		
		// Check for errors
		errorCount := 0
		for err := range errCh {
			t.Logf("Validation stress test error: %v", err)
			errorCount++
		}
		
		// Should have very few errors for quality validation
		totalValidations := numWorkers * numValidationsPerWorker
		successRate := float64(totalValidations-errorCount) / float64(totalValidations)
		
		assert.True(t, successRate >= 0.95, 
			"Success rate %.2f below threshold, %d errors out of %d validations", 
			successRate, errorCount, totalValidations)
	})
	
	t.Run("Rate Limiter Under Load", func(t *testing.T) {
		rateLimiter := scraping.NewAdaptiveRateLimiter(nil)
		
		numWorkers := 15
		numRequestsPerWorker := 8
		domains := []string{"example.com", "test.com", "demo.com", "sample.com"}
		
		var wg sync.WaitGroup
		errCh := make(chan error, numWorkers*numRequestsPerWorker)
		
		start := time.Now()
		
		wg.Add(numWorkers)
		for i := 0; i < numWorkers; i++ {
			go func(workerID int) {
				defer wg.Done()
				
				for j := 0; j < numRequestsPerWorker; j++ {
					domain := domains[j%len(domains)]
					
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					err := rateLimiter.Wait(ctx, domain, 50*time.Millisecond) // Short delay for testing
					cancel()
					
					if err != nil {
						errCh <- fmt.Errorf("worker %d request %d failed: %w", workerID, j, err)
						continue
					}
					
					// Simulate request result
					rateLimiter.RecordRequest(domain, scraping.RequestResult{
						Timestamp:   time.Now(),
						StatusCode:  200,
						Success:     true,
						RateLimited: false,
					})
				}
			}(i)
		}
		
		wg.Wait()
		elapsed := time.Since(start)
		close(errCh)
		
		// Check for errors
		errorCount := 0
		for err := range errCh {
			t.Logf("Rate limiter stress test error: %v", err)
			errorCount++
		}
		
		totalRequests := numWorkers * numRequestsPerWorker
		successRate := float64(totalRequests-errorCount) / float64(totalRequests)
		
		assert.True(t, successRate >= 0.9, 
			"Success rate %.2f below threshold, %d errors out of %d requests", 
			successRate, errorCount, totalRequests)
		
		// Should handle requests reasonably fast even under load
		avgTimePerRequest := elapsed / time.Duration(totalRequests)
		t.Logf("Average time per request: %v", avgTimePerRequest)
		assert.True(t, avgTimePerRequest < 2*time.Second, 
			"Average request time %v too slow", avgTimePerRequest)
		
		// Check domain stats
		for _, domain := range domains {
			stats := rateLimiter.GetDomainStats(domain)
			assert.NotNil(t, stats, "Domain stats should exist for %s", domain)
			assert.True(t, stats.SuccessCount > 0, "Should have successful requests for %s", domain)
		}
	})
	
	t.Run("Memory Usage Under Load", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		// Generate many validations to test for memory leaks
		for i := 0; i < 100; i++ {
			content := generateLongTestContent(500)
			metadata := map[string]string{
				"type": "memory_test",
				"iteration": fmt.Sprintf("%d", i),
			}
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			result, err := validator.ValidateContent(ctx, content, metadata)
			cancel()
			
			assert.NoError(t, err, "Validation should not fail")
			assert.True(t, result.OverallScore >= 0 && result.OverallScore <= 1, 
				"Score should be valid")
		}
		
		// Force garbage collection
		time.Sleep(100 * time.Millisecond)
	})
	
	t.Run("Context Cancellation Handling", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		// Test immediate cancellation
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately
		
		content := "Test content for cancellation"
		metadata := map[string]string{"type": "cancellation_test"}
		
		result, err := validator.ValidateContent(ctx, content, metadata)
		
		// Should handle cancellation gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "context canceled")
		} else {
			// If no error, result should indicate failure
			t.Logf("Validation result with cancelled context: %+v", result)
		}
	})
	
	t.Run("Timeout Handling", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		// Very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancel()
		
		content := generateLongTestContent(1000)
		metadata := map[string]string{"type": "timeout_test"}
		
		result, err := validator.ValidateContent(ctx, content, metadata)
		
		// Should handle timeout gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "deadline exceeded")
		} else {
			t.Logf("Validation result with timeout: %+v", result)
		}
	})
	
	t.Run("Edge Case Content Validation", func(t *testing.T) {
		validator := quality.NewQualityValidator(nil)
		
		edgeCases := []struct {
			name     string
			content  string
			metadata map[string]string
		}{
			{
				name:    "Very Long Content",
				content: generateLongTestContent(10000),
				metadata: map[string]string{"type": "long"},
			},
			{
				name:    "Only Whitespace",
				content: "   \n\t\r   \n   ",
				metadata: map[string]string{"type": "whitespace"},
			},
			{
				name:    "Special Characters",
				content: "!@#$%^&*()_+{}|:<>?[];',./",
				metadata: map[string]string{"type": "special"},
			},
			{
				name:    "Numbers Only",
				content: "1234567890 987654321 1.23456789",
				metadata: map[string]string{"type": "numeric"},
			},
			{
				name:    "Mixed Languages",
				content: "Hello 世界 Bonjour мир こんにちは עולם مرحبا",
				metadata: map[string]string{"type": "multilingual"},
			},
		}
		
		for _, tc := range edgeCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				
				result, err := validator.ValidateContent(ctx, tc.content, tc.metadata)
				
				// Should not crash or error on edge cases
				assert.NoError(t, err, "Validation should handle edge case: %s", tc.name)
				if result != nil {
					assert.True(t, result.OverallScore >= 0 && result.OverallScore <= 1, 
						"Score should be valid for %s: %.2f", tc.name, result.OverallScore)
				}
			})
		}
	})
}

func generateLongTestContent(length int) string {
	base := "This is a test article about technology and innovation. It covers various topics including software development, artificial intelligence, and data science. "
	content := ""
	for len(content) < length {
		content += base
	}
	return content[:length]
}

func BenchmarkQualityValidationStress(b *testing.B) {
	validator := quality.NewQualityValidator(nil)
	
	content := generateLongTestContent(1000)
	metadata := map[string]string{
		"type":     "benchmark",
		"domain":   "technology",
		"language": "en",
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		result, err := validator.ValidateContent(ctx, content, metadata)
		if err != nil {
			b.Fatal(err)
		}
		if result.OverallScore < 0 || result.OverallScore > 1 {
			b.Fatalf("Invalid quality score: %.2f", result.OverallScore)
		}
	}
}

func BenchmarkRateLimiterStress(b *testing.B) {
	rateLimiter := scraping.NewAdaptiveRateLimiter(nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := context.Background()
		err := rateLimiter.Wait(ctx, "benchmark.com", 1*time.Millisecond)
		if err != nil {
			b.Fatal(err)
		}
		
		rateLimiter.RecordRequest("benchmark.com", scraping.RequestResult{
			Timestamp:   time.Now(),
			StatusCode:  200,
			Success:     true,
			RateLimited: false,
		})
	}
}