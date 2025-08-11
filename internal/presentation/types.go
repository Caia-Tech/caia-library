package presentation

import (
	"time"

	"github.com/Caia-Tech/caia-library/internal/procurement"
)

// DocumentPresenter provides methods for presenting documents to users
type DocumentPresenter interface {
	// RenderDocument formats a document for display
	RenderDocument(doc *Document, options *RenderOptions) (*RenderedDocument, error)
	
	// RenderCollection formats multiple documents
	RenderCollection(docs []*Document, options *CollectionOptions) (*RenderedCollection, error)
	
	// RenderSearch formats search results
	RenderSearch(results *SearchResults, options *SearchOptions) (*RenderedSearch, error)
	
	// ExportDocument exports a document in various formats
	ExportDocument(doc *Document, format ExportFormat) ([]byte, error)
}

// RenderOptions configures document rendering
type RenderOptions struct {
	Format          OutputFormat      `json:"format"`
	IncludeMetadata bool              `json:"include_metadata"`
	IncludeQuality  bool              `json:"include_quality"`
	HighlightTerms  []string          `json:"highlight_terms"`
	MaxLength       int               `json:"max_length"`
	Theme           string            `json:"theme"`
	Locale          string            `json:"locale"`
}

// CollectionOptions configures collection rendering
type CollectionOptions struct {
	RenderOptions
	PageSize       int              `json:"page_size"`
	PageNumber     int              `json:"page_number"`
	SortBy         string           `json:"sort_by"`
	SortOrder      string           `json:"sort_order"`
	GroupBy        string           `json:"group_by"`
	ShowStatistics bool             `json:"show_statistics"`
}

// SearchOptions configures search result rendering
type SearchOptions struct {
	CollectionOptions
	ShowSnippets     bool            `json:"show_snippets"`
	SnippetLength    int             `json:"snippet_length"`
	ShowFacets       bool            `json:"show_facets"`
	HighlightMatches bool            `json:"highlight_matches"`
}

// OutputFormat represents the output format
type OutputFormat string

const (
	FormatHTML     OutputFormat = "html"
	FormatMarkdown OutputFormat = "markdown"
	FormatJSON     OutputFormat = "json"
	FormatPlain    OutputFormat = "plain"
	FormatRich     OutputFormat = "rich"
)

// ExportFormat represents export formats
type ExportFormat string

const (
	ExportPDF      ExportFormat = "pdf"
	ExportDOCX     ExportFormat = "docx"
	ExportEPUB     ExportFormat = "epub"
	ExportMarkdown ExportFormat = "markdown"
	ExportJSON     ExportFormat = "json"
	ExportXML      ExportFormat = "xml"
)

// RenderedDocument represents a rendered document
type RenderedDocument struct {
	ID              string                    `json:"id"`
	Title           string                    `json:"title"`
	Content         string                    `json:"content"`
	Format          OutputFormat              `json:"format"`
	Metadata        map[string]interface{}    `json:"metadata,omitempty"`
	QualityMetrics  *procurement.ValidationResult `json:"quality_metrics,omitempty"`
	RenderTime      time.Time                 `json:"render_time"`
	Theme           string                    `json:"theme"`
}

// RenderedCollection represents a rendered collection
type RenderedCollection struct {
	Documents      []*RenderedDocument       `json:"documents"`
	TotalCount     int                       `json:"total_count"`
	PageSize       int                       `json:"page_size"`
	PageNumber     int                       `json:"page_number"`
	Statistics     *CollectionStatistics     `json:"statistics,omitempty"`
	Groups         map[string][]*RenderedDocument `json:"groups,omitempty"`
	RenderTime     time.Time                 `json:"render_time"`
}

// RenderedSearch represents rendered search results
type RenderedSearch struct {
	Query          string                    `json:"query"`
	Results        []*SearchResult           `json:"results"`
	TotalHits      int                       `json:"total_hits"`
	PageSize       int                       `json:"page_size"`
	PageNumber     int                       `json:"page_number"`
	Facets         map[string]*Facet         `json:"facets,omitempty"`
	SearchTime     time.Duration             `json:"search_time"`
	RenderTime     time.Time                 `json:"render_time"`
}

// SearchResults contains raw search results
type SearchResults struct {
	Query      string                    `json:"query"`
	Documents  []*Document       `json:"documents"`
	Scores     map[string]float64        `json:"scores"`
	TotalHits  int                       `json:"total_hits"`
	SearchTime time.Duration             `json:"search_time"`
}

// SearchResult represents a single search result
type SearchResult struct {
	Document       *RenderedDocument         `json:"document"`
	Score          float64                   `json:"score"`
	Snippet        string                    `json:"snippet,omitempty"`
	Highlights     []string                  `json:"highlights,omitempty"`
}

// CollectionStatistics provides statistics about a collection
type CollectionStatistics struct {
	TotalDocuments  int                      `json:"total_documents"`
	AverageQuality  float64                  `json:"average_quality"`
	QualityDistribution map[string]int       `json:"quality_distribution"`
	SourceDistribution  map[string]int       `json:"source_distribution"`
	LanguageDistribution map[string]int      `json:"language_distribution"`
	DateRange          *DateRange            `json:"date_range"`
}

// DateRange represents a date range
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// Facet represents a search facet
type Facet struct {
	Name    string         `json:"name"`
	Values  []*FacetValue  `json:"values"`
}

// FacetValue represents a single facet value
type FacetValue struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// ViewTemplate represents a presentation template
type ViewTemplate struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Format      OutputFormat             `json:"format"`
	Template    string                   `json:"template"`
	Styles      map[string]string        `json:"styles"`
	Scripts     []string                 `json:"scripts"`
	Components  map[string]string        `json:"components"`
}

// InteractiveView provides interactive document viewing
type InteractiveView struct {
	Document       *RenderedDocument        `json:"document"`
	Annotations    []*Annotation            `json:"annotations"`
	Navigation     *Navigation              `json:"navigation"`
	Actions        []*Action                `json:"actions"`
	RelatedDocs    []*RenderedDocument      `json:"related_docs"`
}

// Annotation represents a document annotation
type Annotation struct {
	ID         string    `json:"id"`
	StartPos   int       `json:"start_pos"`
	EndPos     int       `json:"end_pos"`
	Type       string    `json:"type"`
	Content    string    `json:"content"`
	Author     string    `json:"author"`
	CreatedAt  time.Time `json:"created_at"`
}

// Navigation provides document navigation
type Navigation struct {
	TableOfContents []*TOCEntry      `json:"table_of_contents"`
	Previous        *string          `json:"previous,omitempty"`
	Next            *string          `json:"next,omitempty"`
	Breadcrumbs     []string         `json:"breadcrumbs"`
}

// TOCEntry represents a table of contents entry
type TOCEntry struct {
	Title    string      `json:"title"`
	Level    int         `json:"level"`
	Position int         `json:"position"`
	Children []*TOCEntry `json:"children,omitempty"`
}

// Action represents an available action
type Action struct {
	Name        string            `json:"name"`
	Label       string            `json:"label"`
	Icon        string            `json:"icon"`
	Enabled     bool              `json:"enabled"`
	Parameters  map[string]string `json:"parameters"`
}