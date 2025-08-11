package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/Caia-Tech/caia-library/pkg/extractor"
	"github.com/Caia-Tech/caia-library/pkg/logging"
	"github.com/Caia-Tech/caia-library/pkg/pipeline"
)

func main() {
	fmt.Println("🧪 CAIA LIBRARY END-TO-END PIPELINE TEST")
	fmt.Println("========================================")
	fmt.Println("Testing complete document processing workflow")
	fmt.Println()

	// Setup configuration
	config := pipeline.DevelopmentPipelineConfig()
	
	// Initialize logging
	if err := logging.SetupLogger(config.Logging); err != nil {
		fmt.Printf("❌ Failed to setup logging: %v\n", err)
		return
	}
	
	logger := logging.GetPipelineLogger("e2e-test", "main")
	logger.Info().Msg("Starting end-to-end pipeline test")
	
	// Setup pipeline
	fmt.Println("🔧 Setting up pipeline infrastructure...")
	if err := setupPipeline(config); err != nil {
		logger.Fatal().Err(err).Msg("Pipeline setup failed")
	}
	fmt.Println("✅ Pipeline infrastructure ready")
	
	// Initialize storage
	fmt.Println("💾 Initializing storage system...")
	storage, err := initializeStorage(config)
	if err != nil {
		logger.Fatal().Err(err).Msg("Storage initialization failed")
	}
	defer storage.Close()
	fmt.Println("✅ Storage system ready")
	
	// Initialize extractor
	fmt.Println("🔍 Initializing extraction engine...")
	engine := extractor.NewEngine()
	fmt.Println("✅ Extraction engine ready")
	
	// Run comprehensive test
	fmt.Println("\n🚀 Running comprehensive pipeline test...")
	if err := runComprehensiveTest(config, storage, engine); err != nil {
		logger.Fatal().Err(err).Msg("Comprehensive test failed")
	}
	
	fmt.Println("\n📊 Generating final report...")
	if err := generateFinalReport(config, storage); err != nil {
		logger.Error().Err(err).Msg("Report generation failed")
	}
	
	fmt.Println("\n🎉 END-TO-END PIPELINE TEST COMPLETED SUCCESSFULLY!")
	logger.Info().Msg("End-to-end pipeline test completed successfully")
}

func setupPipeline(config *pipeline.PipelineConfig) error {
	// Validate configuration
	if err := pipeline.ValidateConfiguration(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	// Setup directories
	if err := pipeline.SetupDirectories(config); err != nil {
		return fmt.Errorf("directory setup failed: %w", err)
	}
	
	// Initialize Git repository
	if err := pipeline.InitializeGitRepository(config); err != nil {
		return fmt.Errorf("git initialization failed: %w", err)
	}
	
	return nil
}

func initializeStorage(config *pipeline.PipelineConfig) (*storage.HybridStorage, error) {
	metricsCollector := storage.NewSimpleMetricsCollector()
	
	hybridStorage, err := storage.NewHybridStorage(
		config.DataPaths.GitRepo,
		"e2e-test-storage",
		config.Storage,
		metricsCollector,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create hybrid storage: %w", err)
	}
	
	// Test storage health
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := hybridStorage.Health(ctx); err != nil {
		fmt.Printf("⚠️  Storage health warning: %v\n", err)
		fmt.Println("   Continuing with test anyway...")
	}
	
	return hybridStorage, nil
}

func runComprehensiveTest(config *pipeline.PipelineConfig, storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetPipelineLogger("e2e-test", "comprehensive")
	
	// Test 1: PDF Document Processing
	fmt.Println("\n1. 📄 Testing PDF Document Processing")
	if err := testPDFProcessing(storage, engine); err != nil {
		return fmt.Errorf("PDF processing test failed: %w", err)
	}
	fmt.Println("   ✅ PDF processing successful")
	
	// Test 2: Text File Processing
	fmt.Println("\n2. 📝 Testing Text File Processing")
	if err := testTextProcessing(storage, engine); err != nil {
		return fmt.Errorf("text processing test failed: %w", err)
	}
	fmt.Println("   ✅ Text processing successful")
	
	// Test 3: Storage Persistence
	fmt.Println("\n3. 💾 Testing Storage Persistence")
	if err := testStoragePersistence(storage); err != nil {
		return fmt.Errorf("storage persistence test failed: %w", err)
	}
	fmt.Println("   ✅ Storage persistence verified")
	
	// Test 4: Data Retrieval
	fmt.Println("\n4. 🔍 Testing Data Retrieval")
	if err := testDataRetrieval(storage); err != nil {
		return fmt.Errorf("data retrieval test failed: %w", err)
	}
	fmt.Println("   ✅ Data retrieval successful")
	
	logger.Info().Msg("All comprehensive tests passed")
	return nil
}

func testPDFProcessing(storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetPipelineLogger("e2e-test", "pdf-processing")
	
	// Download test PDF
	pdfURL := "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"
	fmt.Printf("   📥 Downloading: %s\n", pdfURL)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	resp, err := http.Get(pdfURL)
	if err != nil {
		return fmt.Errorf("failed to download PDF: %w", err)
	}
	defer resp.Body.Close()
	
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read PDF content: %w", err)
	}
	
	fmt.Printf("   📊 Downloaded: %d bytes\n", len(content))
	
	// Extract text
	fmt.Printf("   🔍 Extracting text...\n")
	text, metadata, err := engine.Extract(ctx, content, "pdf")
	if err != nil {
		return fmt.Errorf("text extraction failed: %w", err)
	}
	
	fmt.Printf("   📝 Extracted: %d characters\n", len(text))
	fmt.Printf("   📄 Pages: %s\n", metadata["pages"])
	
	// Create document
	doc := &document.Document{
		ID: fmt.Sprintf("test-pdf-%d", time.Now().Unix()),
		Source: document.Source{
			Type: "web",
			URL:  pdfURL,
		},
		Content: document.Content{
			Text: text,
			Metadata: metadata,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Store document
	fmt.Printf("   💾 Storing document...\n")
	docID, err := storage.StoreDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("document storage failed: %w", err)
	}
	
	fmt.Printf("   ✅ Stored with ID: %s\n", docID)
	logger.Info().Str("document_id", docID).Msg("PDF processing completed")
	
	return nil
}

func testTextProcessing(storage *storage.HybridStorage, engine *extractor.Engine) error {
	logger := logging.GetPipelineLogger("e2e-test", "text-processing")
	
	// Create test text content
	testContent := `# Sample Document for CAIA Library Testing

This is a comprehensive test document designed to validate the text processing capabilities of the CAIA Library system.

## Key Features Being Tested

1. **Text Extraction**: Validating that plain text content is properly extracted and processed.
2. **Metadata Generation**: Ensuring that appropriate metadata is generated for text documents.
3. **Storage Integration**: Verifying that processed documents are correctly stored in the storage system.
4. **Quality Assessment**: Testing the quality validation pipeline for text content.

## Content Quality Indicators

- **Word Count**: This document contains multiple paragraphs with varied content.
- **Structure**: The document uses proper markdown formatting with headers and lists.
- **Completeness**: All sections provide meaningful content for processing validation.

## Processing Validation

The CAIA Library system should:
- Extract this text content accurately
- Generate appropriate metadata (word count, character count, etc.)
- Store the document with proper indexing
- Enable retrieval and search functionality

This test document serves as a baseline for validating the complete document processing pipeline.`

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Printf("   🔍 Processing text content (%d characters)...\n", len(testContent))
	
	// Extract text (should be pass-through for plain text)
	text, metadata, err := engine.Extract(ctx, []byte(testContent), "txt")
	if err != nil {
		return fmt.Errorf("text extraction failed: %w", err)
	}
	
	fmt.Printf("   📝 Processed: %d characters\n", len(text))
	fmt.Printf("   📊 Lines: %s\n", metadata["lines"])
	
	// Create document
	doc := &document.Document{
		ID: fmt.Sprintf("test-text-%d", time.Now().Unix()),
		Source: document.Source{
			Type: "file",
			URL:  "test-document.txt",
		},
		Content: document.Content{
			Text: text,
			Metadata: map[string]string{
				"source":        "e2e-test",
				"type":          "text",
				"processing_time": time.Now().Format(time.RFC3339),
				"test_category": "text-processing",
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Merge extraction metadata
	for k, v := range metadata {
		doc.Content.Metadata[k] = v
	}
	
	// Store document
	fmt.Printf("   💾 Storing document...\n")
	docID, err := storage.StoreDocument(ctx, doc)
	if err != nil {
		return fmt.Errorf("document storage failed: %w", err)
	}
	
	fmt.Printf("   ✅ Stored with ID: %s\n", docID)
	logger.Info().Str("document_id", docID).Msg("Text processing completed")
	
	return nil
}

func testStoragePersistence(storage *storage.HybridStorage) error {
	logger := logging.GetPipelineLogger("e2e-test", "storage-persistence")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Printf("   🔍 Checking document persistence...\n")
	
	// List all documents
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}
	
	fmt.Printf("   📚 Found %d documents in storage\n", len(docs))
	
	if len(docs) == 0 {
		return fmt.Errorf("no documents found in storage - persistence may have failed")
	}
	
	// Test retrieval of each document
	for i, doc := range docs {
		fmt.Printf("   📄 Testing document %d: %s\n", i+1, doc.ID)
		
		retrieved, err := storage.GetDocument(ctx, doc.ID)
		if err != nil {
			return fmt.Errorf("failed to retrieve document %s: %w", doc.ID, err)
		}
		
		if retrieved.ID != doc.ID {
			return fmt.Errorf("document ID mismatch: expected %s, got %s", doc.ID, retrieved.ID)
		}
		
		fmt.Printf("      ✅ Retrieved successfully (%d chars)\n", len(retrieved.Content.Text))
	}
	
	logger.Info().Int("document_count", len(docs)).Msg("Storage persistence verified")
	return nil
}

func testDataRetrieval(storage *storage.HybridStorage) error {
	logger := logging.GetPipelineLogger("e2e-test", "data-retrieval")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	fmt.Printf("   🔍 Testing advanced data retrieval...\n")
	
	// List documents
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list documents: %w", err)
	}
	
	fmt.Printf("   📚 Retrieved %d documents\n", len(docs))
	
	// Test detailed retrieval
	for i, doc := range docs {
		if i >= 2 { // Limit to first 2 documents for demo
			break
		}
		
		fmt.Printf("   📄 Document %d: %s\n", i+1, doc.ID)
		
		fullDoc, err := storage.GetDocument(ctx, doc.ID)
		if err != nil {
			return fmt.Errorf("failed to get document details: %w", err)
		}
		
		fmt.Printf("      📝 Content: %d characters\n", len(fullDoc.Content.Text))
		fmt.Printf("      🏷️  Metadata: %d fields\n", len(fullDoc.Content.Metadata))
		fmt.Printf("      🕐 Created: %s\n", fullDoc.CreatedAt.Format("2006-01-02 15:04:05"))
		
		// Show content preview
		preview := fullDoc.Content.Text
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		fmt.Printf("      📖 Preview: %s\n", preview)
	}
	
	logger.Info().Int("document_count", len(docs)).Msg("Data retrieval test completed")
	return nil
}

func generateFinalReport(config *pipeline.PipelineConfig, storage *storage.HybridStorage) error {
	logger := logging.GetPipelineLogger("e2e-test", "final-report")
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Get final document count
	docs, err := storage.ListDocuments(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get document count: %w", err)
	}
	
	// Generate summary
	fmt.Printf("📊 FINAL PIPELINE TEST REPORT\n")
	fmt.Printf("============================\n")
	fmt.Printf("• Test Execution Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("• Total Documents Processed: %d\n", len(docs))
	fmt.Printf("• Storage Backend: %s\n", config.Storage.PrimaryBackend)
	fmt.Printf("• Data Directory: %s\n", config.DataPaths.DataRoot)
	fmt.Printf("• Log Level: %s\n", config.Logging.Level)
	fmt.Printf("\n✅ All pipeline components operational\n")
	fmt.Printf("✅ Document processing working\n")
	fmt.Printf("✅ Storage persistence verified\n")
	fmt.Printf("✅ Data retrieval functional\n")
	
	logger.Info().
		Int("total_documents", len(docs)).
		Str("storage_backend", config.Storage.PrimaryBackend).
		Msg("Final report generated")
	
	return nil
}