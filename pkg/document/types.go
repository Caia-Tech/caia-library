package document

import (
	"fmt"
	"time"
)

// Document represents a document in the CAIA Library system
type Document struct {
	ID        string    `json:"id"`
	Source    Source    `json:"source"`
	Content   Content   `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Source describes where a document came from
type Source struct {
	Type string `json:"type"`           // Document type: text, html, pdf
	URL  string `json:"url,omitempty"`  // Source URL if fetched from web
	Path string `json:"path,omitempty"` // Local path if from filesystem
}

// Content holds the document's actual data
type Content struct {
	Raw        []byte            `json:"-"`                     // Raw binary content (not serialized to JSON)
	Text       string            `json:"text"`                  // Extracted text content
	Metadata   map[string]string `json:"metadata"`              // Arbitrary metadata
	Embeddings []float32         `json:"embeddings,omitempty"`  // Vector embeddings
}

// GitPath returns the storage path within the Git repository
// Format: documents/{type}/{YYYY/MM}/{id}
func (d *Document) GitPath() string {
	date := d.CreatedAt.Format("2006/01")
	return fmt.Sprintf("documents/%s/%s/%s", d.Source.Type, date, d.ID)
}

// Validate checks if the document has required fields
func (d *Document) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("document ID cannot be empty")
	}
	if d.Source.Type == "" {
		return fmt.Errorf("document source type cannot be empty")
	}
	if d.Source.URL == "" && d.Source.Path == "" {
		return fmt.Errorf("document must have either URL or path")
	}
	return nil
}