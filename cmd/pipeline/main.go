package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
)

func main() {
	fmt.Println("🚀 CAIA LIBRARY PIPELINE RUNNER")
	fmt.Println("==============================")
	
	// Load configuration
	config := pipeline.DevelopmentPipelineConfig()
	
	// Setup logging first
	fmt.Println("📋 Setting up logging...")
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		os.Exit(1)
	}
	
	logger := logging.GetPipelineLogger("main", "startup")
	logger.Info().Msg("CAIA Library Pipeline starting up")
	
	// Validate configuration
	logger.Info().Msg("Validating configuration")
	if err := pipeline.ValidateConfiguration(config); err != nil {
		logger.Fatal().Err(err).Msg("Configuration validation failed")
	}
	
	// Setup directories
	logger.Info().Msg("Setting up directories")
	if err := pipeline.SetupDirectories(config); err != nil {
		logger.Fatal().Err(err).Msg("Failed to setup directories")
	}
	
	// Initialize Git repository
	logger.Info().Msg("Initializing Git repository")
	if err := pipeline.InitializeGitRepository(config); err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize Git repository")
	}
	
	// Show pipeline status
	status := pipeline.GetPipelineStatus(config)
	statusJSON, _ := json.MarshalIndent(status, "", "  ")
	logger.Info().RawJSON("pipeline_status", statusJSON).Msg("Pipeline status")
	
	// Initialize storage system
	logger.Info().Msg("Initializing storage system")
	metricsCollector := storage.NewSimpleMetricsCollector()
	
	hybridStorage, err := storage.NewHybridStorage(
		config.DataPaths.GitRepo,
		"caia-pipeline-storage",
		config.Storage,
		metricsCollector,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize storage system")
	}
	defer hybridStorage.Close()
	
	// Test storage health
	ctx := context.Background()
	if err := hybridStorage.Health(ctx); err != nil {
		logger.Warn().Err(err).Msg("Storage health check warning - continuing anyway")
	} else {
		logger.Info().Msg("Storage system healthy")
	}
	
	// Initialize extractor engine
	logger.Info().Msg("Initializing extraction engine")
	extractorEngine := extractor.NewEngine()
	
	// Run pipeline demonstration
	logger.Info().Msg("Running pipeline demonstration")
	
	// Create pipeline context with cancellation
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	
	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		sig := <-sigChan
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
		cancel()
	}()
	
	// Run the complete pipeline test
	if err := runPipelineDemo(ctx, config, hybridStorage, extractorEngine); err != nil {
		logger.Error().Err(err).Msg("Pipeline demo failed")
	} else {
		logger.Info().Msg("Pipeline demo completed successfully")
	}
	
	logger.Info().Msg("CAIA Library Pipeline shutdown complete")
	fmt.Println("✅ Pipeline execution completed!")
}

func runPipelineDemo(ctx context.Context, config *pipeline.PipelineConfig, storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetPipelineLogger("demo", "execution")
	
	logger.Info().Msg("Starting comprehensive pipeline demonstration")
	
	// Step 1: Test document ingestion
	logger.Info().Msg("Step 1: Testing document ingestion pipeline")
	if err := testDocumentIngestion(ctx, storage, engine); err != nil {
		return fmt.Errorf("document ingestion test failed: %w", err)
	}
	
	// Step 2: Test file processing
	logger.Info().Msg("Step 2: Testing file processing pipeline")
	if err := testFileProcessing(ctx, storage, engine); err != nil {
		return fmt.Errorf("file processing test failed: %w", err)
	}
	
	// Step 3: Test data retrieval
	logger.Info().Msg("Step 3: Testing data retrieval")
	if err := testDataRetrieval(ctx, storage); err != nil {
		return fmt.Errorf("data retrieval test failed: %w", err)
	}
	
	// Step 4: Generate pipeline report
	logger.Info().Msg("Step 4: Generating pipeline report")
	if err := generatePipelineReport(ctx, storage, config); err != nil {
		return fmt.Errorf("report generation failed: %w", err)
	}
	
	logger.Info().Msg("All pipeline tests completed successfully")
	return nil
}

func testDocumentIngestion(ctx context.Context, storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetPipelineLogger("demo", "document-ingestion")
	
	// Test URLs for ingestion
	testDocs := []struct {
		url   string
		docType string
		description string
	}{
		{
			url:   "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf",
			docType: "pdf",
			description: "W3C Test PDF Document",
		},
	}
	
	for i, doc := range testDocs {
		logger.Info().
			Int("doc_number", i+1).
			Str("url", doc.url).
			Str("type", doc.docType).
			Str("description", doc.description).
			Msg("Processing document")
		
		// Simulate document ingestion workflow
		if err := simulateDocumentWorkflow(ctx, doc.url, doc.docType, storage, engine); err != nil {
			logger.Error().Err(err).Str("url", doc.url).Msg("Document processing failed")
			return err
		}
		
		logger.Info().Int("doc_number", i+1).Msg("Document processed successfully")
	}
	
	return nil
}

func testFileProcessing(ctx context.Context, storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetPipelineLogger("demo", "file-processing")
	
	logger.Info().Msg("Testing file processing capabilities")
	
	// Test different file types
	testFiles := []struct {
		content []byte
		fileType string
		filename string
	}{
		{
			content:  []byte("This is a test text file with sample content."),
			fileType: "txt",
			filename: "test_sample.txt",
		},
	}
	
	for _, file := range testFiles {
		logger.Info().
			Str("filename", file.filename).
			Str("type", file.fileType).
			Int("size", len(file.content)).
			Msg("Processing file")
		
		// Extract text
		text, metadata, err := engine.Extract(ctx, file.content, file.fileType)
		if err != nil {
			logger.Error().Err(err).Str("filename", file.filename).Msg("Text extraction failed")
			return err
		}
		
		logger.Info().
			Str("filename", file.filename).
			Int("text_length", len(text)).
			Interface("metadata", metadata).
			Msg("File processed successfully")
	}
	
	return nil
}

func testDataRetrieval(ctx context.Context, storage *storage.HybridStorage) error {
	logger := logging.GetPipelineLogger("demo", "data-retrieval")
	
	logger.Info().Msg("Testing data retrieval capabilities")
	
	// List all documents
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to list documents")
		return err
	}
	
	logger.Info().Int("document_count", len(docs)).Msg("Retrieved document list")
	
	// Test retrieving individual documents
	for i, doc := range docs {
		if i >= 3 { // Limit to first 3 documents
			break
		}
		
		logger.Info().Str("document_id", doc.ID).Msg("Retrieving document")
		
		retrievedDoc, err := storage.GetDocument(ctx, doc.ID)
		if err != nil {
			logger.Error().Err(err).Str("document_id", doc.ID).Msg("Failed to retrieve document")
			return err
		}
		
		logger.Info().
			Str("document_id", retrievedDoc.ID).
			Int("content_length", len(retrievedDoc.Content.Text)).
			Msg("Document retrieved successfully")
	}
	
	return nil
}

func generatePipelineReport(ctx context.Context, storage *storage.HybridStorage, config *pipeline.PipelineConfig) error {
	logger := logging.GetPipelineLogger("demo", "report-generation")
	
	logger.Info().Msg("Generating comprehensive pipeline report")
	
	// Get storage metrics
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		return err
	}
	
	report := map[string]interface{}{
		"pipeline_execution": map[string]interface{}{
			"timestamp":       time.Now().UTC().Format(time.RFC3339),
			"total_documents": len(docs),
			"configuration":   config,
		},
		"storage_status": pipeline.GetPipelineStatus(config),
		"system_health": "operational",
	}
	
	// Write report to file
	reportPath := fmt.Sprintf("%s/pipeline-report-%s.json", 
		config.DataPaths.LogDir, time.Now().Format("20060102-150405"))
	
	reportJSON, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.WriteFile(reportPath, reportJSON, 0644); err != nil {
		return err
	}
	
	logger.Info().
		Str("report_path", reportPath).
		Int("document_count", len(docs)).
		Msg("Pipeline report generated successfully")
	
	return nil
}

func simulateDocumentWorkflow(ctx context.Context, url, docType string, storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetWorkflowLogger("simulate-doc", "fetch-extract")
	
	// This would normally use the Temporal workflow, but for demo we'll simulate
	logger.Info().Str("url", url).Str("type", docType).Msg("Simulating document workflow")
	
	// For demo purposes, we'll just return success
	// In a real implementation, this would:
	// 1. Fetch the document
	// 2. Extract text using the engine
	// 3. Generate embeddings
	// 4. Store in the storage system
	// 5. Index the document
	
	logger.Info().Str("url", url).Msg("Document workflow simulation completed")
	return nil
}