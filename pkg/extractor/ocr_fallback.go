// +build !ocr

package extractor

import (
	"context"
	"fmt"
)

// OCRExtractor handles OCR text extraction from images (fallback when Tesseract is not available)
type OCRExtractor struct {
	Language string
}

// NewOCRExtractor creates a new OCR extractor with default settings (fallback version)
func NewOCRExtractor() *OCRExtractor {
	return &OCRExtractor{
		Language: "eng",
	}
}

// Extract returns an error indicating OCR is not available
func (o *OCRExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	metadata := map[string]string{
		"type": "ocr",
		"size": fmt.Sprintf("%d", len(content)),
		"language": o.Language,
		"engine": "tesseract_not_available",
		"status": "error",
	}

	return "", metadata, &PDFProcessingError{
		Message: "OCR functionality requires Tesseract to be installed. Install with: brew install tesseract (macOS) or sudo apt install tesseract-ocr (Ubuntu)",
	}
}

// ExtractFromImage is a convenience method (fallback version)
func ExtractTextFromImage(ctx context.Context, imageData []byte, language string) (string, map[string]string, error) {
	extractor := &OCRExtractor{Language: language}
	return extractor.Extract(ctx, imageData)
}