package messaging

import (
	"context"
	"time"
)

// Message is a single published event.
type Message struct {
	// Topic is the dotted event name, e.g. "sales.invoice.issued".
	Topic string

	// Payload is the event-specific struct. Subscribers receive it decoded
	// into their handler's expected type via the HandlerAdapter.
	Payload any

	// OccurredAt is when the event was generated (publisher wall clock).
	OccurredAt time.Time

	// TenantID scopes the event to the emitting tenant. The bus does not enforce
	// isolation, but subscribers should respect it.
	TenantID int64

	// TraceID propagates the request trace for correlation.
	TraceID string

	// Metadata is free-form carrier for causation/correlation ids etc.
	Metadata map[string]string
}

type Handler func(ctx context.Context, msg Message) error
type HandlerAdapter func(payload any) (any, error)
type Cancel func()

// Bus is an asynchronous message bus for decoupled communication between modules
type Bus interface {
	// Publish broadcasts a message to all subscribers of msg.Topic. Publish is
	// non-blocking for the caller after the message is accepted by the bus.
	Publish(ctx context.Context, msg Message) error

	// Subscribe registers a handler for a topic. The optional adapter decodes
	// msg.Payload into a concrete type the handler expects; if nil, the raw
	// payload is passed. Returns a Cancel func to unsubscribe.
	Subscribe(topic string, handler Handler, adapter ...HandlerAdapter) (Cancel, error)

	// Close drains and shuts down the bus.
	Close() error
}
