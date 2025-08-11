package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/internal/temporal/activities"
	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

// High-quality, ethically scrapable sources
var ethicalSources = []SourceConfig{
	// Educational & Academic Resources
	{URL: "https://en.wikipedia.org/wiki/Artificial_intelligence", Type: "html", Category: "AI/ML", Quality: "high", Description: "Comprehensive AI overview"},
	{URL: "https://en.wikipedia.org/wiki/Machine_learning", Type: "html", Category: "AI/ML", Quality: "high", Description: "Machine learning fundamentals"},
	{URL: "https://en.wikipedia.org/wiki/Deep_learning", Type: "html", Category: "AI/ML", Quality: "high", Description: "Deep learning concepts"},
	{URL: "https://en.wikipedia.org/wiki/Natural_language_processing", Type: "html", Category: "AI/ML", Quality: "high", Description: "NLP overview"},
	{URL: "https://en.wikipedia.org/wiki/Computer_vision", Type: "html", Category: "AI/ML", Quality: "high", Description: "Computer vision field"},
	
	// Programming & Technology
	{URL: "https://go.dev/doc/effective_go", Type: "html", Category: "Programming", Quality: "high", Description: "Effective Go programming guide"},
	{URL: "https://go.dev/doc/tutorial/getting-started", Type: "html", Category: "Programming", Quality: "high", Description: "Go getting started tutorial"},
	{URL: "https://golang.org/doc/", Type: "html", Category: "Programming", Quality: "high", Description: "Go documentation"},
	{URL: "https://en.wikipedia.org/wiki/Software_engineering", Type: "html", Category: "Programming", Quality: "high", Description: "Software engineering principles"},
	{URL: "https://en.wikipedia.org/wiki/Algorithm", Type: "html", Category: "Programming", Quality: "high", Description: "Algorithm fundamentals"},
	
	// Science & Mathematics
	{URL: "https://en.wikipedia.org/wiki/Mathematics", Type: "html", Category: "Mathematics", Quality: "high", Description: "Mathematics overview"},
	{URL: "https://en.wikipedia.org/wiki/Statistics", Type: "html", Category: "Mathematics", Quality: "high", Description: "Statistics fundamentals"},
	{URL: "https://en.wikipedia.org/wiki/Linear_algebra", Type: "html", Category: "Mathematics", Quality: "high", Description: "Linear algebra concepts"},
	{URL: "https://en.wikipedia.org/wiki/Calculus", Type: "html", Category: "Mathematics", Quality: "high", Description: "Calculus fundamentals"},
	{URL: "https://en.wikipedia.org/wiki/Probability", Type: "html", Category: "Mathematics", Quality: "high", Description: "Probability theory"},
	
	// Technology & Innovation
	{URL: "https://en.wikipedia.org/wiki/Technology", Type: "html", Category: "Technology", Quality: "high", Description: "Technology overview"},
	{URL: "https://en.wikipedia.org/wiki/Innovation", Type: "html", Category: "Technology", Quality: "high", Description: "Innovation concepts"},
	{URL: "https://en.wikipedia.org/wiki/Internet", Type: "html", Category: "Technology", Quality: "high", Description: "Internet fundamentals"},
	{URL: "https://en.wikipedia.org/wiki/World_Wide_Web", Type: "html", Category: "Technology", Quality: "high", Description: "Web technology"},
	{URL: "https://en.wikipedia.org/wiki/Blockchain", Type: "html", Category: "Technology", Quality: "high", Description: "Blockchain technology"},
	
	// Business & Economics  
	{URL: "https://en.wikipedia.org/wiki/Economics", Type: "html", Category: "Economics", Quality: "high", Description: "Economics fundamentals"},
	{URL: "https://en.wikipedia.org/wiki/Business", Type: "html", Category: "Business", Quality: "high", Description: "Business concepts"},
	{URL: "https://en.wikipedia.org/wiki/Entrepreneurship", Type: "html", Category: "Business", Quality: "high", Description: "Entrepreneurship principles"},
	{URL: "https://en.wikipedia.org/wiki/Management", Type: "html", Category: "Business", Quality: "high", Description: "Management theory"},
	
	// Open Source Documentation (Developer-friendly)
	{URL: "https://docs.python.org/3/tutorial/", Type: "html", Category: "Programming", Quality: "high", Description: "Python tutorial"},
	{URL: "https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide", Type: "html", Category: "Programming", Quality: "high", Description: "JavaScript guide"},
	{URL: "https://reactjs.org/docs/getting-started.html", Type: "html", Category: "Programming", Quality: "high", Description: "React documentation"},
	{URL: "https://nodejs.org/en/docs/", Type: "html", Category: "Programming", Quality: "high", Description: "Node.js documentation"},
}

type SourceConfig struct {
	URL         string
	Type        string
	Category    string
	Quality     string
	Description string
}

func main() {
	fmt.Println("🌟 ETHICAL HIGH-QUALITY DATA SCRAPING")
	fmt.Println("=====================================")
	fmt.Println("Scraping publicly available, high-quality educational content")
	fmt.Println()

	// Check if Temporal server is running
	fmt.Println("📡 Checking Temporal server connectivity...")
	temporalClient, err := client.Dial(client.Options{
		HostPort: "localhost:7233",
	})
	if err != nil {
		log.Fatalf("❌ Cannot connect to Temporal server: %v\n   Run: ./scripts/dev-temporal.sh start", err)
	}
	defer temporalClient.Close()
	fmt.Println("✅ Temporal server connected")

	// Initialize storage
	fmt.Println("💾 Initializing storage system...")
	metricsCollector := storage.NewSimpleMetricsCollector()
	hybridStorage, err := storage.NewHybridStorage(
		"./data/quality-scraping",
		"quality-scraping-repo",
		storage.DefaultHybridConfig(),
		metricsCollector,
	)
	if err != nil {
		log.Fatalf("❌ Failed to initialize storage: %v", err)
	}
	defer hybridStorage.Close()
	
	// Set up global storage for activities
	activities.SetGlobalStorage(hybridStorage, metricsCollector)
	fmt.Println("✅ Storage system initialized")

	// Create and start worker
	fmt.Println("🏭 Starting Temporal worker...")
	w := worker.New(temporalClient, "quality-scraping", worker.Options{
		MaxConcurrentActivityExecutionSize:     5, // Limit concurrent requests
		MaxConcurrentWorkflowTaskExecutionSize: 3,
	})
	
	// Register workflows and activities
	w.RegisterWorkflow(workflows.DocumentIngestionWorkflow)
	w.RegisterWorkflow(workflows.BatchIngestionWorkflow)
	w.RegisterActivity(activities.FetchDocumentActivity)
	w.RegisterActivity(activities.ExtractTextActivity)
	w.RegisterActivity(activities.GenerateEmbeddingsActivity)
	w.RegisterActivity(activities.StoreDocumentActivity)
	w.RegisterActivity(activities.IndexDocumentActivity)
	w.RegisterActivity(activities.MergeBranchActivity)

	// Start worker in background
	go func() {
		if err := w.Run(worker.InterruptCh()); err != nil {
			log.Printf("❌ Worker error: %v", err)
		}
	}()
	
	// Give worker time to start
	time.Sleep(2 * time.Second)
	fmt.Println("✅ Temporal worker started")

	// Create batch input for high-quality sources
	fmt.Printf("📚 Preparing to scrape %d high-quality sources...\n", len(ethicalSources))
	
	var documents []workflows.DocumentInput
	categoryCount := make(map[string]int)
	
	for _, source := range ethicalSources {
		documents = append(documents, workflows.DocumentInput{
			URL:  source.URL,
			Type: source.Type,
			Metadata: map[string]string{
				"category":    source.Category,
				"quality":     source.Quality,
				"description": source.Description,
				"source":      "ethical-quality-scraper",
				"scraped_at":  time.Now().Format(time.RFC3339),
			},
		})
		categoryCount[source.Category]++
	}

	// Show scraping plan
	fmt.Println("\n📊 Scraping Plan:")
	for category, count := range categoryCount {
		fmt.Printf("   • %s: %d sources\n", category, count)
	}
	
	fmt.Println("\n🔄 Starting batch ingestion workflow...")
	
	// Execute batch workflow
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	workflowID := fmt.Sprintf("quality-scraping-%d", time.Now().Unix())
	workflowRun, err := temporalClient.ExecuteWorkflow(ctx, client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: "quality-scraping",
	}, workflows.BatchIngestionWorkflow, documents)
	
	if err != nil {
		log.Fatalf("❌ Failed to start batch workflow: %v", err)
	}

	fmt.Printf("✅ Batch workflow started: %s\n", workflowRun.GetID())
	fmt.Println("   ⏳ Processing sources (this may take several minutes)...")
	
	// Wait for completion
	err = workflowRun.Get(ctx, nil)
	if err != nil {
		log.Printf("❌ Batch workflow failed: %v", err)
	} else {
		fmt.Println("✅ Batch workflow completed successfully!")
	}

	// Verify results
	fmt.Println("\n🔍 Analyzing results...")
	verifyResults(hybridStorage)

	// Generate final report
	fmt.Println("\n📊 Generating quality report...")
	generateQualityReport(hybridStorage, categoryCount)

	// Cleanup
	w.Stop()
	fmt.Println("\n🎉 HIGH-QUALITY ETHICAL SCRAPING COMPLETE!")
}

func verifyResults(storage *storage.HybridStorage) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	documents, err := storage.ListDocuments(ctx, map[string]string{})
	if err != nil {
		fmt.Printf("❌ Failed to verify results: %v\n", err)
		return
	}

	fmt.Printf("✅ Successfully stored %d documents\n", len(documents))
	
	// Analyze by category
	categoryStats := make(map[string]struct {
		Count      int
		TotalWords int
	})

	for _, doc := range documents {
		category := doc.Content.Metadata["category"]
		if category == "" {
			category = "unknown"
		}
		
		wordCount := len(doc.Content.Text) / 5 // Rough word estimate
		stats := categoryStats[category]
		stats.Count++
		stats.TotalWords += wordCount
		categoryStats[category] = stats
	}

	fmt.Println("   📈 Documents by category:")
	for category, stats := range categoryStats {
		avgWords := 0
		if stats.Count > 0 {
			avgWords = stats.TotalWords / stats.Count
		}
		fmt.Printf("   • %s: %d docs, ~%d words avg\n", category, stats.Count, avgWords)
	}

	// Check quality distribution
	qualityStats := make(map[string]int)
	for _, doc := range documents {
		quality := doc.Content.Metadata["quality"]
		if quality == "" {
			quality = "unknown"
		}
		qualityStats[quality]++
	}

	fmt.Println("   ⭐ Quality distribution:")
	for quality, count := range qualityStats {
		fmt.Printf("   • %s: %d documents\n", quality, count)
	}
}

func generateQualityReport(storage *storage.HybridStorage, expectedCategories map[string]int) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	documents, err := storage.ListDocuments(ctx, map[string]string{})
	if err != nil {
		fmt.Printf("❌ Failed to generate report: %v\n", err)
		return
	}

	totalWords := 0
	totalChars := 0
	
	for _, doc := range documents {
		totalWords += len(doc.Content.Text) / 5 // Rough word estimate
		totalChars += len(doc.Content.Text)
	}

	fmt.Printf("📋 QUALITY SCRAPING REPORT\n")
	fmt.Printf("==========================\n")
	fmt.Printf("• Total sources planned: %d\n", len(ethicalSources))
	fmt.Printf("• Documents successfully scraped: %d\n", len(documents))
	fmt.Printf("• Success rate: %.1f%%\n", float64(len(documents))/float64(len(ethicalSources))*100)
	fmt.Printf("• Total content: ~%d words, %d characters\n", totalWords, totalChars)
	fmt.Printf("• Average document size: ~%d words\n", totalWords/max(len(documents), 1))
	
	fmt.Printf("\n🎯 Content Categories Captured:\n")
	actualCategories := make(map[string]int)
	for _, doc := range documents {
		category := doc.Content.Metadata["category"]
		actualCategories[category]++
	}
	
	for category, expected := range expectedCategories {
		actual := actualCategories[category]
		fmt.Printf("• %s: %d/%d (%.1f%%)\n", category, actual, expected, float64(actual)/float64(expected)*100)
	}

	fmt.Printf("\n✅ High-quality educational content successfully collected!\n")
	fmt.Printf("🎊 Ready for LLM training and knowledge applications\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}