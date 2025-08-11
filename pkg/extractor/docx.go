package extractor

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/nguyenthenguyen/docx"
)

// DOCXExtractor handles DOCX file extraction
type DOCXExtractor struct{}

// Extract extracts text and metadata from DOCX content
func (d *DOCXExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	// Basic metadata
	metadata := map[string]string{
		"type": "docx",
		"size": fmt.Sprintf("%d", len(content)),
	}

	// Check if it's actually a DOCX file (ZIP-based format)
	if len(content) < 4 {
		return "", metadata, &PDFProcessingError{
			Message: "file too small to be a valid DOCX document",
		}
	}

	// DOCX files are ZIP files, check for ZIP signature
	if len(content) >= 4 && (content[0] != 0x50 || content[1] != 0x4B) {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("not a valid DOCX file - missing ZIP signature: %x", content[:4]),
		}
	}

	// Create a reader from the content  
	reader := bytes.NewReader(content)
	
	// Read DOCX document from memory
	doc, err := docx.ReadDocxFromMemory(reader, int64(len(content)))
	if err != nil {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("failed to parse DOCX: %v", err),
		}
	}

	// Extract text content
	text := doc.Editable().GetContent()
	
	// Clean up the text
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	
	// Update metadata with actual values
	metadata["text_length"] = fmt.Sprintf("%d", len(text))
	metadata["word_count"] = fmt.Sprintf("%d", len(strings.Fields(text)))
	metadata["line_count"] = fmt.Sprintf("%d", strings.Count(text, "\n")+1)
	metadata["status"] = "success"

	if text == "" {
		return "", metadata, &PDFProcessingError{
			Message: "DOCX document contains no extractable text",
		}
	}

	return text, metadata, nil
}