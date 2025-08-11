package pipeline

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// EventHandler is a function that handles document events
type EventHandler func(ctx context.Context, event *DocumentEvent) error

// Subscription represents an event subscription
type Subscription struct {
	ID          string
	EventTypes  []EventType
	Handler     EventHandler
	BufferSize  int
	channel     chan *DocumentEvent
	ctx         context.Context
	cancel      context.CancelFunc
	mu          sync.Mutex
	active      bool
}

// EventBus manages pub/sub for document events
type EventBus struct {
	mu            sync.RWMutex
	subscriptions map[string]*Subscription
	eventBuffer   chan *DocumentEvent
	workers       int
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	stats         EventBusStats
	statsMu       sync.RWMutex // Protects stats fields
}

// EventBusStats tracks event bus statistics
type EventBusStats struct {
	EventsPublished   int64 `json:"events_published"`
	EventsDelivered   int64 `json:"events_delivered"`
	EventsFailed      int64 `json:"events_failed"`
	ActiveSubscribers int64 `json:"active_subscribers"`
	EventsInBuffer    int64 `json:"events_in_buffer"`
}

// NewEventBus creates a new event bus
func NewEventBus(bufferSize, workers int) *EventBus {
	ctx, cancel := context.WithCancel(context.Background())
	
	eb := &EventBus{
		subscriptions: make(map[string]*Subscription),
		eventBuffer:   make(chan *DocumentEvent, bufferSize),
		workers:       workers,
		ctx:           ctx,
		cancel:        cancel,
	}
	
	// Start worker goroutines
	for i := 0; i < workers; i++ {
		eb.wg.Add(1)
		go eb.worker(i)
	}
	
	log.Info().
		Int("buffer_size", bufferSize).
		Int("workers", workers).
		Msg("Event bus started")
	
	return eb
}

// Publish publishes an event to all matching subscribers
func (eb *EventBus) Publish(event *DocumentEvent) error {
	select {
	case eb.eventBuffer <- event:
		eb.statsMu.Lock()
		eb.stats.EventsPublished++
		eb.stats.EventsInBuffer = int64(len(eb.eventBuffer))
		eb.statsMu.Unlock()
		return nil
	case <-eb.ctx.Done():
		return fmt.Errorf("event bus is shutting down")
	default:
		// Buffer is full, drop event
		log.Warn().
			Str("event_id", event.ID).
			Str("event_type", string(event.Type)).
			Msg("Event dropped due to full buffer")
		return fmt.Errorf("event buffer is full")
	}
}

// Subscribe creates a new subscription for specific event types
func (eb *EventBus) Subscribe(eventTypes []EventType, handler EventHandler, bufferSize int) (*Subscription, error) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	
	ctx, cancel := context.WithCancel(eb.ctx)
	
	sub := &Subscription{
		ID:         generateSubscriptionID(),
		EventTypes: eventTypes,
		Handler:    handler,
		BufferSize: bufferSize,
		channel:    make(chan *DocumentEvent, bufferSize),
		ctx:        ctx,
		cancel:     cancel,
		active:     true,
	}
	
	eb.subscriptions[sub.ID] = sub
	eb.mu.Unlock()
	
	eb.statsMu.Lock()
	eb.stats.ActiveSubscribers++
	eb.statsMu.Unlock()
	
	log.Info().
		Str("subscription_id", sub.ID).
		Interface("event_types", eventTypes).
		Int("buffer_size", bufferSize).
		Msg("New subscription created")
	
	return sub, nil
}

// Unsubscribe removes a subscription
func (eb *EventBus) Unsubscribe(subscriptionID string) error {
	eb.mu.Lock()
	
	sub, exists := eb.subscriptions[subscriptionID]
	if !exists {
		eb.mu.Unlock()
		return fmt.Errorf("subscription not found: %s", subscriptionID)
	}
	
	sub.mu.Lock()
	sub.active = false
	sub.cancel()
	close(sub.channel)
	sub.mu.Unlock()
	
	delete(eb.subscriptions, subscriptionID)
	eb.mu.Unlock()
	
	eb.statsMu.Lock()
	eb.stats.ActiveSubscribers--
	eb.statsMu.Unlock()
	
	log.Info().Str("subscription_id", subscriptionID).Msg("Subscription removed")
	return nil
}

// Close shuts down the event bus
func (eb *EventBus) Close() {
	eb.cancel()
	eb.wg.Wait()
	
	eb.mu.Lock()
	for _, sub := range eb.subscriptions {
		sub.cancel()
	}
	eb.mu.Unlock()
	
	log.Info().Msg("Event bus shut down")
}

// GetStats returns current event bus statistics
func (eb *EventBus) GetStats() EventBusStats {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	
	stats := eb.stats
	stats.EventsInBuffer = int64(len(eb.eventBuffer))
	return stats
}

// worker processes events from the buffer
func (eb *EventBus) worker(workerID int) {
	defer eb.wg.Done()
	
	log.Debug().Int("worker_id", workerID).Msg("Event bus worker started")
	
	for {
		select {
		case event := <-eb.eventBuffer:
			eb.statsMu.Lock()
			eb.stats.EventsInBuffer = int64(len(eb.eventBuffer))
			eb.statsMu.Unlock()
			eb.deliverEvent(event)
		case <-eb.ctx.Done():
			log.Debug().Int("worker_id", workerID).Msg("Event bus worker stopping")
			return
		}
	}
}

// deliverEvent delivers an event to matching subscribers
func (eb *EventBus) deliverEvent(event *DocumentEvent) {
	eb.mu.RLock()
	matchingSubscriptions := make([]*Subscription, 0)
	
	for _, sub := range eb.subscriptions {
		if eb.eventMatchesSubscription(event, sub) {
			matchingSubscriptions = append(matchingSubscriptions, sub)
		}
	}
	eb.mu.RUnlock()
	
	for _, sub := range matchingSubscriptions {
		go eb.deliverToSubscription(event, sub)
	}
}

// deliverToSubscription delivers an event to a specific subscription
func (eb *EventBus) deliverToSubscription(event *DocumentEvent, sub *Subscription) {
	sub.mu.Lock()
	if !sub.active {
		sub.mu.Unlock()
		return
	}
	sub.mu.Unlock()
	
	// Try to deliver with timeout
	ctx, cancel := context.WithTimeout(sub.ctx, 5*time.Second)
	defer cancel()
	
	select {
	case sub.channel <- event:
		// Event queued, now call handler
		go func() {
			if err := sub.Handler(ctx, event); err != nil {
				eb.statsMu.Lock()
				eb.stats.EventsFailed++
				eb.statsMu.Unlock()
				log.Error().
					Err(err).
					Str("subscription_id", sub.ID).
					Str("event_id", event.ID).
					Msg("Event handler failed")
			} else {
				eb.statsMu.Lock()
				eb.stats.EventsDelivered++
				eb.statsMu.Unlock()
			}
		}()
	case <-ctx.Done():
		eb.statsMu.Lock()
		eb.stats.EventsFailed++
		eb.statsMu.Unlock()
		log.Warn().
			Str("subscription_id", sub.ID).
			Str("event_id", event.ID).
			Msg("Event delivery timeout")
	}
}

// eventMatchesSubscription checks if an event matches a subscription
func (eb *EventBus) eventMatchesSubscription(event *DocumentEvent, sub *Subscription) bool {
	for _, eventType := range sub.EventTypes {
		if event.Type == eventType {
			return true
		}
	}
	return false
}

// generateSubscriptionID generates a unique subscription ID
func generateSubscriptionID() string {
	return fmt.Sprintf("sub_%d_%s", time.Now().UnixNano(), generateRandomString(6))
}