package activities

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Caia-Tech/caia-library/internal/temporal/workflows"
)

type CollectorActivities struct {
	httpClient *http.Client
	seenCache  map[string]time.Time // Simple in-memory cache for deduplication
}

func NewCollectorActivities() *CollectorActivities {
	return &CollectorActivities{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		seenCache: make(map[string]time.Time),
	}
}

// CollectFromSourceActivity fetches documents from various sources
func (c *CollectorActivities) CollectFromSourceActivity(ctx context.Context, input workflows.ScheduledIngestionInput) ([]workflows.CollectedDocument, error) {
	switch input.Type {
	case "rss":
		return c.collectRSS(ctx, input)
	case "web":
		return c.collectWeb(ctx, input)
	case "api":
		return c.collectAPI(ctx, input)
	default:
		return nil, fmt.Errorf("unsupported source type: %s", input.Type)
	}
}

// collectRSS fetches documents from RSS feeds
func (c *CollectorActivities) collectRSS(ctx context.Context, input workflows.ScheduledIngestionInput) ([]workflows.CollectedDocument, error) {
	resp, err := c.httpClient.Get(input.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch RSS feed: %w", err)
	}
	defer resp.Body.Close()

	var feed RSSFeed
	if err := xml.NewDecoder(resp.Body).Decode(&feed); err != nil {
		return nil, fmt.Errorf("failed to parse RSS feed: %w", err)
	}

	var documents []workflows.CollectedDocument
	for _, item := range feed.Channel.Items {
		// Apply filters
		if !c.passesFilters(item.Title+" "+item.Description, input.Filters) {
			continue
		}

		// Generate document ID from URL
		docID := c.generateID(item.Link)

		doc := workflows.CollectedDocument{
			ID:   docID,
			URL:  item.Link,
			Type: "web", // RSS items typically link to web pages
			Metadata: map[string]string{
				"title":       item.Title,
				"description": item.Description,
				"pubDate":     item.PubDate,
				"source":      input.Name,
				"feedURL":     input.URL,
			},
		}

		// Add any custom metadata
		for k, v := range input.Metadata {
			doc.Metadata[k] = v
		}

		documents = append(documents, doc)
	}

	return documents, nil
}

// collectWeb performs web scraping (simplified for now)
func (c *CollectorActivities) collectWeb(ctx context.Context, input workflows.ScheduledIngestionInput) ([]workflows.CollectedDocument, error) {
	// Parse sitemap or crawl pages
	// For now, just return the URL itself as a document
	docID := c.generateID(input.URL)
	
	return []workflows.CollectedDocument{{
		ID:   docID,
		URL:  input.URL,
		Type: "web",
		Metadata: map[string]string{
			"source":     input.Name,
			"crawlDepth": "0",
		},
	}}, nil
}

// collectAPI fetches documents from APIs
func (c *CollectorActivities) collectAPI(ctx context.Context, input workflows.ScheduledIngestionInput) ([]workflows.CollectedDocument, error) {
	resp, err := c.httpClient.Get(input.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to call API: %w", err)
	}
	defer resp.Body.Close()

	// Parse JSON response (assuming array of documents)
	var items []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	var documents []workflows.CollectedDocument
	for _, item := range items {
		// Extract URL from item (implementation depends on API structure)
		urlField := ""
		if u, ok := item["url"].(string); ok {
			urlField = u
		} else if u, ok := item["link"].(string); ok {
			urlField = u
		} else if u, ok := item["href"].(string); ok {
			urlField = u
		}

		if urlField == "" {
			continue
		}

		docID := c.generateID(urlField)
		
		// Convert metadata
		metadata := make(map[string]string)
		for k, v := range item {
			if str, ok := v.(string); ok {
				metadata[k] = str
			} else {
				metadata[k] = fmt.Sprintf("%v", v)
			}
		}
		metadata["source"] = input.Name

		documents = append(documents, workflows.CollectedDocument{
			ID:       docID,
			URL:      urlField,
			Type:     "api",
			Metadata: metadata,
		})
	}

	return documents, nil
}

// CheckDuplicateActivity checks if we've seen this document before
func (c *CollectorActivities) CheckDuplicateActivity(ctx context.Context, documentID string) (bool, error) {
	// In production, this would check against a database or Git history
	// For now, use simple in-memory cache
	_, exists := c.seenCache[documentID]
	if !exists {
		c.seenCache[documentID] = time.Now()
	}
	return exists, nil
}

// passesFilters checks if content matches any of the filters
func (c *CollectorActivities) passesFilters(content string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}

	contentLower := strings.ToLower(content)
	for _, filter := range filters {
		if strings.Contains(contentLower, strings.ToLower(filter)) {
			return true
		}
	}
	return false
}

// generateID creates a stable ID from a URL
func (c *CollectorActivities) generateID(rawURL string) string {
	// Normalize URL
	u, err := url.Parse(rawURL)
	if err != nil {
		// Fallback to raw URL hash
		h := md5.Sum([]byte(rawURL))
		return hex.EncodeToString(h[:])
	}

	// Create consistent URL representation
	normalized := u.Scheme + "://" + u.Host + u.Path
	if u.RawQuery != "" {
		normalized += "?" + u.RawQuery
	}

	h := md5.Sum([]byte(normalized))
	return hex.EncodeToString(h[:])
}

// RSS Feed structures
type RSSFeed struct {
	XMLName xml.Name    `xml:"rss"`
	Channel RSSChannel  `xml:"channel"`
}

type RSSChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}