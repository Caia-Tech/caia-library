package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/extractor"
)

func main() {
	fmt.Println("🚀 CAIA LIBRARY FILE UPLOAD & PROCESSING DEMO")
	fmt.Println("=============================================")
	fmt.Println("Demonstrating new PDF/DOCX/OCR capabilities")
	fmt.Println()

	// Test the extractor engine with the new capabilities
	engine := extractor.NewEngine()

	// Test 1: Download and process a real PDF
	fmt.Println("1. 📄 TESTING PDF PROCESSING")
	testPDFProcessing(engine)

	// Test 2: Show DOCX support
	fmt.Println("\n2. 📝 TESTING DOCX SUPPORT")
	testDOCXSupport(engine)

	// Test 3: Show OCR support
	fmt.Println("\n3. 🖼️  TESTING OCR SUPPORT")
	testOCRSupport(engine)

	// Summary
	fmt.Println("\n4. 📋 SUPPORTED FILE TYPES")
	printSupportedTypes()

	fmt.Println("\n✅ DEMO COMPLETED!")
	fmt.Println("\n🔧 NEXT STEPS:")
	fmt.Println("• Start the CAIA Library server")
	fmt.Println("• Use POST /upload endpoint with multipart form data")
	fmt.Println("• Upload PDF, DOCX, or image files for processing")
	fmt.Println("• OCR requires Tesseract installation")
	fmt.Println("• Files are processed through Temporal workflows")
	fmt.Println("• Extracted text is stored in govc storage system")
}

func testPDFProcessing(engine *extractor.Engine) {
	// Download a test PDF
	pdfURL := "https://www.w3.org/WAI/ER/tests/xhtml/testfiles/resources/pdf/dummy.pdf"
	fmt.Printf("   📥 Downloading test PDF: %s\n", pdfURL)
	
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := http.Get(pdfURL)
	if err != nil {
		fmt.Printf("   ❌ Failed to download PDF: %v\n", err)
		return
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("   ❌ Failed to read PDF: %v\n", err)
		return
	}

	fmt.Printf("   📊 Downloaded: %d bytes\n", len(content))

	// Extract text
	text, metadata, err := engine.Extract(ctx, content, "pdf")
	if err != nil {
		fmt.Printf("   ❌ PDF extraction failed: %v\n", err)
		return
	}

	// Display results
	fmt.Printf("   ✅ PDF processed successfully\n")
	fmt.Printf("   📄 Pages: %s\n", metadata["pages"])
	fmt.Printf("   📝 Text length: %s characters\n", metadata["text_length"])
	fmt.Printf("   🎯 Status: %s\n", metadata["status"])
	fmt.Printf("   📖 Preview: %.100s...\n", text)
}

func testDOCXSupport(engine *extractor.Engine) {
	fmt.Printf("   📝 DOCX extractor is available\n")
	fmt.Printf("   🔧 Ready to process .docx and .doc files\n")
	fmt.Printf("   📊 Will extract text, word count, and metadata\n")
	fmt.Printf("   💾 DOCX files are ZIP-based Office documents\n")
	
	// Show what happens with empty content
	ctx := context.Background()
	_, metadata, err := engine.Extract(ctx, []byte{}, "docx")
	if err != nil {
		fmt.Printf("   ⚠️  Example error (expected): %v\n", err)
	}
	if len(metadata) > 0 {
		fmt.Printf("   📊 Metadata structure: %+v\n", metadata)
	}
}

func testOCRSupport(engine *extractor.Engine) {
	fmt.Printf("   🖼️  OCR extractor is available for images\n")
	fmt.Printf("   📸 Supported: PNG, JPG, JPEG, TIFF, BMP, GIF\n")
	fmt.Printf("   🧠 Uses Tesseract OCR engine\n")
	fmt.Printf("   🌐 Default language: English (configurable)\n")
	
	// Check OCR availability
	fmt.Printf("   🔍 Testing OCR availability...\n")
	
	ctx := context.Background()
	_, _, err := engine.Extract(ctx, []byte{}, "png")
	if err != nil {
		fmt.Printf("   ⚠️  OCR test (expected failure): %v\n", err)
		if containsTesseractError(err.Error()) {
			fmt.Printf("   💡 Install Tesseract to enable OCR:\n")
			fmt.Printf("      macOS: brew install tesseract\n")
			fmt.Printf("      Ubuntu: sudo apt install tesseract-ocr\n")
			fmt.Printf("      Windows: Download from tesseract-ocr repo\n")
		}
	}
	fmt.Printf("   📊 OCR metadata includes: confidence, language, engine\n")
}

func printSupportedTypes() {
	fmt.Printf("   📋 File Types Now Supported:\n")
	fmt.Printf("   • 📄 PDF - text extraction (OCR planned for image-based)\n")
	fmt.Printf("   • 📝 DOCX/DOC - Word document processing\n")
	fmt.Printf("   • 🖼️  PNG/JPG/JPEG - OCR text extraction\n")
	fmt.Printf("   • 🖼️  TIFF/BMP/GIF - OCR text extraction\n")
	fmt.Printf("   • 📄 TXT/HTML - plain text processing\n")
	fmt.Printf("\n   🔧 Upload Endpoint: POST /upload\n")
	fmt.Printf("   📋 Form Fields:\n")
	fmt.Printf("      • file: the document file (required)\n")
	fmt.Printf("      • title: document title (optional)\n")
	fmt.Printf("      • description: document description (optional)\n")
	fmt.Printf("      • author: document author (optional)\n")
	fmt.Printf("      • tags: comma-separated tags (optional)\n")
}

func containsTesseractError(errorStr string) bool {
	tesseractKeywords := []string{"tesseract", "tess", "ocr", "language"}
	for _, keyword := range tesseractKeywords {
		if contains(errorStr, keyword) {
			return true
		}
	}
	return false
}

func contains(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || len(str) > len(substr) && (str[:len(substr)] == substr || str[len(str)-len(substr):] == substr || containsInner(str, substr)))
}

func containsInner(str, substr string) bool {
	for i := 1; i <= len(str)-len(substr); i++ {
		if str[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}