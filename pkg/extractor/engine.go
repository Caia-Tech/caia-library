package extractor

import (
	"bytes"
	"context"
	"fmt"
	"strings"
)

type Engine struct {
	extractors map[string]Extractor
}

type Extractor interface {
	Extract(ctx context.Context, content []byte) (string, map[string]string, error)
}

func NewEngine() *Engine {
	return &Engine{
		extractors: map[string]Extractor{
			"text": &TextExtractor{},
			"txt":  &TextExtractor{},
			"html": NewImprovedHTMLExtractor(), // Use improved HTML parser
			"pdf":  &PDFExtractor{EnableOCR: false, MaxPages: 1000},
			"docx": &DOCXExtractor{},
			"doc":  &DOCXExtractor{}, // Treat .doc as .docx (best effort)
			"png":  NewOCRExtractor(),
			"jpg":  NewOCRExtractor(),
			"jpeg": NewOCRExtractor(),
			"tiff": NewOCRExtractor(),
			"bmp":  NewOCRExtractor(),
			"gif":  NewOCRExtractor(),
		},
	}
}

func (e *Engine) Extract(ctx context.Context, content []byte, contentType string) (string, map[string]string, error) {
	extractor, ok := e.extractors[strings.ToLower(contentType)]
	if !ok {
		// Default to text extraction
		extractor = e.extractors["text"]
	}

	return extractor.Extract(ctx, content)
}

// TextExtractor handles plain text files
type TextExtractor struct{}

func (t *TextExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	text := string(content)
	metadata := map[string]string{
		"type":       "text",
		"characters": fmt.Sprintf("%d", len(text)),
		"lines":      fmt.Sprintf("%d", bytes.Count(content, []byte("\n"))+1),
	}
	return text, metadata, nil
}

// HTMLExtractor handles HTML files
type HTMLExtractor struct{}

func (h *HTMLExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	// Simple HTML extraction - in production, use a proper HTML parser
	text := string(content)
	
	// Remove script and style tags
	text = removeHTMLTags(text, "script")
	text = removeHTMLTags(text, "style")
	
	// Remove all HTML tags
	text = stripHTMLTags(text)
	
	metadata := map[string]string{
		"type":       "html",
		"characters": fmt.Sprintf("%d", len(text)),
	}
	
	return strings.TrimSpace(text), metadata, nil
}


// Helper functions
func removeHTMLTags(html, tag string) string {
	start := fmt.Sprintf("<%s", tag)
	end := fmt.Sprintf("</%s>", tag)
	
	for {
		startIdx := strings.Index(html, start)
		if startIdx == -1 {
			break
		}
		
		endIdx := strings.Index(html[startIdx:], end)
		if endIdx == -1 {
			break
		}
		
		endIdx += startIdx + len(end)
		html = html[:startIdx] + html[endIdx:]
	}
	
	return html
}

func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	
	for _, ch := range html {
		if ch == '<' {
			inTag = true
		} else if ch == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(ch)
		}
	}
	
	return result.String()
}