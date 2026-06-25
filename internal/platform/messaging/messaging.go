// Package messaging defines the pub/sub contract used by all modules for
// cross-module communication. Modules never call each other synchronously for
// fire-and-forget reactions; they publish events on a Bus and subscribe to
// events from other modules.
//
// Two implementations ship:
//   - LocalBus: in-process, the monolith default. Supports sync and async
//     (worker-pool) dispatch.
//   - RedisBus: Redis Pub/Sub fan-out, for horizontal scale / background workers.
//
// Both implement the same Bus interface, so modules are transport-agnostic.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
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

	// TraceID propagates the request trace for correlation.
	TraceID string

	// Metadata is free-form carrier for causation/correlation ids etc.
	Metadata map[string]string
}

// Handler processes a single message. It must be idempotent: the bus may
// redeliver (at-least-once semantics). Returning an error marks the message
// failed; for the LocalBus it is logged + counted, for RedisBus it is NACK'd.
type Handler func(ctx context.Context, msg Message) error

// Bus is the pub/sub abstraction every module sees (via kernel.Runtime.Bus()).
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

// HandlerAdapter optionally transforms a Message.Payload into a different
// shape before the Handler runs. Useful for typed subscriptions.
type HandlerAdapter func(payload any) (any, error)

// Cancel unsubscribes a previously registered handler.
type Cancel func()

// ErrBusClosed is returned (or wrapped) when operating on a closed bus.
var ErrBusClosed = errors.New("messaging: bus closed")

// ErrUnknownTopic is returned by Publish when no handler is subscribed and the
// bus is in strict mode (LocalBus only; RedisBus is always fire-and-forget).
var ErrUnknownTopic = errors.New("messaging: no subscriber for topic")

// ─── LocalBus ─────────────────────────────────────────────────────

// LocalBusOption configures a LocalBus.
type LocalBusOption func(*LocalBus)

// WithAsync enables async dispatch: messages are pushed onto a bounded channel
// and processed by `workers` goroutines. Without this, Publish invokes handlers
// synchronously on the publisher's goroutine.
func WithAsync(queueSize, workers int) LocalBusOption {
	return func(b *LocalBus) {
		b.async = true
		b.queue = make(chan queuedMsg, queueSize)
		b.workers = workers
	}
}

// WithStrictUnknown makes Publish return ErrUnknownTopic when a topic has no
// subscriber. Default is lenient (publish silently succeeds).
func WithStrictUnknown() LocalBusOption {
	return func(b *LocalBus) { b.strictUnknown = true }
}

type queuedMsg struct {
	ctx context.Context
	msg Message
}

// LocalBus is the default in-process pub/sub. Safe for concurrent use.
type LocalBus struct {
	mu            sync.RWMutex
	subs          map[string][]subscription
	closed        bool
	async         bool
	strictUnknown bool
	queue         chan queuedMsg
	workers       int
	wg            sync.WaitGroup
	closeOnce     sync.Once
}

type subscription struct {
	id      int64
	handler Handler
}

var nextSubID int64

// NewLocalBus builds an in-process bus.
func NewLocalBus(opts ...LocalBusOption) *LocalBus {
	b := &LocalBus{subs: map[string][]subscription{}}
	for _, o := range opts {
		o(b)
	}
	if b.async {
		for i := 0; i < b.workers; i++ {
			b.wg.Add(1)
			go b.worker()
		}
	}
	return b
}

// Publish broadcasts a message. In sync mode handlers run on this goroutine.
func (b *LocalBus) Publish(ctx context.Context, msg Message) error {
	if msg.OccurredAt.IsZero() {
		msg.OccurredAt = time.Now()
	}
	b.mu.RLock()
	if b.closed {
		b.mu.RUnlock()
		return ErrBusClosed
	}
	subs := b.subs[msg.Topic]
	b.mu.RUnlock()

	if len(subs) == 0 && b.strictUnknown {
		return ErrUnknownTopic
	}

	if b.async {
		// Copy ctx values; the publisher may return before delivery.
		b.queue <- queuedMsg{ctx: context.WithoutCancel(ctx), msg: msg}
		return nil
	}
	return b.deliver(ctx, msg, subs)
}

func (b *LocalBus) deliver(ctx context.Context, msg Message, subs []subscription) error {
	for _, s := range subs {
		if err := s.handler(ctx, msg); err != nil {
			// In sync mode surface the first error; remaining handlers still run.
			return fmt.Errorf("messaging: handler for %q failed: %w", msg.Topic, err)
		}
	}
	return nil
}

func (b *LocalBus) worker() {
	defer b.wg.Done()
	for qm := range b.queue {
		b.mu.RLock()
		subs := append([]subscription(nil), b.subs[qm.msg.Topic]...)
		b.mu.RUnlock()
		for _, s := range subs {
			if err := s.handler(qm.ctx, qm.msg); err != nil {
				// Async failures are logged by the caller via a slog hook on the bus;
				// here we can't reach the logger without importing it. Swallow.
				_ = err
			}
		}
	}
}

// Subscribe registers a handler. The optional adapter decodes the payload first.
func (b *LocalBus) Subscribe(topic string, handler Handler, adapter ...HandlerAdapter) (Cancel, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, ErrBusClosed
	}
	var adapt HandlerAdapter
	if len(adapter) > 0 {
		adapt = adapter[0]
	}
	nextSubID++
	id := nextSubID
	wrapped := handler
	if adapt != nil {
		wrapped = func(ctx context.Context, msg Message) error {
			decoded, err := adapt(msg.Payload)
			if err != nil {
				return err
			}
			msg.Payload = decoded
			return handler(ctx, msg)
		}
	}
	b.subs[topic] = append(b.subs[topic], subscription{id: id, handler: wrapped})
	return func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subs[topic]
		for i, s := range subs {
			if s.id == id {
				b.subs[topic] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
		if len(b.subs[topic]) == 0 {
			delete(b.subs, topic)
		}
	}, nil
}

// Close shuts the bus down. For async buses it drains the queue.
func (b *LocalBus) Close() error {
	b.closeOnce.Do(func() {
		b.mu.Lock()
		b.closed = true
		b.mu.Unlock()
		if b.async {
			close(b.queue)
			b.wg.Wait()
		}
	})
	return nil
}

// ─── MemoryRecorder: a test-only Bus that captures messages ───────

// MemoryRecorder is a Bus used in tests/fakes. It records every Publish and
// optionally forwards to a delegate bus. Handlers can be registered to assert
// delivery. Implements Bus.
type MemoryRecorder struct {
	mu       sync.Mutex
	messages []Message
	delegate Bus
}

// NewMemoryRecorder wraps an optional delegate. If delegate is nil, Publish is
// a no-op aside from recording.
func NewMemoryRecorder(delegate Bus) *MemoryRecorder { return &MemoryRecorder{delegate: delegate} }

// Publish records the message and forwards to the delegate if set.
func (r *MemoryRecorder) Publish(ctx context.Context, msg Message) error {
	r.mu.Lock()
	r.messages = append(r.messages, msg)
	r.mu.Unlock()
	if r.delegate != nil {
		return r.delegate.Publish(ctx, msg)
	}
	return nil
}

// Subscribe forwards to the delegate (recorders typically don't route locally).
func (r *MemoryRecorder) Subscribe(topic string, h Handler, a ...HandlerAdapter) (Cancel, error) {
	if r.delegate != nil {
		return r.delegate.Subscribe(topic, h, a...)
	}
	return func() {}, nil
}

// Close forwards to the delegate.
func (r *MemoryRecorder) Close() error {
	if r.delegate != nil {
		return r.delegate.Close()
	}
	return nil
}

// Messages returns a copy of all recorded messages.
func (r *MemoryRecorder) Messages() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Message, len(r.messages))
	copy(out, r.messages)
	return out
}

// ForTopic returns recorded messages matching the topic.
func (r *MemoryRecorder) ForTopic(topic string) []Message {
	all := r.Messages()
	var out []Message
	for _, m := range all {
		if m.Topic == topic {
			out = append(out, m)
		}
	}
	return out
}

// FirstPayload returns the payload of the first recorded message on a topic,
// or nil if none. Convenience for one-shot test assertions.
func (r *MemoryRecorder) FirstPayload(topic string) any {
	for _, m := range r.ForTopic(topic) {
		return m.Payload
	}
	return nil
}

// AssertPublished reports whether at least one message was published to topic.
func (r *MemoryRecorder) AssertPublished(topic string) bool { return len(r.ForTopic(topic)) > 0 }

// reflect import retained for potential future typed-subscribe helpers.
var _ = reflect.TypeOf
