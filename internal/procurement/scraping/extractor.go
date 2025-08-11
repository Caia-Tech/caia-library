package scraping

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/rs/zerolog/log"
)

// ContentExtractor extracts and processes content from web pages
type ContentExtractor struct {
	client    *http.Client
	config    *ExtractorConfig
	selectors map[string]*SelectorSet
}

// ExtractorConfig configures content extraction behavior
type ExtractorConfig struct {
	UserAgent           string        `json:"user_agent"`
	Timeout             time.Duration `json:"timeout"`
	MaxContentSize      int64         `json:"max_content_size"`
	FollowRedirects     bool          `json:"follow_redirects"`
	MaxRedirects        int           `json:"max_redirects"`
	ExtractText         bool          `json:"extract_text"`
	ExtractMetadata     bool          `json:"extract_metadata"`
	ExtractImages       bool          `json:"extract_images"`
	ExtractLinks        bool          `json:"extract_links"`
	CleanHTML           bool          `json:"clean_html"`
	PreserveFormatting  bool          `json:"preserve_formatting"`
	Languages           []string      `json:"languages"`
	ContentTypes        []string      `json:"content_types"`
}

// SelectorSet contains CSS selectors for different content types
type SelectorSet struct {
	Title       []string `json:"title"`
	Content     []string `json:"content"`
	Author      []string `json:"author"`
	Date        []string `json:"date"`
	Tags        []string `json:"tags"`
	Description []string `json:"description"`
	Image       []string `json:"image"`
	Exclude     []string `json:"exclude"`
}

// ExtractionResult represents the result of content extraction
type ExtractionResult struct {
	Document      *document.Document `json:"document"`
	Success       bool               `json:"success"`
	Error         string             `json:"error,omitempty"`
	StatusCode    int                `json:"status_code"`
	ContentType   string             `json:"content_type"`
	ContentLength int64              `json:"content_length"`
	RedirectChain []string           `json:"redirect_chain"`
	ExtractedAt   time.Time          `json:"extracted_at"`
	ProcessingTime time.Duration     `json:"processing_time"`
}

// NewContentExtractor creates a new content extractor
func NewContentExtractor(config *ExtractorConfig) *ContentExtractor {
	if config == nil {
		config = DefaultExtractorConfig()
	}
	
	client := &http.Client{
		Timeout: config.Timeout,
	}
	
	if !config.FollowRedirects {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}
	
	ce := &ContentExtractor{
		client:    client,
		config:    config,
		selectors: make(map[string]*SelectorSet),
	}
	
	ce.setupDefaultSelectors()
	return ce
}

// DefaultExtractorConfig returns default extractor configuration
func DefaultExtractorConfig() *ExtractorConfig {
	return &ExtractorConfig{
		UserAgent:          "CAIA-Library/1.0 (+https://caia.tech/bot)",
		Timeout:            30 * time.Second,
		MaxContentSize:     10 * 1024 * 1024, // 10MB
		FollowRedirects:    true,
		MaxRedirects:       10,
		ExtractText:        true,
		ExtractMetadata:    true,
		ExtractImages:      false,
		ExtractLinks:       false,
		CleanHTML:          true,
		PreserveFormatting: true,
		Languages:          []string{"en"},
		ContentTypes:       []string{"text/html", "application/xhtml+xml"},
	}
}

// ExtractContent extracts content from a URL
func (ce *ContentExtractor) ExtractContent(ctx context.Context, targetURL string) (*ExtractionResult, error) {
	start := time.Now()
	
	log.Debug().
		Str("url", targetURL).
		Msg("Starting content extraction")
	
	result := &ExtractionResult{
		ExtractedAt:   time.Now(),
		RedirectChain: make([]string, 0),
	}
	
	// Fetch the content
	resp, err := ce.fetchContent(ctx, targetURL)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()
	
	result.StatusCode = resp.StatusCode
	result.ContentType = resp.Header.Get("Content-Type")
	result.ContentLength = resp.ContentLength
	
	// Handle redirects
	if resp.Request.URL.String() != targetURL {
		result.RedirectChain = append(result.RedirectChain, targetURL)
		result.RedirectChain = append(result.RedirectChain, resp.Request.URL.String())
	}
	
	// Check content type
	if !ce.isContentTypeSupported(result.ContentType) {
		result.Success = false
		result.Error = fmt.Sprintf("Unsupported content type: %s", result.ContentType)
		return result, fmt.Errorf("unsupported content type: %s", result.ContentType)
	}
	
	// Read content with size limit
	limitedReader := &io.LimitedReader{
		R: resp.Body,
		N: ce.config.MaxContentSize,
	}
	
	content, err := io.ReadAll(limitedReader)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}
	
	if limitedReader.N <= 0 {
		result.Success = false
		result.Error = "Content exceeds maximum size limit"
		return result, fmt.Errorf("content exceeds maximum size limit")
	}
	
	// Extract document
	doc, err := ce.extractDocument(string(content), resp.Request.URL.String(), result.ContentType)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
		return result, err
	}
	
	result.Document = doc
	result.Success = true
	result.ProcessingTime = time.Since(start)
	
	log.Debug().
		Str("url", targetURL).
		Int("status_code", result.StatusCode).
		Int64("content_length", int64(len(content))).
		Dur("processing_time", result.ProcessingTime).
		Msg("Content extraction completed")
	
	return result, nil
}

// fetchContent fetches content from a URL
func (ce *ContentExtractor) fetchContent(ctx context.Context, targetURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers
	req.Header.Set("User-Agent", ce.config.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", strings.Join(ce.config.Languages, ","))
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	
	resp, err := ce.client.Do(req)
	if err != nil {
		return nil, err
	}
	
	// Check status code
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}
	
	return resp, nil
}

// isContentTypeSupported checks if the content type is supported
func (ce *ContentExtractor) isContentTypeSupported(contentType string) bool {
	// Parse content type to get media type
	mediaType := strings.ToLower(contentType)
	if idx := strings.Index(mediaType, ";"); idx != -1 {
		mediaType = mediaType[:idx]
	}
	mediaType = strings.TrimSpace(mediaType)
	
	for _, supported := range ce.config.ContentTypes {
		if mediaType == supported {
			return true
		}
	}
	
	return false
}

// extractDocument extracts a document from HTML content
func (ce *ContentExtractor) extractDocument(content, sourceURL, contentType string) (*document.Document, error) {
	// Create document
	doc := &document.Document{
		ID: fmt.Sprintf("scraped-%d", time.Now().Unix()),
		Source: document.Source{
			Type: "web_scraping",
			URL:  sourceURL,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Extract metadata first
	metadata := make(map[string]string)
	metadata["source_url"] = sourceURL
	metadata["content_type"] = contentType
	metadata["scraped_at"] = time.Now().Format(time.RFC3339)
	
	if ce.config.ExtractMetadata {
		extractedMeta := ce.extractMetadata(content)
		for k, v := range extractedMeta {
			metadata[k] = v
		}
	}
	
	// Extract and clean text content
	text := content
	if ce.config.ExtractText {
		text = ce.extractTextContent(content)
	}
	
	if ce.config.CleanHTML {
		text = ce.cleanHTML(text)
	}
	
	// Set document content
	doc.Content = document.Content{
		Text:     text,
		Metadata: metadata,
	}
	
	return doc, nil
}

// extractMetadata extracts metadata from HTML content
func (ce *ContentExtractor) extractMetadata(content string) map[string]string {
	metadata := make(map[string]string)
	
	// Extract title
	if title := ce.extractByPattern(content, `<title[^>]*>([^<]+)</title>`); title != "" {
		metadata["title"] = ce.cleanText(title)
	}
	
	// Extract meta tags
	metaTags := ce.extractAllByPattern(content, `<meta\s+([^>]*)>`)
	for _, tag := range metaTags {
		if name := ce.extractByPattern(tag, `name=["']([^"']+)["']`); name != "" {
			if content := ce.extractByPattern(tag, `content=["']([^"']+)["']`); content != "" {
				metadata[name] = ce.cleanText(content)
			}
		}
		
		if property := ce.extractByPattern(tag, `property=["']([^"']+)["']`); property != "" {
			if content := ce.extractByPattern(tag, `content=["']([^"']+)["']`); content != "" {
				metadata[property] = ce.cleanText(content)
			}
		}
	}
	
	// Extract JSON-LD structured data
	if jsonLD := ce.extractByPattern(content, `<script[^>]*type=["']application/ld\+json["'][^>]*>([^<]+)</script>`); jsonLD != "" {
		metadata["json_ld"] = jsonLD
	}
	
	// Extract Open Graph tags
	if ogTitle := ce.extractByPattern(content, `<meta[^>]*property=["']og:title["'][^>]*content=["']([^"']+)["']`); ogTitle != "" {
		metadata["og_title"] = ce.cleanText(ogTitle)
	}
	
	if ogDesc := ce.extractByPattern(content, `<meta[^>]*property=["']og:description["'][^>]*content=["']([^"']+)["']`); ogDesc != "" {
		metadata["og_description"] = ce.cleanText(ogDesc)
	}
	
	// Extract author information
	if author := ce.extractByPattern(content, `<meta[^>]*name=["']author["'][^>]*content=["']([^"']+)["']`); author != "" {
		metadata["author"] = ce.cleanText(author)
	}
	
	// Extract publication date
	if pubDate := ce.extractByPattern(content, `<meta[^>]*name=["'](?:publish_date|publication_date)["'][^>]*content=["']([^"']+)["']`); pubDate != "" {
		metadata["publication_date"] = pubDate
	}
	
	// Extract keywords
	if keywords := ce.extractByPattern(content, `<meta[^>]*name=["']keywords["'][^>]*content=["']([^"']+)["']`); keywords != "" {
		metadata["keywords"] = keywords
	}
	
	return metadata
}

// extractTextContent extracts readable text from HTML
func (ce *ContentExtractor) extractTextContent(content string) string {
	// Remove script and style tags
	content = ce.removeByPattern(content, `<script[^>]*>.*?</script>`)
	content = ce.removeByPattern(content, `<style[^>]*>.*?</style>`)
	content = ce.removeByPattern(content, `<!--.*?-->`)
	
	// Try to extract main content
	mainContent := ce.extractMainContent(content)
	if mainContent != "" {
		content = mainContent
	}
	
	// Remove HTML tags but preserve some formatting
	if ce.config.PreserveFormatting {
		// Replace block elements with newlines
		content = regexp.MustCompile(`</(div|p|br|h[1-6]|li|blockquote)>`).ReplaceAllString(content, "\n")
		content = regexp.MustCompile(`<(div|p|br|h[1-6]|li|blockquote)[^>]*>`).ReplaceAllString(content, "\n")
	}
	
	// Remove all remaining HTML tags
	content = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(content, "")
	
	// Decode HTML entities
	content = ce.decodeHTMLEntities(content)
	
	// Clean up whitespace
	content = ce.cleanWhitespace(content)
	
	return content
}

// extractMainContent tries to extract the main content area
func (ce *ContentExtractor) extractMainContent(content string) string {
	// Try common content selectors
	contentSelectors := []string{
		`<main[^>]*>(.*?)</main>`,
		`<article[^>]*>(.*?)</article>`,
		`<div[^>]*class=["'][^"']*content[^"']*["'][^>]*>(.*?)</div>`,
		`<div[^>]*class=["'][^"']*post[^"']*["'][^>]*>(.*?)</div>`,
		`<div[^>]*class=["'][^"']*entry[^"']*["'][^>]*>(.*?)</div>`,
		`<div[^>]*id=["']content["'][^>]*>(.*?)</div>`,
	}
	
	for _, selector := range contentSelectors {
		if match := ce.extractByPattern(content, selector); match != "" {
			return match
		}
	}
	
	return ""
}

// setupDefaultSelectors sets up default CSS selectors for different sites
func (ce *ContentExtractor) setupDefaultSelectors() {
	// GitHub selectors
	ce.selectors["github.com"] = &SelectorSet{
		Title:   []string{"h1.entry-title", ".js-issue-title", "h1"},
		Content: []string{".markdown-body", ".readme", ".blob-wrapper"},
		Author:  []string{".author", ".commit-author"},
		Date:    []string{".commit-date", "time"},
		Exclude: []string{".header", ".footer", ".sidebar"},
	}
	
	// arXiv selectors
	ce.selectors["arxiv.org"] = &SelectorSet{
		Title:   []string{"h1.title", ".title"},
		Content: []string{".abstract", "blockquote"},
		Author:  []string{".authors", ".author"},
		Date:    []string{".dateline"},
	}
	
	// Documentation sites
	ce.selectors["docs.python.org"] = &SelectorSet{
		Title:   []string{"h1", ".section h1"},
		Content: []string{".body", ".document", ".section"},
		Exclude: []string{".sphinxsidebar", ".header", ".footer"},
	}
}

// Helper methods

func (ce *ContentExtractor) extractByPattern(text, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func (ce *ContentExtractor) extractAllByPattern(text, pattern string) []string {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllStringSubmatch(text, -1)
	results := make([]string, 0, len(matches))
	for _, match := range matches {
		if len(match) > 1 {
			results = append(results, match[1])
		} else if len(match) > 0 {
			results = append(results, match[0])
		}
	}
	return results
}

func (ce *ContentExtractor) removeByPattern(text, pattern string) string {
	re := regexp.MustCompile("(?s)" + pattern) // (?s) makes . match newlines
	return re.ReplaceAllString(text, "")
}

func (ce *ContentExtractor) cleanText(text string) string {
	// Trim whitespace
	text = strings.TrimSpace(text)
	
	// Decode HTML entities
	text = ce.decodeHTMLEntities(text)
	
	// Clean up extra whitespace
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	
	return text
}

func (ce *ContentExtractor) decodeHTMLEntities(text string) string {
	entities := map[string]string{
		"&amp;":    "&",
		"&lt;":     "<",
		"&gt;":     ">",
		"&quot;":   "\"",
		"&apos;":   "'",
		"&nbsp;":   " ",
		"&ndash;":  "–",
		"&mdash;":  "—",
		"&hellip;": "…",
		"&copy;":   "©",
		"&reg;":    "®",
		"&trade;":  "™",
	}
	
	for entity, replacement := range entities {
		text = strings.ReplaceAll(text, entity, replacement)
	}
	
	// Handle numeric entities (simplified)
	numericEntityPattern := regexp.MustCompile(`&#(\d+);`)
	text = numericEntityPattern.ReplaceAllStringFunc(text, func(match string) string {
		// This is a simplified implementation
		// In production, you'd properly decode the numeric entity
		return " "
	})
	
	return text
}

func (ce *ContentExtractor) cleanHTML(text string) string {
	// Remove extra newlines
	text = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(text, "\n\n")
	
	// Remove leading/trailing whitespace from each line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	
	// Join back and remove empty lines at start/end
	text = strings.Join(lines, "\n")
	text = strings.Trim(text, "\n")
	
	return text
}

func (ce *ContentExtractor) cleanWhitespace(text string) string {
	// Normalize whitespace
	text = regexp.MustCompile(`[ \t]+`).ReplaceAllString(text, " ")
	text = regexp.MustCompile(`\n[ \t]+`).ReplaceAllString(text, "\n")
	text = regexp.MustCompile(`[ \t]+\n`).ReplaceAllString(text, "\n")
	
	// Limit consecutive newlines
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")
	
	// Trim
	text = strings.TrimSpace(text)
	
	return text
}

// ExtractFromFile extracts content from a local file (for testing)
func (ce *ContentExtractor) ExtractFromFile(filePath string) (*ExtractionResult, error) {
	// This would be implemented for local file extraction
	// Useful for testing and development
	return nil, fmt.Errorf("file extraction not implemented")
}

// GetSupportedContentTypes returns the list of supported content types
func (ce *ContentExtractor) GetSupportedContentTypes() []string {
	return ce.config.ContentTypes
}

// ValidateURL checks if a URL is valid for extraction
func (ce *ContentExtractor) ValidateURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}
	
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}
	
	return nil
}