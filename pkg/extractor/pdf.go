package extractor

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFProcessingError represents a non-retryable PDF processing error
type PDFProcessingError struct {
	Message string
}

func (e *PDFProcessingError) Error() string {
	return e.Message
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// PDFExtractor handles PDF file extraction
type PDFExtractor struct {
	EnableOCR bool
	MaxPages  int
}

// Extract extracts text and metadata from PDF content
func (p *PDFExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	// Basic metadata
	metadata := map[string]string{
		"type": "pdf",
		"size": fmt.Sprintf("%d", len(content)),
	}
	
	// Check if it's actually a PDF
	if len(content) < 4 || string(content[:4]) != "%PDF" {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("not a valid PDF file - content starts with: %q", string(content[:min(20, len(content))])),
		}
	}
	
	// Extract text using pdf library
	reader := bytes.NewReader(content)
	doc, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("failed to parse PDF: %v", err),
		}
	}

	var textBuilder strings.Builder
	var pageCount int
	
	// Extract text from each page
	for i := 1; i <= doc.NumPage(); i++ {
		pageCount++
		
		// Stop if we hit max pages limit
		if p.MaxPages > 0 && pageCount > p.MaxPages {
			break
		}
		
		page := doc.Page(i)
		if page.V.IsNull() {
			continue
		}
		
		pageText, err := page.GetPlainText(nil)
		if err != nil {
			// Log error but continue with other pages
			continue
		}
		
		textBuilder.WriteString(pageText)
		textBuilder.WriteString("\n\n")
	}

	text := strings.TrimSpace(textBuilder.String())
	
	// If no text was extracted and OCR is enabled, try OCR
	if text == "" && p.EnableOCR {
		metadata["ocr_attempted"] = "true"
		
		// For now, we can't easily extract images from PDF pages with this library
		// In a production system, you'd use a more advanced PDF library like pdfcpu
		// or convert PDF pages to images first, then OCR them
		metadata["ocr_note"] = "OCR enabled but requires image extraction from PDF"
		
		return "", metadata, &PDFProcessingError{
			Message: "PDF contains no extractable text and OCR image extraction not yet implemented",
		}
	}
	
	// Update metadata with actual values
	metadata["pages"] = fmt.Sprintf("%d", doc.NumPage())
	metadata["extracted_pages"] = fmt.Sprintf("%d", pageCount)
	metadata["text_length"] = fmt.Sprintf("%d", len(text))
	metadata["ocr_enabled"] = fmt.Sprintf("%t", p.EnableOCR)
	metadata["status"] = "success"
	
	if text == "" {
		return "", metadata, &PDFProcessingError{
			Message: "PDF contains no extractable text",
		}
	}
	
	return text, metadata, nil
}