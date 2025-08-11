package synthetic

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/rs/zerolog/log"
)

// SyntheticGenerator implements multi-LLM content generation
type SyntheticGenerator struct {
	models           map[string]procurement.LLMProvider
	qualityValidator procurement.QualityValidator
	contentPlanner   procurement.ContentPlanner
	attributionMgr   procurement.AttributionManager
	storage          storage.StorageBackend
	config          *procurement.ServiceConfig
	
	// Metrics and monitoring
	metrics     *procurement.ProcurementMetrics
	metricsMu   sync.RWMutex
	
	// Request tracking
	activeRequests map[string]*RequestContext
	requestsMu     sync.RWMutex
	
	// Template management
	templates map[procurement.ContentType]*procurement.ContentTemplate
	templatesMu sync.RWMutex
}

// RequestContext tracks the context of a generation request
type RequestContext struct {
	Request     *procurement.GenerationRequest
	Status      procurement.ProcessingStatus
	StartTime   time.Time
	Model       string
	RetryCount  int
	LastError   error
}

// NewSyntheticGenerator creates a new synthetic content generator
func NewSyntheticGenerator(
	models map[string]procurement.LLMProvider,
	validator procurement.QualityValidator,
	planner procurement.ContentPlanner,
	attribution procurement.AttributionManager,
	storage storage.StorageBackend,
	config *procurement.ServiceConfig,
) *SyntheticGenerator {
	
	generator := &SyntheticGenerator{
		models:          models,
		qualityValidator: validator,
		contentPlanner:  planner,
		attributionMgr:  attribution,
		storage:         storage,
		config:          config,
		activeRequests:  make(map[string]*RequestContext),
		templates:       make(map[procurement.ContentType]*procurement.ContentTemplate),
		metrics: &procurement.ProcurementMetrics{
			ModelPerformance:    make(map[string]*procurement.ModelMetrics),
			QualityDistribution: make(map[string]int),
			TopicDistribution:   make(map[string]int),
			LastUpdated:        time.Now(),
		},
	}
	
	// Initialize model metrics
	for modelName := range models {
		generator.metrics.ModelPerformance[modelName] = &procurement.ModelMetrics{
			ModelName: modelName,
		}
	}
	
	// Load default templates
	generator.loadDefaultTemplates()
	
	return generator
}

// GenerateContent generates synthetic content for a single request
func (sg *SyntheticGenerator) GenerateContent(ctx context.Context, request *procurement.GenerationRequest) (*procurement.GenerationResult, error) {
	start := time.Now()
	
	// Track request
	reqCtx := &RequestContext{
		Request:   request,
		Status:    procurement.StatusPending,
		StartTime: start,
	}
	sg.trackRequest(request.ID, reqCtx)
	defer sg.untrackRequest(request.ID)
	
	log.Info().
		Str("request_id", request.ID).
		Str("topic", request.Topic.Name).
		Str("content_type", string(request.ContentType)).
		Msg("Starting synthetic content generation")
	
	// Update status
	reqCtx.Status = procurement.StatusProcessing
	
	// Select optimal model for request
	model, err := sg.selectOptimalModel(request)
	if err != nil {
		return sg.createFailureResult(request, start, err), nil
	}
	reqCtx.Model = model.GetModelName()
	
	// Generate content with retries
	result, err := sg.generateWithRetries(ctx, request, model)
	if err != nil {
		return sg.createFailureResult(request, start, err), nil
	}
	
	// Validate quality
	reqCtx.Status = procurement.StatusValidating
	if err := sg.validateAndEnhanceResult(ctx, result); err != nil {
		log.Error().Err(err).Str("request_id", request.ID).Msg("Quality validation failed")
		return sg.createFailureResult(request, start, err), nil
	}
	
	// Store document
	if result.Document != nil {
		if _, err := sg.storage.StoreDocument(ctx, result.Document); err != nil {
			log.Error().Err(err).Str("request_id", request.ID).Msg("Document storage failed")
			// Don't fail the request, just log the error
		}
	}
	
	// Update metrics
	sg.updateMetrics(result, model.GetModelName(), time.Since(start))
	
	reqCtx.Status = procurement.StatusCompleted
	result.ProcessingTime = time.Since(start)
	result.Success = true
	
	log.Info().
		Str("request_id", request.ID).
		Float64("quality_score", result.QualityScore).
		Dur("processing_time", result.ProcessingTime).
		Msg("Synthetic content generation completed")
	
	return result, nil
}

// GenerateBatch generates content for multiple requests concurrently
func (sg *SyntheticGenerator) GenerateBatch(ctx context.Context, requests []*procurement.GenerationRequest) ([]*procurement.GenerationResult, error) {
	if len(requests) == 0 {
		return []*procurement.GenerationResult{}, nil
	}
	
	log.Info().Int("batch_size", len(requests)).Msg("Starting batch content generation")
	
	// Create channels for results
	resultChan := make(chan *procurement.GenerationResult, len(requests))
	semaphore := make(chan struct{}, sg.config.MaxConcurrentRequests)
	
	var wg sync.WaitGroup
	
	// Process requests concurrently
	for _, request := range requests {
		wg.Add(1)
		go func(req *procurement.GenerationRequest) {
			defer wg.Done()
			
			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			
			// Generate content
			result, err := sg.GenerateContent(ctx, req)
			if err != nil {
				result = sg.createFailureResult(req, time.Now(), err)
			}
			
			resultChan <- result
		}(request)
	}
	
	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()
	
	// Collect results
	var results []*procurement.GenerationResult
	for result := range resultChan {
		results = append(results, result)
	}
	
	log.Info().
		Int("total_requests", len(requests)).
		Int("successful", sg.countSuccessfulResults(results)).
		Int("failed", len(results)-sg.countSuccessfulResults(results)).
		Msg("Batch content generation completed")
	
	return results, nil
}

// selectOptimalModel chooses the best model for a given request
func (sg *SyntheticGenerator) selectOptimalModel(request *procurement.GenerationRequest) (procurement.LLMProvider, error) {
	var bestModel procurement.LLMProvider
	var bestScore float64
	
	for modelName, model := range sg.models {
		if !model.IsAvailable() {
			continue
		}
		
		// Check if model is enabled
		if !sg.isModelEnabled(modelName) {
			continue
		}
		
		// Calculate model score based on various factors
		score := sg.calculateModelScore(model, request)
		
		if bestModel == nil || score > bestScore {
			bestModel = model
			bestScore = score
		}
	}
	
	if bestModel == nil {
		return nil, fmt.Errorf("no suitable model available for request")
	}
	
	return bestModel, nil
}

// calculateModelScore evaluates how suitable a model is for a request
func (sg *SyntheticGenerator) calculateModelScore(model procurement.LLMProvider, request *procurement.GenerationRequest) float64 {
	score := 0.0
	
	// Base capability score
	capabilities := model.GetCapabilities()
	for _, capability := range capabilities {
		if sg.isCapabilityRequired(request, capability) {
			score += 0.3
		}
	}
	
	// Performance score from metrics
	metrics := sg.getModelMetrics(model.GetModelName())
	if metrics != nil {
		score += metrics.SuccessRate * 0.25        // 25% weight on success rate
		score += metrics.AverageQuality * 0.25     // 25% weight on quality
		
		// Cost efficiency (lower cost = higher score)
		if metrics.CostPerRequest > 0 {
			costScore := 1.0 / (1.0 + metrics.CostPerRequest*100) // Normalize cost impact
			score += costScore * 0.2 // 20% weight on cost
		}
	}
	
	// Priority boost for preferred models
	if weight, exists := sg.config.ModelWeights[model.GetModelName()]; exists {
		score *= weight
	}
	
	return score
}

// generateWithRetries generates content with retry logic
func (sg *SyntheticGenerator) generateWithRetries(ctx context.Context, request *procurement.GenerationRequest, model procurement.LLMProvider) (*procurement.GenerationResult, error) {
	var lastErr error
	retryPolicy := sg.config.RetryPolicy
	
	for attempt := 0; attempt <= retryPolicy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := time.Duration(float64(retryPolicy.BaseDelay) * sg.calculateBackoffFactor(attempt, retryPolicy.BackoffFactor))
			if delay > retryPolicy.MaxDelay {
				delay = retryPolicy.MaxDelay
			}
			
			log.Warn().
				Str("request_id", request.ID).
				Int("attempt", attempt).
				Dur("delay", delay).
				Err(lastErr).
				Msg("Retrying content generation after delay")
			
			select {
			case <-time.After(delay):
				// Continue with retry
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		
		// Create timeout context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, sg.config.DefaultTimeout)
		
		result, err := model.Generate(attemptCtx, request)
		cancel()
		
		if err == nil && result != nil {
			result.GenerationModel = model.GetModelName()
			return result, nil
		}
		
		lastErr = err
		log.Warn().
			Str("request_id", request.ID).
			Str("model", model.GetModelName()).
			Int("attempt", attempt+1).
			Err(err).
			Msg("Content generation attempt failed")
	}
	
	return nil, fmt.Errorf("content generation failed after %d attempts: %w", retryPolicy.MaxRetries+1, lastErr)
}

// validateAndEnhanceResult validates generated content and enhances the result
func (sg *SyntheticGenerator) validateAndEnhanceResult(ctx context.Context, result *procurement.GenerationResult) error {
	if result.Document == nil {
		return fmt.Errorf("no document generated")
	}
	
	// Validate content quality
	validation, err := sg.qualityValidator.ValidateContent(ctx, result.Document.Content.Text, result.Document.Content.Metadata)
	if err != nil {
		return fmt.Errorf("quality validation failed: %w", err)
	}
	
	result.ValidationResult = validation
	result.QualityScore = validation.OverallScore
	
	// Check quality threshold
	if result.QualityScore < sg.config.QualityThreshold {
		return fmt.Errorf("content quality score (%.2f) below threshold (%.2f)", result.QualityScore, sg.config.QualityThreshold)
	}
	
	// Enrich document metadata and attribution
	if err := sg.attributionMgr.EnrichMetadata(result.Document, result); err != nil {
		log.Warn().Err(err).Msg("Failed to enrich document metadata")
		// Don't fail the request for metadata issues
	}
	
	return nil
}

// Helper methods

func (sg *SyntheticGenerator) trackRequest(requestID string, ctx *RequestContext) {
	sg.requestsMu.Lock()
	defer sg.requestsMu.Unlock()
	sg.activeRequests[requestID] = ctx
}

func (sg *SyntheticGenerator) untrackRequest(requestID string) {
	sg.requestsMu.Lock()
	defer sg.requestsMu.Unlock()
	delete(sg.activeRequests, requestID)
}

func (sg *SyntheticGenerator) isModelEnabled(modelName string) bool {
	for _, enabled := range sg.config.EnabledModels {
		if enabled == modelName {
			return true
		}
	}
	return len(sg.config.EnabledModels) == 0 // If no specific models enabled, allow all
}

func (sg *SyntheticGenerator) isCapabilityRequired(request *procurement.GenerationRequest, capability string) bool {
	// Define capability requirements based on content type and topic
	switch request.ContentType {
	case procurement.ContentTypeCodeExample:
		return capability == "code_generation"
	case procurement.ContentTypeResearchAbstract:
		return capability == "research_writing"
	case procurement.ContentTypeTutorial:
		return capability == "educational_content"
	default:
		return capability == "general_writing"
	}
}

func (sg *SyntheticGenerator) getModelMetrics(modelName string) *procurement.ModelMetrics {
	sg.metricsMu.RLock()
	defer sg.metricsMu.RUnlock()
	return sg.metrics.ModelPerformance[modelName]
}

func (sg *SyntheticGenerator) calculateBackoffFactor(attempt int, factor float64) float64 {
	result := 1.0
	for i := 0; i < attempt; i++ {
		result *= factor
	}
	return result
}

func (sg *SyntheticGenerator) createFailureResult(request *procurement.GenerationRequest, startTime time.Time, err error) *procurement.GenerationResult {
	return &procurement.GenerationResult{
		RequestID:      request.ID,
		Success:        false,
		Error:          err.Error(),
		ProcessingTime: time.Since(startTime),
		CreatedAt:      time.Now(),
	}
}

func (sg *SyntheticGenerator) countSuccessfulResults(results []*procurement.GenerationResult) int {
	count := 0
	for _, result := range results {
		if result.Success {
			count++
		}
	}
	return count
}

func (sg *SyntheticGenerator) updateMetrics(result *procurement.GenerationResult, modelName string, duration time.Duration) {
	sg.metricsMu.Lock()
	defer sg.metricsMu.Unlock()
	
	// Update overall metrics
	sg.metrics.RequestsProcessed++
	if result.Success {
		sg.metrics.RequestsSuccessful++
		
		// Update quality score average
		totalQuality := sg.metrics.AverageQualityScore * float64(sg.metrics.RequestsSuccessful-1)
		sg.metrics.AverageQualityScore = (totalQuality + result.QualityScore) / float64(sg.metrics.RequestsSuccessful)
		
		// Update quality distribution
		tier := sg.getQualityTier(result.QualityScore)
		sg.metrics.QualityDistribution[string(tier)]++
	} else {
		sg.metrics.RequestsFailed++
	}
	
	// Update processing time average
	totalTime := sg.metrics.AverageProcessingTime * time.Duration(sg.metrics.RequestsProcessed-1)
	sg.metrics.AverageProcessingTime = (totalTime + duration) / time.Duration(sg.metrics.RequestsProcessed)
	
	// Update model-specific metrics
	modelMetrics := sg.metrics.ModelPerformance[modelName]
	modelMetrics.RequestsHandled++
	modelMetrics.SuccessRate = float64(sg.metrics.RequestsSuccessful) / float64(modelMetrics.RequestsHandled)
	modelMetrics.LastUsed = time.Now()
	
	if result.Success {
		// Update model quality average
		totalQuality := modelMetrics.AverageQuality * float64(modelMetrics.RequestsHandled-1)
		modelMetrics.AverageQuality = (totalQuality + result.QualityScore) / float64(modelMetrics.RequestsHandled)
		
		// Update model latency average
		totalLatency := modelMetrics.AverageLatency * time.Duration(modelMetrics.RequestsHandled-1)
		modelMetrics.AverageLatency = (totalLatency + duration) / time.Duration(modelMetrics.RequestsHandled)
	}
	
	sg.metrics.LastUpdated = time.Now()
}

func (sg *SyntheticGenerator) getQualityTier(score float64) procurement.QualityTier {
	switch {
	case score >= 0.9:
		return procurement.QualityTierPremium
	case score >= 0.8:
		return procurement.QualityTierHigh
	case score >= 0.65:
		return procurement.QualityTierMedium
	case score >= 0.5:
		return procurement.QualityTierLow
	default:
		return procurement.QualityTierReject
	}
}

func (sg *SyntheticGenerator) loadDefaultTemplates() {
	// Load default templates for each content type
	defaultTemplates := map[procurement.ContentType]string{
		procurement.ContentTypeResearchAbstract: `# {{.Title}}

## Abstract
{{.Background}}

## Methodology
{{.Methodology}}

## Results
{{.Results}}

## Implications
{{.Implications}}

## Keywords
{{.Keywords}}`,
		
		procurement.ContentTypeTutorial: `# {{.Title}}

## Overview
{{.Overview}}

## Prerequisites
{{.Prerequisites}}

## Step-by-Step Guide
{{.Steps}}

## Example Implementation
{{.Example}}

## Common Issues & Solutions
{{.Troubleshooting}}`,
		
		procurement.ContentTypeDocumentation: `# {{.Title}}

## Description
{{.Description}}

## Usage
{{.Usage}}

## Parameters
{{.Parameters}}

## Examples
{{.Examples}}

## Notes
{{.Notes}}`,
	}
	
	sg.templatesMu.Lock()
	defer sg.templatesMu.Unlock()
	
	for contentType, template := range defaultTemplates {
		sg.templates[contentType] = &procurement.ContentTemplate{
			ID:          fmt.Sprintf("default_%s", string(contentType)),
			Name:        fmt.Sprintf("Default %s Template", string(contentType)),
			ContentType: contentType,
			Template:    template,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
	}
}

// GetMetrics returns current procurement metrics
func (sg *SyntheticGenerator) GetMetrics() *procurement.ProcurementMetrics {
	sg.metricsMu.RLock()
	defer sg.metricsMu.RUnlock()
	
	// Return a copy to prevent race conditions
	metrics := *sg.metrics
	modelPerf := make(map[string]*procurement.ModelMetrics)
	for name, perf := range sg.metrics.ModelPerformance {
		perfCopy := *perf
		modelPerf[name] = &perfCopy
	}
	metrics.ModelPerformance = modelPerf
	
	return &metrics
}

// GetModelPerformance returns performance metrics for a specific model
func (sg *SyntheticGenerator) GetModelPerformance(modelName string) (*procurement.ModelMetrics, error) {
	sg.metricsMu.RLock()
	defer sg.metricsMu.RUnlock()
	
	metrics, exists := sg.metrics.ModelPerformance[modelName]
	if !exists {
		return nil, fmt.Errorf("model %s not found", modelName)
	}
	
	// Return a copy
	metricsCopy := *metrics
	return &metricsCopy, nil
}

// UpdateConfiguration updates the service configuration
func (sg *SyntheticGenerator) UpdateConfiguration(config *procurement.ServiceConfig) error {
	if config == nil {
		return fmt.Errorf("configuration cannot be nil")
	}
	
	// Validate configuration
	if config.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("max concurrent requests must be positive")
	}
	
	if config.QualityThreshold < 0 || config.QualityThreshold > 1 {
		return fmt.Errorf("quality threshold must be between 0 and 1")
	}
	
	sg.config = config
	log.Info().Msg("Synthetic generator configuration updated")
	
	return nil
}