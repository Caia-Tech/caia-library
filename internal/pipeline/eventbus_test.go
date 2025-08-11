package pipeline

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Caia-Tech/caia-library/pkg/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventBusBasicPubSub(t *testing.T) {
	// Create event bus
	eventBus := NewEventBus(100, 2)
	defer eventBus.Close()
	
	// Track received events
	var receivedEvents int32
	var lastEvent *DocumentEvent
	
	// Create handler
	handler := func(ctx context.Context, event *DocumentEvent) error {
		atomic.AddInt32(&receivedEvents, 1)
		lastEvent = event
		return nil
	}
	
	// Subscribe to document added events
	sub, err := eventBus.Subscribe([]EventType{EventDocumentAdded}, handler, 10)
	require.NoError(t, err)
	require.NotNil(t, sub)
	
	// Create test document
	doc := &document.Document{
		ID: "test-doc-001",
		Source: document.Source{
			Type: "text",
			URL:  "https://example.com/test.txt",
		},
		Content: document.Content{
			Text: "Test document for event bus",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Publish event
	event := NewDocumentEvent(EventDocumentAdded, doc)
	err = eventBus.Publish(event)
	require.NoError(t, err)
	
	// Wait for event processing
	time.Sleep(100 * time.Millisecond)
	
	// Verify event was received
	assert.Equal(t, int32(1), atomic.LoadInt32(&receivedEvents))
	assert.NotNil(t, lastEvent)
	assert.Equal(t, EventDocumentAdded, lastEvent.Type)
	assert.Equal(t, doc.ID, lastEvent.Document.ID)
	
	// Check statistics
	stats := eventBus.GetStats()
	assert.Equal(t, int64(1), stats.EventsPublished)
	assert.Equal(t, int64(1), stats.ActiveSubscribers)
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	eventBus := NewEventBus(100, 2)
	defer eventBus.Close()
	
	// Track events for multiple subscribers
	var subscriber1Events int32
	var subscriber2Events int32
	
	handler1 := func(ctx context.Context, event *DocumentEvent) error {
		atomic.AddInt32(&subscriber1Events, 1)
		return nil
	}
	
	handler2 := func(ctx context.Context, event *DocumentEvent) error {
		atomic.AddInt32(&subscriber2Events, 1)
		return nil
	}
	
	// Subscribe both handlers to the same event type
	_, err := eventBus.Subscribe([]EventType{EventDocumentAdded}, handler1, 10)
	require.NoError(t, err)
	
	_, err = eventBus.Subscribe([]EventType{EventDocumentAdded}, handler2, 10)
	require.NoError(t, err)
	
	// Publish event
	doc := &document.Document{ID: "multi-test-001"}
	event := NewDocumentEvent(EventDocumentAdded, doc)
	err = eventBus.Publish(event)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	
	// Both subscribers should receive the event
	assert.Equal(t, int32(1), atomic.LoadInt32(&subscriber1Events))
	assert.Equal(t, int32(1), atomic.LoadInt32(&subscriber2Events))
	
	stats := eventBus.GetStats()
	assert.Equal(t, int64(2), stats.ActiveSubscribers)
}

func TestEventBusEventFiltering(t *testing.T) {
	eventBus := NewEventBus(100, 2)
	defer eventBus.Close()
	
	var addedEvents int32
	var updatedEvents int32
	
	// Handler only for added events
	addedHandler := func(ctx context.Context, event *DocumentEvent) error {
		if event.Type == EventDocumentAdded {
			atomic.AddInt32(&addedEvents, 1)
		}
		return nil
	}
	
	// Handler only for updated events
	updatedHandler := func(ctx context.Context, event *DocumentEvent) error {
		if event.Type == EventDocumentUpdated {
			atomic.AddInt32(&updatedEvents, 1)
		}
		return nil
	}
	
	// Subscribe to different event types
	_, err := eventBus.Subscribe([]EventType{EventDocumentAdded}, addedHandler, 10)
	require.NoError(t, err)
	
	_, err = eventBus.Subscribe([]EventType{EventDocumentUpdated}, updatedHandler, 10)
	require.NoError(t, err)
	
	// Publish different types of events
	doc := &document.Document{ID: "filter-test-001"}
	
	addedEvent := NewDocumentEvent(EventDocumentAdded, doc)
	updatedEvent := NewDocumentEvent(EventDocumentUpdated, doc)
	
	err = eventBus.Publish(addedEvent)
	require.NoError(t, err)
	
	err = eventBus.Publish(updatedEvent)
	require.NoError(t, err)
	
	// Wait for processing
	time.Sleep(100 * time.Millisecond)
	
	// Check that events were filtered correctly
	assert.Equal(t, int32(1), atomic.LoadInt32(&addedEvents))
	assert.Equal(t, int32(1), atomic.LoadInt32(&updatedEvents))
}