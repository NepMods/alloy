package auth

import "context"

// ─── Provided ports (this module's public contract) ──────────────
//
// Other modules Require these. Bumping any signature → bump Manifest.Version.

// IdentityResolver answers "who is this user?" without exposing credentials.
// Modules call this when they have only a user id (e.g. from a journal entry
// created_by field) and need a display name / tenant membership.
type IdentityResolver interface {
	// UserByID returns a public profile for a user, or ErrUserNotFound.
	UserByID(ctx context.Context, userID int64) (*UserProfile, error)
	// UsersByIDs batches lookups (used by report "created by" columns).
	UsersByIDs(ctx context.Context, userIDs []int64) (map[int64]UserProfile, error)
	// Exists reports whether a user id is valid.
	Exists(ctx context.Context, userID int64) (bool, error)
}

// MembershipResolver answers role/membership questions for RBAC-aware modules.
type MembershipResolver interface {
	// MembersOf lists the members of a tenant with their roles.
	MembersOf(ctx context.Context, tenantID int64) ([]Member, error)
	// Role returns a single user's role in a tenant, or "" if not a member.
	Role(ctx context.Context, tenantID, userID int64) (string, error)
}

// ─── Value types (part of the contract) ──────────────────────────

// UserProfile is the public, credential-free view of a user. It is the ONLY
// shape other modules ever see for a user.
type UserProfile struct {
	ID    int64  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// Member is a tenant membership row as seen by other modules.
type Member struct {
	UserID int64  `json:"user_id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Role   string `json:"role"`
}

// AuditAction is a typed reminder that core reports audit entries via the
// kernel's Audit interface (it does NOT write audit_log itself).
const (
	AuditActionRegister = "core.user.register"
	AuditActionLogin    = "core.user.login"
	AuditActionUpdate   = "core.user.update"
	AuditActionInvite   = "core.member.invite"
	AuditActionRoleSet  = "core.member.role_set"
)

// Events core publishes. Other modules may subscribe (e.g. notifications).
const (
	EventUserRegistered = "core.user.registered" // payload: UserProfile
	EventUserLoggedIn   = "core.user.logged_in"  // payload: UserProfile
)
