package extractor

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"golang.org/x/net/html"
)

// ImprovedHTMLExtractor uses proper HTML parsing
type ImprovedHTMLExtractor struct{}

func NewImprovedHTMLExtractor() *ImprovedHTMLExtractor {
	return &ImprovedHTMLExtractor{}
}

func (h *ImprovedHTMLExtractor) Extract(ctx context.Context, content []byte) (string, map[string]string, error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	// Extract text content
	var textBuilder strings.Builder
	var title string
	extractText(doc, &textBuilder, &title)

	text := cleanupText(textBuilder.String())

	metadata := map[string]string{
		"type":       "html",
		"characters": fmt.Sprintf("%d", len(text)),
		"title":      title,
	}

	return text, metadata, nil
}

func extractText(n *html.Node, w io.Writer, title *string) {
	// Skip script, style, noscript, and nav elements
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript", "nav", "header", "footer", "aside":
			return
		case "title":
			if *title == "" && n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				*title = strings.TrimSpace(n.FirstChild.Data)
			}
			return
		}
	}

	// Extract text nodes
	if n.Type == html.TextNode {
		text := strings.TrimSpace(n.Data)
		if text != "" {
			// Check if parent is a block element that should add newlines
			if n.Parent != nil && isBlockElement(n.Parent.Data) {
				fmt.Fprintf(w, "\n%s\n", text)
			} else {
				fmt.Fprintf(w, " %s ", text)
			}
		}
	}

	// Recursively process child nodes
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractText(c, w, title)
	}
}

func isBlockElement(tag string) bool {
	blockElements := map[string]bool{
		"p": true, "div": true, "h1": true, "h2": true, "h3": true,
		"h4": true, "h5": true, "h6": true, "li": true, "blockquote": true,
		"article": true, "section": true, "main": true, "pre": true,
		"td": true, "th": true, "dt": true, "dd": true,
	}
	return blockElements[tag]
}

func cleanupText(text string) string {
	// Remove excessive whitespace
	lines := strings.Split(text, "\n")
	var cleaned []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Collapse multiple spaces
			line = strings.Join(strings.Fields(line), " ")
			cleaned = append(cleaned, line)
		}
	}

	// Join with proper spacing
	result := strings.Join(cleaned, "\n\n")

	// Remove common noise patterns
	noisePatterns := []string{
		"Toggle", "subsection",
		"Create account", "Log in",
		"Main page", "Contents",
		"Current events", "Random article",
		"About Wikipedia", "Contact us",
		"Donate",
	}

	for _, pattern := range noisePatterns {
		result = strings.ReplaceAll(result, pattern+"\n\n", "")
		result = strings.ReplaceAll(result, "\n\n"+pattern, "")
	}

	return strings.TrimSpace(result)
}