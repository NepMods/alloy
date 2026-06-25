// Package audit writes to the kernel-owned, immutable audit_log. Modules MUST
// record any security-relevant mutation through here; they never own the table.
//
// Immutability is enforced at the schema level: the audit_log table has no
// UPDATE/DELETE path in the application code, and a future DB role will revoke
// UPDATE/DELETE on it entirely. For now the contract is social + review-enforced.
package audit

import (
	"alloy/models/contract"
	"context"
	"errors"
)

// Writer is the persistence seam for audit entries. The kernel implements it.
type Writer interface {
	Write(ctx context.Context, entry contract.AuditEntry) error
}

// Audit implements contract.Audit. It is a thin facade over a Writer, defaulting
// tenant/actor from context when the entry omits them.
type Audit struct {
	w Writer
}

// New builds an Audit backed by w.
func New(w Writer) *Audit { return &Audit{w: w} }

// Record persists an audit entry.
func (a *Audit) Record(ctx context.Context, entry contract.AuditEntry) error {
	if a == nil || a.w == nil {
		return errors.New("audit: no writer configured")
	}
	return a.w.Write(ctx, entry)
}

// NoopWriter discards everything — used in module unit tests.
type NoopWriter struct{}

// Write implements Writer.
func (NoopWriter) Write(context.Context, contract.AuditEntry) error { return nil }
