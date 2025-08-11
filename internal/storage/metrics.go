package storage

import (
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// SimpleMetricsCollector provides basic metrics collection for storage operations
type SimpleMetricsCollector struct {
	metrics []StorageMetrics
	mutex   sync.RWMutex
}

// NewSimpleMetricsCollector creates a new simple metrics collector
func NewSimpleMetricsCollector() *SimpleMetricsCollector {
	return &SimpleMetricsCollector{
		metrics: make([]StorageMetrics, 0),
	}
}

// RecordMetric records a storage operation metric
func (s *SimpleMetricsCollector) RecordMetric(metric StorageMetrics) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.metrics = append(s.metrics, metric)
	
	// Log the metric for debugging
	logger := log.With().
		Str("operation", metric.OperationType).
		Str("backend", metric.Backend).
		Int64("duration_ns", metric.Duration).
		Bool("success", metric.Success).
		Logger()
	
	if metric.Error != nil {
		logger = logger.With().Err(metric.Error).Logger()
	}
	
	logger.Debug().Msg("Storage operation metric recorded")
}

// GetMetrics returns all collected metrics
func (s *SimpleMetricsCollector) GetMetrics() []StorageMetrics {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	// Return a copy to prevent race conditions
	result := make([]StorageMetrics, len(s.metrics))
	copy(result, s.metrics)
	return result
}

// GetMetricsSummary returns a summary of metrics for analysis
func (s *SimpleMetricsCollector) GetMetricsSummary() map[string]interface{} {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	
	summary := make(map[string]interface{})
	
	// Group by backend and operation
	byBackend := make(map[string]map[string]*OperationStats)
	
	for _, metric := range s.metrics {
		if byBackend[metric.Backend] == nil {
			byBackend[metric.Backend] = make(map[string]*OperationStats)
		}
		
		if byBackend[metric.Backend][metric.OperationType] == nil {
			byBackend[metric.Backend][metric.OperationType] = &OperationStats{}
		}
		
		stats := byBackend[metric.Backend][metric.OperationType]
		stats.Count++
		stats.TotalDuration += metric.Duration
		
		if metric.Success {
			stats.SuccessCount++
		} else {
			stats.FailureCount++
		}
		
		// Track min/max/avg duration
		if stats.Count == 1 {
			stats.MinDuration = metric.Duration
			stats.MaxDuration = metric.Duration
		} else {
			if metric.Duration < stats.MinDuration {
				stats.MinDuration = metric.Duration
			}
			if metric.Duration > stats.MaxDuration {
				stats.MaxDuration = metric.Duration
			}
		}
		stats.AvgDuration = stats.TotalDuration / int64(stats.Count)
	}
	
	summary["by_backend"] = byBackend
	summary["total_operations"] = len(s.metrics)
	
	return summary
}

// ClearMetrics clears all collected metrics
func (s *SimpleMetricsCollector) ClearMetrics() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	
	s.metrics = make([]StorageMetrics, 0)
}

// OperationStats holds statistics for a specific operation type
type OperationStats struct {
	Count         int   `json:"count"`
	SuccessCount  int   `json:"success_count"`
	FailureCount  int   `json:"failure_count"`
	TotalDuration int64 `json:"total_duration_ns"`
	MinDuration   int64 `json:"min_duration_ns"`
	MaxDuration   int64 `json:"max_duration_ns"`
	AvgDuration   int64 `json:"avg_duration_ns"`
}

// GetSuccessRate returns the success rate as a percentage
func (o *OperationStats) GetSuccessRate() float64 {
	if o.Count == 0 {
		return 0.0
	}
	return float64(o.SuccessCount) / float64(o.Count) * 100.0
}

// GetAvgDurationMs returns the average duration in milliseconds
func (o *OperationStats) GetAvgDurationMs() float64 {
	return float64(o.AvgDuration) / float64(time.Millisecond)
}