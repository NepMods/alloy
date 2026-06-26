package kernel

import (
	"alloy/internal/app/config"
	"alloy/internal/platform/messaging"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	platformaudit "alloy/internal/platform/audit"

	contract "alloy/models/contract"

	server "alloy/models/server"

	"github.com/NepMods/ember"

	goredis "github.com/redis/go-redis/v9"
)

// Deps are the inputs needed to build the kernel.
type Deps struct {
	Config config.Config
	DB     *ember.DB
	Redis  goredis.UniversalClient
	Bus    messaging.Bus
	Server *server.Server
	Ctx    context.Context
}

// Kernel is the runtime: it implements contract.Runtime and owns the global
// tables/interfaces. Built once at boot; injected into every module's Register.
type Kernel struct {
	cfg config.Config
	db  *ember.DB
	rdb goredis.UniversalClient
	bus messaging.Bus
	srv *server.Server
	ctx context.Context

	audit    contract.Audit
	sessions contract.Sessions
}

// New builds the kernel, wiring its kernel-owned table implementations.
func New(deps Deps) (*Kernel, error) {
	if deps.DB == nil {
		return nil, errors.New("kernel: nil DB")
	}
	k := &Kernel{
		cfg: deps.Config, db: deps.DB, rdb: deps.Redis,
		bus: deps.Bus, srv: deps.Server, ctx: deps.Ctx,
	}
	// Wire kernel-owned interfaces over their kernel tables.
	auditWriter := &dbAuditWriter{db: deps.DB}
	k.audit = platformaudit.New(auditWriter)
	k.sessions = &sessionsService{db: deps.DB, cfg: deps.Config}

	return k, nil
}

// RegisterPermissions feeds the RBAC engine the union of all module permissions.
// Called by cmd/api after the registry is populated, before modules Register().
func (k *Kernel) RegisterPermissions(perms []contract.Permission) {
}

// ─── contract.Runtime impl ───────────────────────────────────────

func (k *Kernel) Logger() func(string) {
	return func(s string) {
		k.bus.Publish(k.ctx, messaging.Message{
			Topic:   "log",
			Payload: s,
		})
	}
}
func (k *Kernel) Config() config.Config       { return k.cfg }
func (k *Kernel) DB() contract.DBHandle       { return &dbHandle{db: k.db} }
func (k *Kernel) Redis() contract.RedisHandle { return contract.RedisHandle{RDB: k.rdb} }
func (k *Kernel) Bus() messaging.Bus          { return k.bus }
func (k *Kernel) Audit() contract.Audit       { return k.audit }
func (k *Kernel) Sessions() contract.Sessions { return k.sessions }
func (k *Kernel) HTTPRoot() contract.HTTPRoot { return k.srv.HTTPRoot() }
func (k *Kernel) Context() context.Context    { return k.ctx }

// dbHandle / redisHandle are tiny adapters satisfying contract.DBHandle/RedisHandle.
type dbHandle struct{ db *ember.DB }

func (h *dbHandle) Raw() any                       { return h.db }
func (h *dbHandle) Ping(ctx context.Context) error { return h.db.Ping(ctx) }

// ─── audit_log writer ────────────────────────────────────────────

type dbAuditWriter struct {
	db  *ember.DB
	log func(string)
}

// auditRow is the kernel-owned audit_log shape.
type auditRow struct {
	ID        int64  `ember:"primaryKey;autoIncr"`
	TenantID  *int64 `ember:"column:tenant_id;nullable"`
	ActorID   *int64 `ember:"column:actor_id;nullable"`
	Module    string
	Action    string
	Subject   *string
	Before    []byte `ember:"nullable"`
	After     []byte `ember:"nullable"`
	IP        *string
	UserAgent *string
	Metadata  []byte `ember:"nullable"`
	CreatedAt time.Time
}

func (*auditRow) TableName() string { return "audit_log" }

func (w *dbAuditWriter) Write(ctx context.Context, e contract.AuditEntry) error {
	row := auditRow{Module: e.Module, Action: e.Action}

	if e.ActorID != 0 {
		row.ActorID = &e.ActorID
	}
	if e.Subject != "" {
		row.Subject = &e.Subject
	}
	if e.IP != "" {
		row.IP = &e.IP
	}
	if e.UserAgent != "" {
		row.UserAgent = &e.UserAgent
	}
	row.Before = toJSONB(e.Before)
	row.After = toJSONB(e.After)
	row.Metadata = toJSONBStr(e.Metadata)
	row.CreatedAt = time.Now().UTC()
	if err := w.db.Model().Create(ctx, &row); err != nil {
		w.log("audit write failed, " + "err : " + err.Error() + ", module: " + e.Module + ", action: " + e.Action)
		return fmt.Errorf("kernel: audit write: %w", err)
	}
	return nil
}

func toJSONB(v map[string]any) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}
func toJSONBStr(v map[string]string) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

// ─── sessions service ────────────────────────────────────────────

type sessionsService struct {
	db  *ember.DB
	cfg config.Config
}

// sessionRow mirrors the kernel sessions table.
type sessionRow struct {
	ID               int64  `ember:"primaryKey;autoIncr"`
	TenantID         *int64 `ember:"column:tenant_id;nullable"`
	UserID           int64
	RefreshTokenHash string `ember:"column:refresh_token_hash"`
	IssuedAt         int64
	ExpiresAt        int64
	Revoked          bool
	UserAgent        *string    `ember:"column:user_agent;nullable"`
	IP               *string    `ember:"column:ip;nullable"`
	RevokedAt        *time.Time `ember:"column:revoked_at;nullable"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (*sessionRow) TableName() string { return "sessions" }

func (s *sessionsService) Create(ctx context.Context, in contract.Session) (string, error) {
	// The caller passes the refresh token it issued to the client (in.RefreshToken).
	// We store only its hash; the client keeps the raw token. If the caller didn't
	// supply one, generate one (rare; tests sometimes do this).
	token := in.RefreshToken
	if token == "" {
		token = randomToken(32)
	}
	hash := hashToken(token)
	row := sessionRow{
		UserID: in.UserID, RefreshTokenHash: hash,
		IssuedAt: time.Now().Unix(), ExpiresAt: in.ExpiresAt,
		Revoked: false, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if in.UserAgent != "" {
		row.UserAgent = &in.UserAgent
	}
	if in.IP != "" {
		row.IP = &in.IP
	}
	if err := s.db.Model().Create(ctx, &row); err != nil {
		return "", fmt.Errorf("kernel: create session: %w", err)
	}
	return token, nil
}

func (s *sessionsService) Verify(ctx context.Context, token string) (*contract.Session, error) {
	if token == "" {
		return nil, errors.New("kernel: empty refresh token")
	}
	hash := hashToken(token)
	var row sessionRow
	err := s.db.Model().First(ctx, &row, func(b *ember.Builder) {
		b.Where("refresh_token_hash", "=", hash)
	})
	if err != nil {
		return nil, fmt.Errorf("kernel: session not found: %w", err)
	}
	if row.Revoked {
		return nil, errors.New("kernel: session revoked")
	}
	if row.ExpiresAt < time.Now().Unix() {
		return nil, errors.New("kernel: session expired")
	}
	out := &contract.Session{
		ID: row.ID, UserID: row.UserID, RefreshToken: token,
		IssuedAt: row.IssuedAt, ExpiresAt: row.ExpiresAt, Revoked: row.Revoked,
	}
	return out, nil
}

func (s *sessionsService) Revoke(ctx context.Context, token string) error {
	hash := hashToken(token)
	_, err := s.db.Table("sessions").
		Where("refresh_token_hash", "=", hash).
		Update(ctx, map[string]any{"revoked": true, "revoked_at": time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("kernel: revoke session: %w", err)
	}
	return nil
}

func (s *sessionsService) RevokeUser(ctx context.Context, userID int64) error {
	_, err := s.db.Table("sessions").
		Where("user_id", "=", userID).
		Update(ctx, map[string]any{"revoked": true, "revoked_at": time.Now().UTC()})
	if err != nil {
		return fmt.Errorf("kernel: revoke user sessions: %w", err)
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────

func randomToken(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
