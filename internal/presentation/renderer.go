package presentation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
	"github.com/rs/zerolog/log"
)

// Renderer implements the DocumentPresenter interface
type Renderer struct {
	templates map[string]*ViewTemplate
	config    *RendererConfig
}

// RendererConfig configures the renderer
type RendererConfig struct {
	DefaultFormat    OutputFormat `json:"default_format"`
	MaxContentLength int          `json:"max_content_length"`
	EnableCaching    bool         `json:"enable_caching"`
	DefaultTheme     string       `json:"default_theme"`
}

// NewRenderer creates a new document renderer
func NewRenderer(config *RendererConfig) *Renderer {
	if config == nil {
		config = &RendererConfig{
			DefaultFormat:    FormatHTML,
			MaxContentLength: 10000,
			EnableCaching:    true,
			DefaultTheme:     "light",
		}
	}

	r := &Renderer{
		templates: make(map[string]*ViewTemplate),
		config:    config,
	}

	// Initialize default templates
	r.initializeTemplates()

	return r
}

// RenderDocument renders a single document
func (r *Renderer) RenderDocument(doc *Document, options *RenderOptions) (*RenderedDocument, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	if options == nil {
		options = &RenderOptions{
			Format:          r.config.DefaultFormat,
			IncludeMetadata: true,
			IncludeQuality:  true,
			MaxLength:       r.config.MaxContentLength,
			Theme:           r.config.DefaultTheme,
		}
	}

	log.Debug().
		Str("doc_id", doc.ID).
		Str("format", string(options.Format)).
		Msg("Rendering document")

	rendered := &RenderedDocument{
		ID:         doc.ID,
		Title:      r.extractTitle(doc),
		Theme:      options.Theme,
		Format:     options.Format,
		RenderTime: time.Now(),
	}

	// Render content based on format
	content, err := r.renderContent(doc.Content, options)
	if err != nil {
		return nil, fmt.Errorf("failed to render content: %w", err)
	}
	rendered.Content = content

	// Include metadata if requested
	if options.IncludeMetadata && doc.Metadata != nil {
		rendered.Metadata = r.formatMetadata(doc.Metadata)
	}

	// Include quality metrics if available and requested
	if options.IncludeQuality && doc.Metadata != nil {
		if qualityData, ok := doc.Metadata["quality"]; ok {
			if qualityJSON, ok := qualityData.(string); ok {
				var quality procurement.ValidationResult
				if err := json.Unmarshal([]byte(qualityJSON), &quality); err == nil {
					rendered.QualityMetrics = &quality
				}
			}
		}
	}

	return rendered, nil
}

// RenderCollection renders multiple documents
func (r *Renderer) RenderCollection(docs []*Document, options *CollectionOptions) (*RenderedCollection, error) {
	if options == nil {
		options = &CollectionOptions{
			RenderOptions: RenderOptions{
				Format:          r.config.DefaultFormat,
				IncludeMetadata: false,
				MaxLength:       500, // Shorter for collections
				Theme:           r.config.DefaultTheme,
			},
			PageSize:   20,
			PageNumber: 1,
			SortBy:     "created_at",
			SortOrder:  "desc",
		}
	}

	log.Debug().
		Int("doc_count", len(docs)).
		Int("page", options.PageNumber).
		Msg("Rendering collection")

	// Calculate pagination
	totalCount := len(docs)
	startIdx := (options.PageNumber - 1) * options.PageSize
	endIdx := startIdx + options.PageSize
	if endIdx > totalCount {
		endIdx = totalCount
	}

	// Render individual documents
	renderedDocs := make([]*RenderedDocument, 0)
	if startIdx < totalCount {
		for i := startIdx; i < endIdx; i++ {
			rendered, err := r.RenderDocument(docs[i], &options.RenderOptions)
			if err != nil {
				log.Warn().Err(err).Str("doc_id", docs[i].ID).Msg("Failed to render document")
				continue
			}
			renderedDocs = append(renderedDocs, rendered)
		}
	}

	collection := &RenderedCollection{
		Documents:  renderedDocs,
		TotalCount: totalCount,
		PageSize:   options.PageSize,
		PageNumber: options.PageNumber,
		RenderTime: time.Now(),
	}

	// Add statistics if requested
	if options.ShowStatistics {
		collection.Statistics = r.calculateStatistics(docs)
	}

	// Group documents if requested
	if options.GroupBy != "" {
		collection.Groups = r.groupDocuments(renderedDocs, options.GroupBy)
	}

	return collection, nil
}

// RenderSearch renders search results
func (r *Renderer) RenderSearch(results *SearchResults, options *SearchOptions) (*RenderedSearch, error) {
	if results == nil {
		return nil, fmt.Errorf("search results are nil")
	}

	if options == nil {
		options = &SearchOptions{
			CollectionOptions: CollectionOptions{
				RenderOptions: RenderOptions{
					Format:    r.config.DefaultFormat,
					MaxLength: 200,
					Theme:     r.config.DefaultTheme,
				},
				PageSize:   20,
				PageNumber: 1,
			},
			ShowSnippets:     true,
			SnippetLength:    150,
			HighlightMatches: true,
		}
	}

	log.Debug().
		Str("query", results.Query).
		Int("hits", results.TotalHits).
		Msg("Rendering search results")

	// Calculate pagination
	startIdx := (options.PageNumber - 1) * options.PageSize
	endIdx := startIdx + options.PageSize
	if endIdx > len(results.Documents) {
		endIdx = len(results.Documents)
	}

	// Render search results
	searchResults := make([]*SearchResult, 0)
	if startIdx < len(results.Documents) {
		for i := startIdx; i < endIdx; i++ {
			doc := results.Documents[i]
			
			// Render the document
			rendered, err := r.RenderDocument(doc, &options.RenderOptions)
			if err != nil {
				log.Warn().Err(err).Str("doc_id", doc.ID).Msg("Failed to render search result")
				continue
			}

			searchResult := &SearchResult{
				Document: rendered,
				Score:    results.Scores[doc.ID],
			}

			// Generate snippet if requested
			if options.ShowSnippets {
				searchResult.Snippet = r.generateSnippet(doc.Content, results.Query, options.SnippetLength)
			}

			// Add highlights if requested
			if options.HighlightMatches {
				searchResult.Highlights = r.findHighlights(doc.Content, results.Query)
			}

			searchResults = append(searchResults, searchResult)
		}
	}

	rendered := &RenderedSearch{
		Query:      results.Query,
		Results:    searchResults,
		TotalHits:  results.TotalHits,
		PageSize:   options.PageSize,
		PageNumber: options.PageNumber,
		SearchTime: results.SearchTime,
		RenderTime: time.Now(),
	}

	// Add facets if requested
	if options.ShowFacets {
		rendered.Facets = r.generateFacets(results.Documents)
	}

	return rendered, nil
}

// ExportDocument exports a document in various formats
func (r *Renderer) ExportDocument(doc *Document, format ExportFormat) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("document is nil")
	}

	log.Debug().
		Str("doc_id", doc.ID).
		Str("format", string(format)).
		Msg("Exporting document")

	switch format {
	case ExportJSON:
		return r.exportJSON(doc)
	case ExportMarkdown:
		return r.exportMarkdown(doc)
	case ExportXML:
		return r.exportXML(doc)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", format)
	}
}

// Helper methods

func (r *Renderer) renderContent(content string, options *RenderOptions) (string, error) {
	// Truncate if needed
	if options.MaxLength > 0 && len(content) > options.MaxLength {
		content = content[:options.MaxLength] + "..."
	}

	// Highlight terms if provided
	if len(options.HighlightTerms) > 0 {
		content = r.highlightTerms(content, options.HighlightTerms)
	}

	// Format based on output format
	switch options.Format {
	case FormatHTML:
		return r.formatHTML(content), nil
	case FormatMarkdown:
		return r.formatMarkdown(content), nil
	case FormatPlain:
		return r.formatPlain(content), nil
	case FormatJSON:
		return r.formatJSON(content), nil
	default:
		return content, nil
	}
}

func (r *Renderer) formatHTML(content string) string {
	// Escape HTML and convert newlines to <br>
	escaped := html.EscapeString(content)
	paragraphs := strings.Split(escaped, "\n\n")
	
	var buf bytes.Buffer
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			buf.WriteString("<p>")
			buf.WriteString(strings.ReplaceAll(p, "\n", "<br>"))
			buf.WriteString("</p>")
		}
	}
	
	return buf.String()
}

func (r *Renderer) formatMarkdown(content string) string {
	// Basic markdown formatting
	lines := strings.Split(content, "\n")
	var formatted []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			formatted = append(formatted, "")
		} else if strings.HasPrefix(line, "#") {
			formatted = append(formatted, line)
		} else {
			formatted = append(formatted, line)
		}
	}
	
	return strings.Join(formatted, "\n")
}

func (r *Renderer) formatPlain(content string) string {
	// Remove extra whitespace and format plainly
	lines := strings.Split(content, "\n")
	var cleaned []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleaned = append(cleaned, line)
		}
	}
	
	return strings.Join(cleaned, "\n")
}

func (r *Renderer) formatJSON(content string) string {
	// Try to parse as JSON for pretty printing
	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err == nil {
		pretty, _ := json.MarshalIndent(data, "", "  ")
		return string(pretty)
	}
	// If not JSON, return as string in JSON format
	jsonStr, _ := json.Marshal(content)
	return string(jsonStr)
}

func (r *Renderer) highlightTerms(content string, terms []string) string {
	for _, term := range terms {
		// Case-insensitive highlighting
		lower := strings.ToLower(content)
		termLower := strings.ToLower(term)
		
		indices := r.findAllIndices(lower, termLower)
		offset := 0
		
		for _, idx := range indices {
			actualIdx := idx + offset
			before := content[:actualIdx]
			match := content[actualIdx : actualIdx+len(term)]
			after := content[actualIdx+len(term):]
			
			highlighted := fmt.Sprintf("**%s**", match)
			content = before + highlighted + after
			offset += 4 // Length of "**" * 2
		}
	}
	return content
}

func (r *Renderer) findAllIndices(s, substr string) []int {
	var indices []int
	start := 0
	for {
		idx := strings.Index(s[start:], substr)
		if idx == -1 {
			break
		}
		indices = append(indices, start+idx)
		start += idx + len(substr)
	}
	return indices
}

func (r *Renderer) extractTitle(doc *Document) string {
	// Try to extract title from metadata
	if doc.Metadata != nil {
		if title, ok := doc.Metadata["title"].(string); ok && title != "" {
			return title
		}
		if name, ok := doc.Metadata["name"].(string); ok && name != "" {
			return name
		}
	}

	// Extract from content (first line or heading)
	lines := strings.Split(doc.Content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			// Remove markdown heading markers
			line = strings.TrimPrefix(line, "#")
			line = strings.TrimSpace(line)
			if len(line) > 100 {
				return line[:100] + "..."
			}
			return line
		}
	}

	return fmt.Sprintf("Document %s", doc.ID)
}

func (r *Renderer) formatMetadata(metadata map[string]interface{}) map[string]interface{} {
	formatted := make(map[string]interface{})
	
	for key, value := range metadata {
		// Skip internal fields
		if strings.HasPrefix(key, "_") {
			continue
		}
		
		// Format timestamps
		if strings.Contains(key, "time") || strings.Contains(key, "date") {
			if timeStr, ok := value.(string); ok {
				if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
					formatted[key] = t.Format("2006-01-02 15:04:05")
					continue
				}
			}
		}
		
		formatted[key] = value
	}
	
	return formatted
}

func (r *Renderer) generateSnippet(content, query string, length int) string {
	// Find the position of the query in the content
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	
	pos := strings.Index(lowerContent, lowerQuery)
	if pos == -1 {
		// Query not found, return beginning of content
		if len(content) > length {
			return content[:length] + "..."
		}
		return content
	}
	
	// Calculate snippet boundaries
	start := pos - length/2
	if start < 0 {
		start = 0
	}
	
	end := pos + len(query) + length/2
	if end > len(content) {
		end = len(content)
	}
	
	snippet := content[start:end]
	
	// Add ellipsis if needed
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	
	return snippet
}

func (r *Renderer) findHighlights(content, query string) []string {
	var highlights []string
	
	// Split query into terms
	terms := strings.Fields(query)
	
	// Find sentences containing query terms
	sentences := strings.Split(content, ".")
	for _, sentence := range sentences {
		sentenceLower := strings.ToLower(sentence)
		for _, term := range terms {
			if strings.Contains(sentenceLower, strings.ToLower(term)) {
				highlights = append(highlights, strings.TrimSpace(sentence))
				break
			}
		}
		
		if len(highlights) >= 3 {
			break
		}
	}
	
	return highlights
}

func (r *Renderer) calculateStatistics(docs []*Document) *CollectionStatistics {
	stats := &CollectionStatistics{
		TotalDocuments:       len(docs),
		QualityDistribution:  make(map[string]int),
		SourceDistribution:   make(map[string]int),
		LanguageDistribution: make(map[string]int),
	}

	var totalQuality float64
	var minDate, maxDate time.Time

	for _, doc := range docs {
		if doc.Metadata != nil {
			// Quality distribution
			if quality, ok := doc.Metadata["quality_tier"].(string); ok {
				stats.QualityDistribution[quality]++
			}

			// Source distribution
			if source, ok := doc.Metadata["source"].(string); ok {
				stats.SourceDistribution[source]++
			}

			// Language distribution
			if lang, ok := doc.Metadata["language"].(string); ok {
				stats.LanguageDistribution[lang]++
			}

			// Quality score
			if score, ok := doc.Metadata["quality_score"].(float64); ok {
				totalQuality += score
			}

			// Date range
			if created, ok := doc.Metadata["created_at"].(string); ok {
				if t, err := time.Parse(time.RFC3339, created); err == nil {
					if minDate.IsZero() || t.Before(minDate) {
						minDate = t
					}
					if maxDate.IsZero() || t.After(maxDate) {
						maxDate = t
					}
				}
			}
		}
	}

	if len(docs) > 0 {
		stats.AverageQuality = totalQuality / float64(len(docs))
	}

	if !minDate.IsZero() && !maxDate.IsZero() {
		stats.DateRange = &DateRange{
			Start: minDate,
			End:   maxDate,
		}
	}

	return stats
}

func (r *Renderer) groupDocuments(docs []*RenderedDocument, groupBy string) map[string][]*RenderedDocument {
	groups := make(map[string][]*RenderedDocument)

	for _, doc := range docs {
		var groupKey string

		if doc.Metadata != nil {
			if value, ok := doc.Metadata[groupBy]; ok {
				groupKey = fmt.Sprintf("%v", value)
			}
		}

		if groupKey == "" {
			groupKey = "Other"
		}

		groups[groupKey] = append(groups[groupKey], doc)
	}

	return groups
}

func (r *Renderer) generateFacets(docs []*Document) map[string]*Facet {
	facets := make(map[string]*Facet)

	// Common facet fields
	facetFields := []string{"source", "language", "quality_tier", "domain", "type"}

	for _, field := range facetFields {
		facetValues := make(map[string]int)

		for _, doc := range docs {
			if doc.Metadata != nil {
				if value, ok := doc.Metadata[field]; ok {
					key := fmt.Sprintf("%v", value)
					facetValues[key]++
				}
			}
		}

		if len(facetValues) > 0 {
			values := make([]*FacetValue, 0)
			for val, count := range facetValues {
				values = append(values, &FacetValue{
					Value: val,
					Count: count,
				})
			}

			facets[field] = &Facet{
				Name:   field,
				Values: values,
			}
		}
	}

	return facets
}

func (r *Renderer) exportJSON(doc *Document) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

func (r *Renderer) exportMarkdown(doc *Document) ([]byte, error) {
	var buf bytes.Buffer

	// Title
	title := r.extractTitle(doc)
	buf.WriteString(fmt.Sprintf("# %s\n\n", title))

	// Metadata
	if doc.Metadata != nil {
		buf.WriteString("## Metadata\n\n")
		for key, value := range doc.Metadata {
			if !strings.HasPrefix(key, "_") {
				buf.WriteString(fmt.Sprintf("- **%s**: %v\n", key, value))
			}
		}
		buf.WriteString("\n")
	}

	// Content
	buf.WriteString("## Content\n\n")
	buf.WriteString(doc.Content)

	return buf.Bytes(), nil
}

func (r *Renderer) exportXML(doc *Document) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	buf.WriteString("<document>\n")
	buf.WriteString(fmt.Sprintf("  <id>%s</id>\n", html.EscapeString(doc.ID)))
	buf.WriteString(fmt.Sprintf("  <title>%s</title>\n", html.EscapeString(r.extractTitle(doc))))

	if doc.Metadata != nil {
		buf.WriteString("  <metadata>\n")
		for key, value := range doc.Metadata {
			if !strings.HasPrefix(key, "_") {
				buf.WriteString(fmt.Sprintf("    <%s>%v</%s>\n", key, html.EscapeString(fmt.Sprintf("%v", value)), key))
			}
		}
		buf.WriteString("  </metadata>\n")
	}

	buf.WriteString(fmt.Sprintf("  <content><![CDATA[%s]]></content>\n", doc.Content))
	buf.WriteString("</document>\n")

	return buf.Bytes(), nil
}

func (r *Renderer) initializeTemplates() {
	// Initialize default HTML template
	htmlTemplate := &ViewTemplate{
		Name:        "default-html",
		Description: "Default HTML template",
		Format:      FormatHTML,
		Template: `<!DOCTYPE html>
<html>
<head>
    <title>{{.Title}}</title>
    <style>{{.Styles}}</style>
</head>
<body>
    <div class="document">
        <h1>{{.Title}}</h1>
        <div class="content">{{.Content}}</div>
        {{if .Metadata}}
        <div class="metadata">
            {{range $key, $value := .Metadata}}
            <div class="meta-item">
                <span class="meta-key">{{$key}}:</span>
                <span class="meta-value">{{$value}}</span>
            </div>
            {{end}}
        </div>
        {{end}}
    </div>
</body>
</html>`,
		Styles: map[string]string{
			"default": `
                body { font-family: Arial, sans-serif; margin: 20px; }
                .document { max-width: 800px; margin: 0 auto; }
                h1 { color: #333; }
                .content { line-height: 1.6; }
                .metadata { margin-top: 20px; padding: 10px; background: #f5f5f5; }
                .meta-item { margin: 5px 0; }
                .meta-key { font-weight: bold; }
            `,
		},
	}

	r.templates["default-html"] = htmlTemplate
}