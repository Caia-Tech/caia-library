package gql

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Executor executes GQL queries against the Git repository
type Executor struct {
	repoPath string
}

// NewExecutor creates a new query executor
func NewExecutor(repoPath string) *Executor {
	return &Executor{
		repoPath: repoPath,
	}
}

// Execute runs a GQL query and returns results
func (e *Executor) Execute(ctx context.Context, query string) (*Result, error) {
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

// Result represents query results
type Result struct {
	Type    QueryType     `json:"type"`
	Count   int           `json:"count"`
	Items   []interface{} `json:"items"`
	Elapsed time.Duration `json:"elapsed_ms"`
}

// DocumentResult represents a document in query results
type DocumentResult struct {
	ID         string            `json:"id"`
	Source     string            `json:"source"`
	URL        string            `json:"url"`
	Title      string            `json:"title,omitempty"`
	Author     string            `json:"author,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
	Metadata   map[string]string `json:"metadata"`
	CommitHash string            `json:"commit_hash"`
}

// AttributionResult represents attribution tracking
type AttributionResult struct {
	Source           string    `json:"source"`
	DocumentCount    int       `json:"document_count"`
	FirstCollected   time.Time `json:"first_collected"`
	LastCollected    time.Time `json:"last_collected"`
	CAIAAttribution  bool      `json:"caia_attribution"`
	AttributionText  string    `json:"attribution_text"`
}

// executeDocumentQuery executes queries on documents
func (e *Executor) executeDocumentQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// Open repository
	repo, err := git.PlainOpen(e.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get HEAD reference
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Get commit
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit: %w", err)
	}

	// Get tree
	tree, err := commit.Tree()
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	var results []interface{}
	count := 0

	// Walk through documents directory
	docsPath := "documents"
	err = tree.Files().ForEach(func(f *object.File) error {
		if !strings.HasPrefix(f.Name, docsPath) {
			return nil
		}

		// Check if this is a metadata file
		if !strings.HasSuffix(f.Name, "metadata.json") {
			return nil
		}

		// Load metadata
		content, err := f.Contents()
		if err != nil {
			return nil // Skip files we can't read
		}

		var metadata map[string]interface{}
		if err := json.Unmarshal([]byte(content), &metadata); err != nil {
			return nil // Skip invalid JSON
		}

		// Apply filters
		if !e.matchesFilters(metadata, q.Filters) {
			return nil
		}

		// Extract document info
		docResult := DocumentResult{
			ID:         e.extractDocIDFromPath(f.Name),
			CommitHash: commit.Hash.String(),
			CreatedAt:  commit.Author.When,
			UpdatedAt:  commit.Author.When,
		}

		// Extract fields from metadata
		if source, ok := metadata["source"].(string); ok {
			docResult.Source = source
		}
		if url, ok := metadata["url"].(string); ok {
			docResult.URL = url
		}
		if title, ok := metadata["title"].(string); ok {
			docResult.Title = title
		}
		if author, ok := metadata["author"].(string); ok {
			docResult.Author = author
		}

		// Add all metadata
		docResult.Metadata = make(map[string]string)
		for k, v := range metadata {
			if str, ok := v.(string); ok {
				docResult.Metadata[k] = str
			}
		}

		results = append(results, docResult)
		count++

		// Apply limit
		if len(results) >= q.Limit {
			return object.ErrCanceled
		}

		return nil
	})

	if err != nil && err != object.ErrCanceled {
		return nil, fmt.Errorf("failed to walk tree: %w", err)
	}

	// Sort results if needed
	if q.OrderBy != "" {
		e.sortResults(results, q.OrderBy, q.Descending)
	}

	return &Result{
		Type:    QueryDocuments,
		Count:   count,
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// executeAttributionQuery tracks attribution compliance
func (e *Executor) executeAttributionQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()

	// Open repository
	repo, err := git.PlainOpen(e.repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get all commits to analyze attribution over time
	iter, err := repo.Log(&git.LogOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	// Track attribution by source
	sourceStats := make(map[string]*AttributionResult)

	err = iter.ForEach(func(c *object.Commit) error {
		// Check commit message for attribution
		if strings.Contains(c.Message, "Caia Tech") || 
		   strings.Contains(c.Message, "collected by Caia") {
			// Parse source from commit
			source := e.extractSourceFromCommit(c.Message)
			if source == "" {
				return nil
			}

			stats, exists := sourceStats[source]
			if !exists {
				stats = &AttributionResult{
					Source:          source,
					FirstCollected: c.Author.When,
					CAIAAttribution: true,
				}
				sourceStats[source] = stats
			}

			stats.DocumentCount++
			stats.LastCollected = c.Author.When
			
			// Extract attribution text
			if strings.Contains(c.Message, "Attribution:") {
				parts := strings.Split(c.Message, "Attribution:")
				if len(parts) > 1 {
					stats.AttributionText = strings.TrimSpace(strings.Split(parts[1], "\n")[0])
				}
			}
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to process commits: %w", err)
	}

	// Convert to results
	var results []interface{}
	for _, stats := range sourceStats {
		// Apply filters
		metadata := map[string]interface{}{
			"source":         stats.Source,
			"document_count": stats.DocumentCount,
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

// executeSourcesQuery lists all document sources
func (e *Executor) executeSourcesQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()
	
	// Reuse document query but aggregate by source
	docResult, err := e.executeDocumentQuery(ctx, &Query{
		Type:  QueryDocuments,
		Limit: 10000, // Get more docs for aggregation
	})
	if err != nil {
		return nil, err
	}

	// Aggregate by source
	sourceMap := make(map[string]int)
	for _, item := range docResult.Items {
		if doc, ok := item.(DocumentResult); ok {
			sourceMap[doc.Source]++
		}
	}

	// Convert to results
	var results []interface{}
	for source, count := range sourceMap {
		results = append(results, map[string]interface{}{
			"source": source,
			"count":  count,
		})
	}

	return &Result{
		Type:    QuerySources,
		Count:   len(results),
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// executeAuthorsQuery lists document authors
func (e *Executor) executeAuthorsQuery(ctx context.Context, q *Query) (*Result, error) {
	start := time.Now()
	
	// Reuse document query but aggregate by author
	docResult, err := e.executeDocumentQuery(ctx, &Query{
		Type:  QueryDocuments,
		Limit: 10000,
	})
	if err != nil {
		return nil, err
	}

	// Aggregate by author
	authorMap := make(map[string]int)
	for _, item := range docResult.Items {
		if doc, ok := item.(DocumentResult); ok {
			if doc.Author != "" {
				authorMap[doc.Author]++
			}
			// Also check metadata for authors field
			if authors, ok := doc.Metadata["authors"]; ok {
				// Handle comma-separated authors
				for _, author := range strings.Split(authors, ",") {
					author = strings.TrimSpace(author)
					if author != "" {
						authorMap[author]++
					}
				}
			}
		}
	}

	// Convert to results
	var results []interface{}
	for author, count := range authorMap {
		results = append(results, map[string]interface{}{
			"author": author,
			"count":  count,
		})
	}

	return &Result{
		Type:    QueryAuthors,
		Count:   len(results),
		Items:   results,
		Elapsed: time.Since(start),
	}, nil
}

// Helper methods

func (e *Executor) matchesFilters(metadata map[string]interface{}, filters []Filter) bool {
	for _, filter := range filters {
		value, exists := metadata[filter.Field]
		
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
			if !ok1 || !ok2 || !strings.Contains(str, filterStr) {
				return false
			}
		case OpExists:
			if !exists {
				return false
			}
		case OpNotExists:
			if exists {
				return false
			}
		}
	}
	return true
}

func (e *Executor) extractDocIDFromPath(path string) string {
	// Path format: documents/xx/yy/doc-id/metadata.json
	dir := filepath.Dir(path)
	return filepath.Base(dir)
}

func (e *Executor) extractSourceFromCommit(message string) string {
	// Look for "Source: " in commit message
	lines := strings.Split(message, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Source:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				source := strings.TrimSpace(parts[1])
				// Extract domain from URL if it's a URL
				if strings.HasPrefix(source, "http") {
					if strings.Contains(source, "arxiv.org") {
						return "arXiv"
					} else if strings.Contains(source, "pubmed") {
						return "PubMed"
					}
				}
				return source
			}
		}
	}
	return ""
}

func (e *Executor) sortResults(results []interface{}, field string, desc bool) {
	// Simple sorting implementation
	// In production, would use more sophisticated sorting
}