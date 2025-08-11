package gql

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/storage"
	"github.com/Caia-Tech/caia-library/pkg/document"
)

// GovcExecutor executes GQL queries against the govc backend
// This is optimized to use the document index for O(1) lookups
type GovcExecutor struct {
	backend storage.StorageBackend
}

// NewGovcExecutor creates a new query executor using govc backend
func NewGovcExecutor(backend storage.StorageBackend) *GovcExecutor {
	return &GovcExecutor{
		backend: backend,
	}
}

// Execute runs a GQL query using the govc backend for optimized performance
func (e *GovcExecutor) Execute(ctx context.Context, query string) (*Result, error) {
	// Parse the query
	parser := NewParser()
	q, err := parser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// Execute based on query type
	switch q.Type {
	case QueryDocuments:
		return e.executeDocumentQuery(ctx, q)
	case QueryAttribution:
		return e.executeAttributionQuery(ctx, q)
	case QuerySources:
		return e.executeSourcesQuery(ctx, q)
	case QueryAuthors:
		return e.executeAuthorsQuery(ctx, q)
	default:
		return nil, fmt.Errorf("unsupported query type: %s", q.Type)
	}
}

// executeDocumentQuery executes queries on documents using govc backend
func (e *GovcExecutor) executeDocumentQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// Get govc backend to access document index
	govcBackend, ok := e.backend.(*storage.GovcBackend)
	if !ok {
		// Fallback to slower method if not using govc
		return e.executeDocumentQuerySlow(ctx, q)
	}

	// Get all document IDs from the document index (O(1) operation)
	docIndex := govcBackend.GetDocumentIndex()
	allDocIDs := docIndex.GetAllDocumentIDs()

	var results []interface{}
	processedCount := 0

	// Process documents in batches for better memory usage
	for _, docID := range allDocIDs {
		// Apply limit early to avoid processing unnecessary documents
		if len(results) >= q.Limit {
			break
		}

		// Retrieve document using fast O(1) index lookup
		doc, err := e.backend.GetDocument(ctx, docID)
		if err != nil {
			continue // Skip documents we can't retrieve
		}

		processedCount++

		// Apply filters
		if !e.matchesDocumentFilters(doc, q.Filters) {
			continue
		}

		// Convert to result format
		docResult := DocumentResult{
			ID:        doc.ID,
			Source:    doc.Source.Type,
			URL:       doc.Source.URL,
			CreatedAt: doc.CreatedAt,
			UpdatedAt: doc.UpdatedAt,
			Metadata:  doc.Content.Metadata,
		}

		// Extract title from metadata or content
		if title, exists := doc.Content.Metadata["title"]; exists {
			docResult.Title = title
		}

		// Extract author information
		if author, exists := doc.Content.Metadata["author"]; exists {
			docResult.Author = author
		} else if authors, exists := doc.Content.Metadata["authors"]; exists {
			// Use first author if multiple
			if authorList := strings.Split(authors, ","); len(authorList) > 0 {
				docResult.Author = strings.TrimSpace(authorList[0])
			}
		}

		// Add commit hash from metadata if available
		if hash, exists := doc.Content.Metadata["commit_hash"]; exists {
			docResult.CommitHash = hash
		}

		results = append(results, docResult)
	}

	// Sort results if requested
	if q.OrderBy != "" {
		e.sortDocumentResults(results, q.OrderBy, q.Descending)
	}

	// Apply final limit after sorting
	if len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return &Result{
		Type:    QueryDocuments,
		Count:   len(results),
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// executeAttributionQuery tracks attribution compliance
func (e *GovcExecutor) executeAttributionQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// Get documents and analyze their attribution
	docQuery := &Query{
		Type:  QueryDocuments,
		Limit: 10000, // Large limit for analysis
	}

	docResult, err := e.executeDocumentQuery(ctx, docQuery)
	if err != nil {
		return nil, err
	}

	// Track attribution by source
	sourceStats := make(map[string]*AttributionResult)

	for _, item := range docResult.Items {
		doc, ok := item.(DocumentResult)
		if !ok {
			continue
		}

		source := doc.Source
		if source == "" {
			source = "unknown"
		}

		stats, exists := sourceStats[source]
		if !exists {
			stats = &AttributionResult{
				Source:          source,
				FirstCollected:  doc.CreatedAt,
				LastCollected:   doc.CreatedAt,
				CAIAAttribution: e.hasCAIAAttribution(doc),
			}
			sourceStats[source] = stats
		}

		stats.DocumentCount++
		if doc.CreatedAt.Before(stats.FirstCollected) {
			stats.FirstCollected = doc.CreatedAt
		}
		if doc.CreatedAt.After(stats.LastCollected) {
			stats.LastCollected = doc.CreatedAt
		}

		// Check for attribution text in metadata
		if attribution, exists := doc.Metadata["attribution"]; exists {
			stats.AttributionText = attribution
		}
	}

	// Convert to results and apply filters
	var results []interface{}
	for _, stats := range sourceStats {
		// Create metadata map for filtering
		metadata := map[string]interface{}{
			"source":           stats.Source,
			"document_count":   stats.DocumentCount,
			"caia_attribution": stats.CAIAAttribution,
		}

		if !e.matchesFilters(metadata, q.Filters) {
			continue
		}

		results = append(results, stats)
		if len(results) >= q.Limit {
			break
		}
	}

	return &Result{
		Type:    QueryAttribution,
		Count:   len(results),
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// executeSourcesQuery lists all document sources with counts
func (e *GovcExecutor) executeSourcesQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// Get documents and aggregate by source
	docQuery := &Query{
		Type:  QueryDocuments,
		Limit: 10000,
	}

	docResult, err := e.executeDocumentQuery(ctx, docQuery)
	if err != nil {
		return nil, err
	}

	// Aggregate by source
	sourceMap := make(map[string]int)
	for _, item := range docResult.Items {
		if doc, ok := item.(DocumentResult); ok {
			source := doc.Source
			if source == "" {
				source = "unknown"
			}
			sourceMap[source]++
		}
	}

	// Convert to results and apply filters
	var results []interface{}
	for source, count := range sourceMap {
		sourceResult := map[string]interface{}{
			"source": source,
			"count":  count,
		}

		if !e.matchesFilters(sourceResult, q.Filters) {
			continue
		}

		results = append(results, sourceResult)
	}

	// Sort by count descending by default
	sort.Slice(results, func(i, j int) bool {
		a := results[i].(map[string]interface{})["count"].(int)
		b := results[j].(map[string]interface{})["count"].(int)
		return a > b
	})

	// Apply limit
	if len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return &Result{
		Type:    QuerySources,
		Count:   len(results),
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// executeAuthorsQuery lists document authors with counts
func (e *GovcExecutor) executeAuthorsQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// Get documents and aggregate by author
	docQuery := &Query{
		Type:  QueryDocuments,
		Limit: 10000,
	}

	docResult, err := e.executeDocumentQuery(ctx, docQuery)
	if err != nil {
		return nil, err
	}

	// Aggregate by author
	authorMap := make(map[string]int)
	for _, item := range docResult.Items {
		if doc, ok := item.(DocumentResult); ok {
			// Count primary author
			if doc.Author != "" {
				authorMap[doc.Author]++
			}

			// Also count additional authors from metadata
			if authors, exists := doc.Metadata["authors"]; exists {
				for _, author := range strings.Split(authors, ",") {
					author = strings.TrimSpace(author)
					if author != "" && author != doc.Author {
						authorMap[author]++
					}
				}
			}
		}
	}

	// Convert to results and apply filters
	var results []interface{}
	for author, count := range authorMap {
		authorResult := map[string]interface{}{
			"author": author,
			"count":  count,
		}

		if !e.matchesFilters(authorResult, q.Filters) {
			continue
		}

		results = append(results, authorResult)
	}

	// Sort by count descending by default
	sort.Slice(results, func(i, j int) bool {
		a := results[i].(map[string]interface{})["count"].(int)
		b := results[j].(map[string]interface{})["count"].(int)
		return a > b
	})

	// Apply limit
	if len(results) > q.Limit {
		results = results[:q.Limit]
	}

	return &Result{
		Type:    QueryAuthors,
		Count:   len(results),
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// executeDocumentQuerySlow - fallback for non-govc backends
func (e *GovcExecutor) executeDocumentQuerySlow(ctx context.Context, q *Query) (*Result, error) {
	// This would implement the slower approach for non-govc backends
	// For now, return error encouraging govc usage
	return nil, fmt.Errorf("optimized queries require govc backend - consider using NewGovcBackend")
}

// Helper methods

func (e *GovcExecutor) matchesDocumentFilters(doc *document.Document, filters []Filter) bool {
	for _, filter := range filters {
		if !e.matchesDocumentFilter(doc, filter) {
			return false
		}
	}
	return true
}

func (e *GovcExecutor) matchesDocumentFilter(doc *document.Document, filter Filter) bool {
	var value interface{}
	var exists bool

	// Map filter field to document field
	switch filter.Field {
	case "id":
		value = doc.ID
		exists = true
	case "source":
		value = doc.Source.Type
		exists = doc.Source.Type != ""
	case "url":
		value = doc.Source.URL
		exists = doc.Source.URL != ""
	case "created_at":
		value = doc.CreatedAt
		exists = true
	case "updated_at":
		value = doc.UpdatedAt
		exists = true
	case "title":
		value, exists = doc.Content.Metadata["title"]
	case "author":
		value, exists = doc.Content.Metadata["author"]
		if !exists {
			value, exists = doc.Content.Metadata["authors"]
		}
	default:
		// Check in metadata
		value, exists = doc.Content.Metadata[filter.Field]
	}

	return e.matchesFilterValue(value, exists, filter)
}

func (e *GovcExecutor) matchesFilters(metadata map[string]interface{}, filters []Filter) bool {
	for _, filter := range filters {
		value, exists := metadata[filter.Field]
		if !e.matchesFilterValue(value, exists, filter) {
			return false
		}
	}
	return true
}

func (e *GovcExecutor) matchesFilterValue(value interface{}, exists bool, filter Filter) bool {
	switch filter.Operator {
	case OpEquals:
		if !exists || value != filter.Value {
			return false
		}
	case OpNotEquals:
		if exists && value == filter.Value {
			return false
		}
	case OpContains:
		if !exists {
			return false
		}
		str, ok1 := value.(string)
		filterStr, ok2 := filter.Value.(string)
		if !ok1 || !ok2 || !strings.Contains(strings.ToLower(str), strings.ToLower(filterStr)) {
			return false
		}
	case OpGreater:
		if !exists {
			return false
		}
		// Handle time comparison
		if t1, ok1 := value.(time.Time); ok1 {
			if t2, ok2 := filter.Value.(time.Time); ok2 {
				return t1.After(t2)
			}
		}
		// Handle numeric comparison
		if n1, ok1 := value.(float64); ok1 {
			if n2, ok2 := filter.Value.(float64); ok2 {
				return n1 > n2
			}
		}
		return false
	case OpLess:
		if !exists {
			return false
		}
		// Handle time comparison
		if t1, ok1 := value.(time.Time); ok1 {
			if t2, ok2 := filter.Value.(time.Time); ok2 {
				return t1.Before(t2)
			}
		}
		// Handle numeric comparison
		if n1, ok1 := value.(float64); ok1 {
			if n2, ok2 := filter.Value.(float64); ok2 {
				return n1 < n2
			}
		}
		return false
	case OpExists:
		if !exists {
			return false
		}
	case OpNotExists:
		if exists {
			return false
		}
	}
	return true
}

func (e *GovcExecutor) hasCAIAAttribution(doc DocumentResult) bool {
	// Check for Caia Tech attribution in metadata
	if attribution, exists := doc.Metadata["attribution"]; exists {
		return strings.Contains(strings.ToLower(attribution), "caia") ||
			strings.Contains(strings.ToLower(attribution), "caiatech")
	}

	// Check for Caia Tech in other metadata fields
	for _, value := range doc.Metadata {
		if strings.Contains(strings.ToLower(value), "caia") {
			return true
		}
	}

	return false
}

func (e *GovcExecutor) sortDocumentResults(results []interface{}, orderBy string, descending bool) {
	sort.Slice(results, func(i, j int) bool {
		a, aOk := results[i].(DocumentResult)
		b, bOk := results[j].(DocumentResult)
		if !aOk || !bOk {
			return false
		}

		var aVal, bVal interface{}
		switch orderBy {
		case "created_at":
			aVal, bVal = a.CreatedAt, b.CreatedAt
		case "updated_at":
			aVal, bVal = a.UpdatedAt, b.UpdatedAt
		case "title":
			aVal, bVal = a.Title, b.Title
		case "author":
			aVal, bVal = a.Author, b.Author
		case "source":
			aVal, bVal = a.Source, b.Source
		default:
			// Check metadata
			aVal = a.Metadata[orderBy]
			bVal = b.Metadata[orderBy]
		}

		// Compare values
		less := false
		switch av := aVal.(type) {
		case string:
			if bv, ok := bVal.(string); ok {
				less = av < bv
			}
		case time.Time:
			if bv, ok := bVal.(time.Time); ok {
				less = av.Before(bv)
			}
		case int:
			if bv, ok := bVal.(int); ok {
				less = av < bv
			}
		case float64:
			if bv, ok := bVal.(float64); ok {
				less = av < bv
			}
		}

		if descending {
			return !less
		}
		return less
	})
}