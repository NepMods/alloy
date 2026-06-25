package messaging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"
)

// RedisBus implements Bus over Redis Pub/Sub. Use for cross-process fan-out
// (background workers, horizontal scale). Messages are JSON-encoded; payloads
// must be serializable. At-least-once delivery is the application's
// responsibility via idempotent handlers — Redis Pub/Sub itself is at-most-once
// per connected subscriber.
type RedisBus struct {
	rdb         redis.UniversalClient
	channelFunc func(topic string) string // default: identity; lets you prefix channels
	mu          sync.RWMutex
	closed      bool
	cancels     []func() error
	closeOnce   sync.Once

	// onDeliver is an optional hook (e.g. logging) invoked per message.
	onDeliver func(topic string, err error)
}

// RedisBusOption configures a RedisBus.
type RedisBusOption func(*RedisBus)

func WithChannelPrefix(prefix string) RedisBusOption {
	return func(b *RedisBus) {
		b.channelFunc = func(topic string) string { return prefix + topic }
	}
}

// WithDeliverHook lets the caller observe per-message delivery outcomes.
func WithDeliverHook(fn func(topic string, err error)) RedisBusOption {
	return func(b *RedisBus) { b.onDeliver = fn }
}

// NewRedisBus builds a Redis-backed bus over an existing redis client.
func NewRedisBus(rdb redis.UniversalClient, opts ...RedisBusOption) *RedisBus {
	b := &RedisBus{rdb: rdb, channelFunc: func(t string) string { return t }}
	for _, o := range opts {
		o(b)
	}
	return b
}

// envelope is the on-the-wire shape. Payload is raw Message for decode fidelity.
type envelope struct {
	Topic      string            `json:"topic"`
	Payload    json.RawMessage   `json:"payload"`
	OccurredAt string            `json:"occurred_at"`
	TraceID    string            `json:"trace_id,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// Publish broadcasts to all Redis subscribers of the topic.
func (b *RedisBus) Publish(ctx context.Context, msg Message) error {
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrBusClosed
	}
	b.mu.RUnlock()

	payloadBytes, err := json.Marshal(msg.Payload)
	if err != nil {
		return fmt.Errorf("messaging: marshal payload: %w", err)
	}
	env := envelope{
		Topic:      msg.Topic,
		Payload:    payloadBytes,
		OccurredAt: msg.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000Z"),
		TraceID:    msg.TraceID,
		Metadata:   msg.Metadata,
	}
	body, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("messaging: marshal envelope: %w", err)
	}
	return b.rdb.Publish(ctx, b.channelFunc(msg.Topic), body).Err()
}

// Subscribe registers a handler. The optional adapter decodes the JSON payload
// into a concrete type before the handler runs; if no adapter is supplied, the
// handler receives the raw json.RawMessage as Payload (decode it yourself).
func (b *RedisBus) Subscribe(topic string, handler Handler, adapter ...HandlerAdapter) (Cancel, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, ErrBusClosed
	}
	b.mu.Unlock()

	var adapt HandlerAdapter
	if len(adapter) > 0 {
		adapt = adapter[0]
	}

	pubsub := b.rdb.Subscribe(context.Background(), b.channelFunc(topic))
	ch := pubsub.Channel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range ch {
			var env envelope
			if err := json.Unmarshal([]byte(msg.Payload), &env); err != nil {
				if b.onDeliver != nil {
					b.onDeliver(topic, err)
				}
				continue
			}
			payload := any(env.Payload)
			if adapt != nil {
				decoded, err := adapt(env.Payload)
				if err != nil {
					if b.onDeliver != nil {
						b.onDeliver(topic, err)
					}
					continue
				}
				payload = decoded
			}
			out := Message{
				Topic:    env.Topic,
				Payload:  payload,
				TraceID:  env.TraceID,
				Metadata: env.Metadata,
			}
			// Use a fresh context; pub/sub has no inbound request context.
			if err := handler(context.Background(), out); err != nil {
				if b.onDeliver != nil {
					b.onDeliver(topic, err)
				}
			} else if b.onDeliver != nil {
				b.onDeliver(topic, nil)
			}
		}
	}()

	cancel := func() {
		_ = pubsub.Close()
		<-done
	}
	b.mu.Lock()
	b.cancels = append(b.cancels, func() error { cancel(); return nil })
	b.mu.Unlock()
	return cancel, nil
}

// Close shuts down all subscriptions.
func (b *RedisBus) Close() error {
	var firstErr error
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		cancels := b.cancels
		b.cancels = nil
		b.mu.Unlock()
		for _, c := range cancels {
			if err := c(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
	})
	return firstErr
}

// guard against accidental nil-client misuse
var _ = errors.New
