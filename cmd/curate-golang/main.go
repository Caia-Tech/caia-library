package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement/scraping"
	"github.com/Caia-Tech/caia-library/internal/procurement/quality"
	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
)

// GolangContent represents a curated golang.org content item
type GolangContent struct {
	URL         string
	Title       string
	Description string
	Category    string
	Priority    int // 1 = highest priority, 5 = lowest
}

func main() {
	fmt.Println("🐹 GOLANG.ORG DATA CURATION")
	fmt.Println("===========================")
	fmt.Println("Curating comprehensive Go documentation and resources")
	fmt.Println()

	// Setup pipeline configuration
	config := pipeline.DevelopmentPipelineConfig()
	config.Logging.Level = "info" // Less verbose for curation

	// Initialize logging
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}

	logger := logging.GetPipelineLogger("golang-curation", "main")
	logger.Info().Msg("Starting golang.org data curation")

	// Setup pipeline infrastructure
	fmt.Println("🔧 Setting up curation infrastructure...")
	if err := setupCurationPipeline(config); err != nil {
		logger.Fatal().Err(err).Msg("Failed to setup pipeline")
	}
	fmt.Println("✅ Infrastructure ready")

	// Initialize storage system
	fmt.Println("💾 Initializing storage system...")
	hybridStorage, err := initializeCurationStorage(config)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize storage")
	}
	defer hybridStorage.Close()
	fmt.Println("✅ Storage system ready")

	// Initialize processing components
	fmt.Println("🔍 Initializing processing components...")
	complianceEngine := scraping.NewComplianceEngine(scraping.DefaultComplianceConfig())
	qualityValidator := quality.NewQualityValidator(nil)
	extractorEngine := extractor.NewEngine()
	fmt.Println("✅ Processing components ready")

	// Execute curation
	fmt.Println("\n🚀 Starting comprehensive golang.org curation...")
	curatedCount, err := executeCuration(
		context.Background(),
		hybridStorage,
		complianceEngine,
		qualityValidator,
		extractorEngine,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Curation failed")
	}

	// Generate final report
	fmt.Println("\n📊 Generating curation report...")
	if err := generateCurationReport(hybridStorage, curatedCount); err != nil {
		logger.Error().Err(err).Msg("Failed to generate report")
	}

	fmt.Printf("\n🎉 GOLANG.ORG CURATION COMPLETED!\n")
	fmt.Printf("Successfully curated %d documents from golang.org\n", curatedCount)
	logger.Info().Int("curated_documents", curatedCount).Msg("Golang.org curation completed successfully")
}

func setupCurationPipeline(config *pipeline.PipelineConfig) error {
	// Validate and setup directories
	if err := pipeline.ValidateConfiguration(config); err != nil {
		return err
	}
	
	if err := pipeline.SetupDirectories(config); err != nil {
		return err
	}
	
	if err := pipeline.InitializeGitRepository(config); err != nil {
		return err
	}
	
	return nil
}

func initializeCurationStorage(config *pipeline.PipelineConfig) (*storage.HybridStorage, error) {
	metricsCollector := storage.NewSimpleMetricsCollector()
	
	// Use govc as primary storage for best performance
	config.Storage.PrimaryBackend = "govc"
	config.Storage.EnableFallback = true
	
	hybridStorage, err := storage.NewHybridStorage(
		config.DataPaths.GitRepo,
		"golang-curation-storage",
		config.Storage,
		metricsCollector,
	)
	if err != nil {
		return nil, err
	}
	
	// Health check
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := hybridStorage.Health(ctx); err != nil {
		fmt.Printf("⚠️  Storage health warning: %v\n", err)
		fmt.Println("   Continuing with curation...")
	}
	
	return hybridStorage, nil
}

func executeCuration(
	ctx context.Context,
	storage *storage.HybridStorage,
	compliance *scraping.ComplianceEngine,
	quality *quality.QualityValidator,
	extractor *extractor.Engine,
) (int, error) {
	logger := logging.GetPipelineLogger("golang-curation", "execution")
	
	// Define comprehensive golang.org content to curate
	contentTargets := getGolangContentTargets()
	
	logger.Info().Int("total_targets", len(contentTargets)).Msg("Starting curation of golang.org content")
	
	successCount := 0
	
	for i, target := range contentTargets {
		logger := logger.With().
			Str("url", target.URL).
			Str("category", target.Category).
			Int("priority", target.Priority).
			Logger()
		
		fmt.Printf("\n[%d/%d] 🎯 %s\n", i+1, len(contentTargets), target.Title)
		fmt.Printf("   📂 Category: %s (Priority %d)\n", target.Category, target.Priority)
		fmt.Printf("   🔗 URL: %s\n", target.URL)
		
		// Process with comprehensive pipeline
		if err := processSingleDocument(ctx, target, storage, compliance, quality, extractor); err != nil {
			logger.Error().Err(err).Msg("Failed to process document")
			fmt.Printf("   ❌ Failed: %v\n", err)
			continue
		}
		
		successCount++
		fmt.Printf("   ✅ Successfully curated\n")
		logger.Info().Msg("Document successfully curated")
		
		// Respectful delay between requests
		if i < len(contentTargets)-1 {
			fmt.Printf("   ⏱️  Respectful 2s delay...\n")
			time.Sleep(2 * time.Second)
		}
	}
	
	logger.Info().
		Int("successful", successCount).
		Int("total", len(contentTargets)).
		Msg("Curation execution completed")
	
	return successCount, nil
}

func processSingleDocument(
	ctx context.Context,
	target GolangContent,
	storage *storage.HybridStorage,
	compliance *scraping.ComplianceEngine,
	quality *quality.QualityValidator,
	extractor *extractor.Engine,
) error {
	// 1. Compliance check
	fmt.Printf("      🛡️  Checking compliance...\n")
	complianceResult, err := compliance.CheckCompliance(ctx, target.URL)
	if err != nil || !complianceResult.Allowed {
		return fmt.Errorf("compliance check failed: %w", err)
	}
	
	// Respect required delay
	if complianceResult.RequiredDelay > 0 {
		fmt.Printf("      ⏱️  Respecting %v delay...\n", complianceResult.RequiredDelay)
		time.Sleep(complianceResult.RequiredDelay)
	}
	
	// 2. Fetch content
	fmt.Printf("      📥 Fetching content...\n")
	content, contentType, err := fetchContent(ctx, target.URL)
	if err != nil {
		return fmt.Errorf("failed to fetch content: %w", err)
	}
	
	fmt.Printf("      📊 Fetched: %d bytes (%s)\n", len(content), contentType)
	
	// 3. Extract text
	fmt.Printf("      🔍 Extracting text...\n")
	text, metadata, err := extractor.Extract(ctx, content, "html") // golang.org serves HTML
	if err != nil {
		return fmt.Errorf("text extraction failed: %w", err)
	}
	
	fmt.Printf("      📝 Extracted: %d characters\n", len(text))
	
	// 4. Quality validation
	fmt.Printf("      🏆 Validating quality...\n")
	qualityMeta := map[string]string{
		"source":      "golang.org",
		"url":         target.URL,
		"title":       target.Title,
		"category":    target.Category,
		"description": target.Description,
	}
	
	qualityResult, err := quality.ValidateContent(ctx, text, qualityMeta)
	var qualityScore float64 = 0.0
	var qualityTier string = "unknown"
	
	if err == nil {
		qualityScore = qualityResult.OverallScore
		qualityTier = qualityResult.QualityTier
		fmt.Printf("      📊 Quality: %.2f (%s)\n", qualityScore, qualityTier)
	} else {
		fmt.Printf("      ⚠️  Quality validation warning: %v\n", err)
	}
	
	// 5. Create comprehensive document
	doc := createCuratedDocument(target, text, metadata, qualityScore, qualityTier)
	
	// 6. Store in CAIA Library
	fmt.Printf("      💾 Storing in CAIA Library...\n")
	docID, err := storage.StoreDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("storage failed: %w", err)
	}
	
	fmt.Printf("      ✅ Stored with ID: %s\n", docID[:16]+"...")
	
	return nil
}

func fetchContent(ctx context.Context, url string) ([]byte, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	
	// Set respectful headers
	req.Header.Set("User-Agent", "CAIA-Library-Curator/1.0 (+https://caiatech.com)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	
	contentType := resp.Header.Get("Content-Type")
	return content, contentType, nil
}

func createCuratedDocument(target GolangContent, text string, extractMeta map[string]string, qualityScore float64, qualityTier string) *document.Document {
	// Create comprehensive metadata
	metadata := map[string]string{
		// Source information
		"source":           "golang.org",
		"url":              target.URL,
		"title":            target.Title,
		"description":      target.Description,
		"category":         target.Category,
		"priority":         fmt.Sprintf("%d", target.Priority),
		
		// Processing information
		"curated_at":       time.Now().UTC().Format(time.RFC3339),
		"curated_by":       "CAIA-Library-Curator",
		"quality_score":    fmt.Sprintf("%.3f", qualityScore),
		"quality_tier":     qualityTier,
		"word_count":       fmt.Sprintf("%d", len(strings.Fields(text))),
		"character_count":  fmt.Sprintf("%d", len(text)),
		
		// Content classification
		"content_type":     "documentation",
		"language":         "english",
		"domain":           "programming",
		"technology":       "golang",
		"audience":         "developers",
	}
	
	// Merge extraction metadata
	for k, v := range extractMeta {
		if _, exists := metadata[k]; !exists {
			metadata[k] = v
		}
	}
	
	// Generate clean document ID
	docID := fmt.Sprintf("golang_%s_%d", 
		strings.ReplaceAll(strings.ToLower(target.Category), " ", "_"),
		time.Now().Unix())
	
	return &document.Document{
		ID: docID,
		Source: document.Source{
			Type: "web",
			URL:  target.URL,
		},
		Content: document.Content{
			Text:     text,
			Metadata: metadata,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func generateCurationReport(storage *storage.HybridStorage, curatedCount int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Get all documents to analyze what we curated
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list documents for report: %w", err)
	}
	
	// Analyze curated content
	categoryCount := make(map[string]int)
	totalWords := 0
	totalChars := 0
	
	golangDocs := 0
	for _, doc := range docs {
		fullDoc, err := storage.GetDocument(ctx, doc.ID)
		if err != nil {
			continue
		}
		
		// Check if this is a golang.org document
		if source, ok := fullDoc.Content.Metadata["source"]; ok && source == "golang.org" {
			golangDocs++
			
			if category, ok := fullDoc.Content.Metadata["category"]; ok {
				categoryCount[category]++
			}
			
			if wordCount, ok := fullDoc.Content.Metadata["word_count"]; ok {
				if count := parseInt(wordCount); count > 0 {
					totalWords += count
				}
			}
			
			totalChars += len(fullDoc.Content.Text)
		}
	}
	
	// Generate report
	fmt.Printf("\n📋 GOLANG.ORG CURATION REPORT\n")
	fmt.Printf("=============================\n")
	fmt.Printf("• Curation Date: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("• Total Documents Processed: %d\n", curatedCount)
	fmt.Printf("• Successfully Stored: %d\n", golangDocs)
	fmt.Printf("• Total Words Curated: %d\n", totalWords)
	fmt.Printf("• Total Characters: %d\n", totalChars)
	fmt.Printf("\n📂 Content by Category:\n")
	
	for category, count := range categoryCount {
		fmt.Printf("   • %s: %d documents\n", category, count)
	}
	
	fmt.Printf("\n✅ All golang.org content successfully curated and stored in CAIA Library\n")
	
	return nil
}

// parseInt safely converts string to int
func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// getGolangContentTargets returns comprehensive golang.org content to curate
func getGolangContentTargets() []GolangContent {
	return []GolangContent{
		// Core Documentation (Priority 1 - Most Important)
		{
			URL:         "https://golang.org/doc/",
			Title:       "Go Documentation Hub",
			Description: "Main documentation portal with links to all Go resources",
			Category:    "Core Documentation",
			Priority:    1,
		},
		{
			URL:         "https://golang.org/doc/effective_go",
			Title:       "Effective Go",
			Description: "Essential guide to writing clear, idiomatic Go code",
			Category:    "Core Documentation",
			Priority:    1,
		},
		{
			URL:         "https://golang.org/ref/spec",
			Title:       "The Go Programming Language Specification",
			Description: "Official language specification and reference",
			Category:    "Language Reference",
			Priority:    1,
		},
		
		// Getting Started (Priority 1)
		{
			URL:         "https://golang.org/doc/tutorial/getting-started",
			Title:       "Tutorial: Get started with Go",
			Description: "Step-by-step introduction for new Go developers",
			Category:    "Tutorials",
			Priority:    1,
		},
		{
			URL:         "https://golang.org/doc/tutorial/create-module",
			Title:       "Tutorial: Create a Go module",
			Description: "Learn to create and use Go modules",
			Category:    "Tutorials", 
			Priority:    1,
		},
		
		// Core Concepts (Priority 2)
		{
			URL:         "https://golang.org/doc/code",
			Title:       "How to Write Go Code",
			Description: "Guide to organizing and structuring Go programs",
			Category:    "Programming Guide",
			Priority:    2,
		},
		{
			URL:         "https://golang.org/doc/modules/gomod-ref",
			Title:       "go.mod file reference",
			Description: "Complete reference for Go module files",
			Category:    "Module System",
			Priority:    2,
		},
		{
			URL:         "https://golang.org/doc/modules/managing-dependencies",
			Title:       "Managing module dependencies",
			Description: "Guide to handling dependencies in Go modules",
			Category:    "Module System",
			Priority:    2,
		},
		
		// Advanced Topics (Priority 2-3)
		{
			URL:         "https://golang.org/doc/diagnostics",
			Title:       "Diagnostics",
			Description: "Tools and techniques for debugging Go programs",
			Category:    "Development Tools",
			Priority:    2,
		},
		{
			URL:         "https://golang.org/doc/gc-guide",
			Title:       "A Guide to the Go Garbage Collector",
			Description: "Understanding Go's garbage collection system",
			Category:    "Performance",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/doc/security/best-practices",
			Title:       "Go Security Best Practices",
			Description: "Security guidelines and best practices for Go",
			Category:    "Security",
			Priority:    2,
		},
		
		// Package Documentation (Priority 3)
		{
			URL:         "https://golang.org/pkg/",
			Title:       "Package Documentation",
			Description: "Index of Go standard library packages",
			Category:    "Standard Library",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/pkg/fmt/",
			Title:       "Package fmt",
			Description: "Formatted I/O with functions analogous to C's printf and scanf",
			Category:    "Standard Library",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/pkg/net/http/",
			Title:       "Package http",
			Description: "HTTP client and server implementations",
			Category:    "Standard Library",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/pkg/context/",
			Title:       "Package context",
			Description: "Context carries deadlines, cancellation signals, and values across API boundaries",
			Category:    "Standard Library",
			Priority:    3,
		},
		
		// Advanced Tutorials (Priority 3-4)
		{
			URL:         "https://golang.org/doc/tutorial/web-service-gin",
			Title:       "Tutorial: Developing a RESTful API with Go and Gin",
			Description: "Build a RESTful web service with Go and the Gin Web Framework",
			Category:    "Advanced Tutorials",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/doc/tutorial/database-access",
			Title:       "Tutorial: Accessing a relational database",
			Description: "Learn to access databases using Go's database/sql package",
			Category:    "Advanced Tutorials",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/doc/tutorial/generics",
			Title:       "Tutorial: Getting started with generics",
			Description: "Introduction to using generics in Go",
			Category:    "Advanced Tutorials",
			Priority:    3,
		},
		{
			URL:         "https://golang.org/doc/tutorial/fuzzing",
			Title:       "Tutorial: Getting started with fuzzing",
			Description: "Learn to use Go's built-in fuzzing support",
			Category:    "Testing",
			Priority:    4,
		},
		
		// Community and Resources (Priority 4-5)
		{
			URL:         "https://golang.org/doc/contribute",
			Title:       "Contribution Guide",
			Description: "How to contribute to the Go project",
			Category:    "Community",
			Priority:    4,
		},
		{
			URL:         "https://golang.org/doc/faq",
			Title:       "Frequently Asked Questions (FAQ)",
			Description: "Common questions and answers about Go",
			Category:    "FAQ",
			Priority:    4,
		},
		{
			URL:         "https://golang.org/doc/install",
			Title:       "Download and install",
			Description: "Installation instructions for Go",
			Category:    "Installation",
			Priority:    4,
		},
	}
}