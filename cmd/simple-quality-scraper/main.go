package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/activities"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// High-quality, ethically scrapable sources (smaller set for testing)
var qualitySources = []SourceConfig{
	// Educational & Academic Resources
	{URL: "https://en.wikipedia.org/wiki/Artificial_intelligence", Category: "AI/ML", Description: "Comprehensive AI overview"},
	{URL: "https://en.wikipedia.org/wiki/Machine_learning", Category: "AI/ML", Description: "Machine learning fundamentals"},
	{URL: "https://en.wikipedia.org/wiki/Deep_learning", Category: "AI/ML", Description: "Deep learning concepts"},
	
	// Programming & Technology  
	{URL: "https://go.dev/doc/effective_go", Category: "Programming", Description: "Effective Go programming guide"},
	{URL: "https://golang.org/doc/", Category: "Programming", Description: "Go documentation"},
	{URL: "https://en.wikipedia.org/wiki/Software_engineering", Category: "Programming", Description: "Software engineering principles"},
	
	// Science & Mathematics
	{URL: "https://en.wikipedia.org/wiki/Mathematics", Category: "Mathematics", Description: "Mathematics overview"},
	{URL: "https://en.wikipedia.org/wiki/Statistics", Category: "Mathematics", Description: "Statistics fundamentals"},
	
	// Technology & Innovation
	{URL: "https://en.wikipedia.org/wiki/Technology", Category: "Technology", Description: "Technology overview"},
	{URL: "https://en.wikipedia.org/wiki/Innovation", Category: "Technology", Description: "Innovation concepts"},
}

type SourceConfig struct {
	URL         string
	Category    string
	Description string
}

func main() {
	fmt.Println("🌟 SIMPLE HIGH-QUALITY DATA SCRAPING")
	fmt.Println("====================================")
	fmt.Printf("Scraping %d high-quality educational sources sequentially\n", len(qualitySources))
	fmt.Println()

	// Connect to Temporal
	temporalClient, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		fmt.Printf("❌ Cannot connect to Temporal: %v\n", err)
		fmt.Println("   Run: ./scripts/dev-temporal.sh start")
		return
	}
	defer temporalClient.Close()
	fmt.Println("✅ Temporal server connected")

	// Initialize storage
	metricsCollector := storage.NewSimpleMetricsCollector()
	hybridStorage, err := storage.NewHybridStorage(
		"./data/quality-scraping",
		"simple-quality-repo",
		storage.DefaultHybridConfig(),
		metricsCollector,
	)
	if err != nil {
		fmt.Printf("❌ Storage init failed: %v\n", err)
		return
	}
	defer hybridStorage.Close()
	
	activities.SetGlobalStorage(hybridStorage, metricsCollector)
	fmt.Println("✅ Storage system ready")

	// Start worker  
	w := worker.New(temporalClient, "simple-quality", worker.Options{
		MaxConcurrentActivityExecutionSize: 3,
	})
	
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	w.RegisterActivity(activities.FetchDocumentActivity)
	w.RegisterActivity(activities.ExtractTextActivity)
	w.RegisterActivity(activities.GenerateEmbeddingsActivity)
	w.RegisterActivity(activities.StoreDocumentActivity)
	w.RegisterActivity(activities.IndexDocumentActivity)
	w.RegisterActivity(activities.MergeBranchActivity)

	go func() {
		w.Run(worker.InterruptCh())
	}()
	
	time.Sleep(1 * time.Second)
	fmt.Println("✅ Worker started")

	// Process sources sequentially
	fmt.Printf("\n🔄 Processing %d sources...\n", len(qualitySources))
	
	successCount := 0
	categoryCount := make(map[string]int)
	
	for i, source := range qualitySources {
		fmt.Printf("\n[%d/%d] %s\n", i+1, len(qualitySources), source.Description)
		fmt.Printf("        URL: %s\n", source.URL)
		
		// Create workflow input
		input := workflows.DocumentInput{
			URL:  source.URL,
			Type: "html",
			Metadata: map[string]string{
				"category":    source.Category,
				"description": source.Description,
				"source":      "simple-quality-scraper",
				"scraped_at":  time.Now().Format(time.RFC3339),
			},
		}

		// Execute workflow
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		
		workflowID := fmt.Sprintf("simple-quality-%d-%d", i, time.Now().Unix())
		workflowRun, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: "simple-quality",
		}, workflows.DocumentIngestionWorkflow, input)
		
		if err != nil {
			fmt.Printf("        ❌ Failed to start workflow: %v\n", err)
			cancel()
			continue
		}

		// Wait for completion
		err = workflowRun.Get(ctx, nil)
		cancel()
		
		if err != nil {
			fmt.Printf("        ❌ Workflow failed: %v\n", err)
		} else {
			fmt.Printf("        ✅ Successfully processed\n")
			successCount++
			categoryCount[source.Category]++
		}
		
		// Rate limiting - be respectful
		if i < len(qualitySources)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	// Generate results
	fmt.Printf("\n🎉 SCRAPING COMPLETE\n")
	fmt.Printf("===================\n")
	fmt.Printf("• Sources processed: %d/%d (%.1f%% success)\n", 
		successCount, len(qualitySources), 
		float64(successCount)/float64(len(qualitySources))*100)
	
	fmt.Printf("\n📊 Content by Category:\n")
	for category, count := range categoryCount {
		fmt.Printf("   • %s: %d documents\n", category, count)
	}

	// Verify storage
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	documents, err := hybridStorage.ListDocuments(ctx, map[string]string{})
	if err != nil {
		fmt.Printf("❌ Failed to verify storage: %v\n", err)
	} else {
		fmt.Printf("\n💾 Storage Verification:\n")
		fmt.Printf("   • Total documents stored: %d\n", len(documents))
		
		totalWords := 0
		for _, doc := range documents {
			totalWords += len(doc.Content.Text) / 5 // Rough estimate
		}
		
		avgWords := 0
		if len(documents) > 0 {
			avgWords = totalWords / len(documents)
		}
		
		fmt.Printf("   • Total content: ~%d words\n", totalWords)
		fmt.Printf("   • Average per document: ~%d words\n", avgWords)
		
		// Show sample documents
		fmt.Printf("\n📄 Sample Documents:\n")
		for i, doc := range documents {
			if i >= 3 { // Show first 3
				break
			}
			category := doc.Content.Metadata["category"]
			wordCount := len(doc.Content.Text) / 5
			fmt.Printf("   [%d] %s (%s, ~%d words)\n", i+1, 
				doc.Content.Metadata["description"], category, wordCount)
		}
	}

	w.Stop()
	fmt.Printf("\n🚀 High-quality educational content successfully scraped!\n")
	fmt.Printf("📚 Ready for analysis and LLM training applications\n")
}