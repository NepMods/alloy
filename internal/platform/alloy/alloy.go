// Package httpx provides the JSON response envelope and error mapping shared by
// every module's HTTP handlers. All API responses use one of the OK variants;
// all errors use the Problem writer so the client always sees the same shape.
package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// Envelope is the canonical API response body for success cases.
type Envelope struct {
	Data any    `json:"data,omitempty"`
	Meta *Meta  `json:"meta,omitempty"`
	Err  *Error `json:"error,omitempty"`
}

// Meta carries pagination info.
type Meta struct {
	Page    int   `json:"page"`
	PerPage int   `json:"per_page"`
	Total   int64 `json:"total"`
}

// Error is the canonical error body.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	// Details included only when APP_ENV != production (avoids leaking internals).
	Details any `json:"details,omitempty"`
}

// OK writes a 200 with {data: v}.
func OK(w http.ResponseWriter, r *http.Request, v any) {
	write(w, r, http.StatusOK, Envelope{Data: v})
}

// Created writes a 201 with {data: v}.
func Created(w http.ResponseWriter, r *http.Request, v any) {
	write(w, r, http.StatusCreated, Envelope{Data: v})
}

// OKPaginated writes a 200 with {data: items, meta: {...}}.
func OKPaginated(w http.ResponseWriter, r *http.Request, items any, page, perPage int, total int64) {
	write(w, r, http.StatusOK, Envelope{
		Data: items,
		Meta: &Meta{Page: page, PerPage: perPage, Total: total},
	})
}

// NoContent writes 204.
func NoContent(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// write serializes the envelope as JSON.
func write(w http.ResponseWriter, r *http.Request, status int, env Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if status == http.StatusNoContent {
		return
	}
	if err := json.NewEncoder(w).Encode(env); err != nil {
		// can't change status now; best-effort log via request context if present.
		_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"encode failed"}}`))
	}
}

// ─── Errors ──────────────────────────────────────────────────────

// AppError is a typed API error. Modules construct one and pass to Problem().
type AppError struct {
	Status  int
	Code    string
	Message string
	Cause   error
	Details any
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return e.Code + ": " + e.Message + ": " + e.Cause.Error()
	}
	return e.Code + ": " + e.Message
}
func (e *AppError) Unwrap() error { return e.Cause }

// NewAppError builds a typed error.
func NewAppError(status int, code, msg string, opts ...func(*AppError)) *AppError {
	e := &AppError{Status: status, Code: code, Message: msg}
	for _, o := range opts {
		o(e)
	}
	return e
}

// WithCause attaches a wrapped cause (not serialized in prod).
func WithCause(err error) func(*AppError) {
	return func(e *AppError) { e.Cause = err }
}

// WithDetails attaches debug details.
func WithDetails(v any) func(*AppError) {
	return func(e *AppError) { e.Details = v }
}

// Convenience constructors for the common cases.
func BadRequest(code, msg string, opts ...func(*AppError)) *AppError {
	return NewAppError(http.StatusBadRequest, code, msg, opts...)
}
func Unauthorized(msg string) *AppError {
	if msg == "" {
		msg = "authentication required"
	}
	return NewAppError(http.StatusUnauthorized, "unauthorized", msg)
}
func Forbidden(msg string) *AppError {
	if msg == "" {
		msg = "you do not have permission to do this"
	}
	return NewAppError(http.StatusForbidden, "forbidden", msg)
}
func NotFound(msg string) *AppError {
	if msg == "" {
		msg = "not found"
	}
	return NewAppError(http.StatusNotFound, "not_found", msg)
}
func Conflict(code, msg string) *AppError {
	return NewAppError(http.StatusConflict, code, msg)
}
func Internal(msg string, cause error) *AppError {
	return NewAppError(http.StatusInternalServerError, "internal", msg, WithCause(cause))
}

// Problem writes an AppError as the canonical error envelope. If err is not an
// *AppError it is mapped to a 500 "internal".
func Problem(w http.ResponseWriter, r *http.Request, err error) {
	var ae *AppError
	if !errors.As(err, &ae) {
		ae = Internal("something went wrong", err)
	}
	env := Envelope{Err: &Error{Code: ae.Code, Message: ae.Message}}
	if ae.Details != nil && !isProd(r) {
		env.Err.Details = ae.Details
	}
	write(w, r, ae.Status, env)
}

// isProd is set per-request via context (see server). Default false.
func isProd(r *http.Request) bool {
	if v, ok := r.Context().Value(ctxIsProdKey{}).(bool); ok {
		return v
	}
	return false
}

type ctxIsProdKey struct{}

// WithIsProd stashes the prod flag in request context for Problem().
func WithIsProd(ctx context.Context, isProd bool) context.Context {
	return context.WithValue(ctx, ctxIsProdKey{}, isProd)
}

// ─── decode ──────────────────────────────────────────────────────

// DecodeJSON decodes r.Body into v and returns a BadRequest AppError on failure.
func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return BadRequest("invalid_body", "request body is required")
	}
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return BadRequest("invalid_body", "could not parse JSON body", WithCause(err))
	}
	return nil
}
