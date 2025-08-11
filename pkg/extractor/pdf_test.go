package extractor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractTextFromPDF(t *testing.T) {
	tests := []struct {
		name        string
		content     []byte
		expectError bool
	}{
		{
			name:        "empty content",
			content:     []byte{},
			expectError: true,
		},
		{
			name:        "invalid PDF content",
			content:     []byte("This is not a PDF file"),
			expectError: true,
		},
		{
			name:        "nil content",
			content:     nil,
			expectError: true,
		},
	}

	extractor := &PDFExtractor{}
	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, metadata, err := extractor.Extract(ctx, tt.content)
			
			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, text)
				// Metadata may still be returned even on error
				assert.Contains(t, metadata, "type")
				assert.Equal(t, "pdf", metadata["type"])
				
				// Check error type
				_, ok := err.(*PDFProcessingError)
				assert.True(t, ok, "Expected PDFProcessingError, got %T", err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, text)
				assert.NotEmpty(t, metadata)
			}
		})
	}
}

func TestPDFExtractor_ErrorTypes(t *testing.T) {
	// Test PDFProcessingError
	err := &PDFProcessingError{Message: "test PDF processing"}
	assert.Equal(t, "test PDF processing", err.Error())
}

func TestPDFExtractor_ContentSizeValidation(t *testing.T) {
	extractor := &PDFExtractor{}
	ctx := context.Background()

	// Test with very large content (simulated)
	largeContent := make([]byte, 1024) // 1KB
	// Fill with invalid PDF header
	copy(largeContent, []byte("Not a PDF"))

	text, metadata, err := extractor.Extract(ctx, largeContent)
	
	assert.Error(t, err)
	assert.Empty(t, text)
	// Metadata may still be returned even on error
	assert.Contains(t, metadata, "type")
	assert.Equal(t, "pdf", metadata["type"])
	
	// Should be a PDFProcessingError due to invalid format
	_, ok := err.(*PDFProcessingError)
	assert.True(t, ok)
}