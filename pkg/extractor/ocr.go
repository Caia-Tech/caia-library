// +build ocr

package extractor

import (
	"context"
	"fmt"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

// OCRExtractor handles OCR text extraction from images
type OCRExtractor struct {
	Language string // Tesseract language code (e.g., "eng", "eng+fra")
	PageSegmentationMode gosseract.PageSegMode
	OCREngineMode gosseract.EngineMode
}

// NewOCRExtractor creates a new OCR extractor with default settings
func NewOCRExtractor() *OCRExtractor {
	return &OCRExtractor{
		Language: "eng", // English by default
		PageSegmentationMode: gosseract.PSM_AUTO, // Automatic page segmentation
		OCREngineMode: gosseract.OEM_LSTM_ONLY, // Use LSTM OCR engine
	}
}

// Extract extracts text from image content using OCR
func (o *OCRExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	// Basic metadata
	metadata := map[string]string{
		"type": "ocr",
		"size": fmt.Sprintf("%d", len(content)),
		"language": o.Language,
		"engine": "tesseract",
	}

	if len(content) == 0 {
		return "", metadata, &PDFProcessingError{
			Message: "no image content provided for OCR",
		}
	}

	// Create Tesseract client
	client := gosseract.NewClient()
	defer client.Close()

	// Configure OCR settings
	err := client.SetLanguage(o.Language)
	if err != nil {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("failed to set OCR language '%s': %v", o.Language, err),
		}
	}

	err = client.SetPageSegMode(o.PageSegmentationMode)
	if err != nil {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("failed to set page segmentation mode: %v", err),
		}
	}

	// Set image data from byte array
	err = client.SetImageFromBytes(content)
	if err != nil {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("failed to set OCR image data: %v", err),
		}
	}

	// Extract text using OCR
	text, err := client.Text()
	if err != nil {
		return "", metadata, &PDFProcessingError{
			Message: fmt.Sprintf("OCR text extraction failed: %v", err),
		}
	}

	// Clean up the extracted text
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	// Get confidence score if available
	confidence, err := client.GetMeanConfidence()
	if err == nil {
		metadata["confidence"] = fmt.Sprintf("%.2f", confidence)
	}

	// Update metadata with results
	metadata["text_length"] = fmt.Sprintf("%d", len(text))
	metadata["word_count"] = fmt.Sprintf("%d", len(strings.Fields(text)))
	metadata["line_count"] = fmt.Sprintf("%d", strings.Count(text, "\n")+1)
	metadata["status"] = "success"

	if text == "" {
		return "", metadata, &PDFProcessingError{
			Message: "OCR could not extract any text from the image",
		}
	}

	return text, metadata, nil
}

// ExtractFromImage is a convenience method for extracting text from image files
func ExtractTextFromImage(ctx context.Context, imageData []byte, language string) (string, map[string]string, error) {
	extractor := &OCRExtractor{
		Language: language,
		PageSegmentationMode: gosseract.PSM_AUTO,
		OCREngineMode: gosseract.OEM_LSTM_ONLY,
	}
	
	return extractor.Extract(ctx, imageData)
}