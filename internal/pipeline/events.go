package pipeline

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
)

// EventType represents the type of document event
type EventType string

const (
	EventDocumentAdded     EventType = "document.added"
	EventDocumentUpdated   EventType = "document.updated"
	EventDocumentDeleted   EventType = "document.deleted"
	EventDocumentProcessed EventType = "document.processed"
	EventDocumentCleaned   EventType = "document.cleaned"
	EventDocumentIndexed   EventType = "document.indexed"
	EventProcessingFailed  EventType = "processing.failed"
)

// DocumentEvent represents an event in the document processing pipeline
type DocumentEvent struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Document  *document.Document     `json:"document,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Error     string                 `json:"error,omitempty"`
}

// NewDocumentEvent creates a new document event
func NewDocumentEvent(eventType EventType, doc *document.Document) *DocumentEvent {
	return &DocumentEvent{
		ID:        GenerateEventID(),
		Type:      eventType,
		Timestamp: time.Now(),
		Document:  doc,
		Metadata:  make(map[string]interface{}),
	}
}

// GenerateEventID generates a unique event ID
func GenerateEventID() string {
	return fmt.Sprintf("evt_%d_%s", time.Now().UnixNano(), generateRandomString(8))
}

func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}