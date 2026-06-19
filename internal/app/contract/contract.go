package contract

import (
	"alloy/internal/platform/messaging"
	"context"
	"net/http"
	"reflect"
)

type Module interface {
	Manifest() Manifest

	Register(reg *Registry, rt Runtime) error
}

// Manifest is a module's contract with the rest of the system.
type Manifest struct {
	Name        string
	Version     string
	Summary     string
	Provides    []PortSpec
	Requires    []PortSpec
	Permissions []Permission
	Events      []EventSpec
	Migrations  []MigrationSpec
}

// PortSpec describes a port interface (a contract boundary).
type PortSpec struct {
	Name       string
	Iface      reflect.Type
	MinVersion string
}

// Permission is an RBAC capability declared by a module.
type Permission struct {
	Key          string
	Description  string
	DefaultRoles []string
}

// EventSpec documents a pub/sub relationship.
type EventSpec struct {
	Name      string
	Direction EventDirection
	Payload   reflect.Type
}

// EventDirection is whether a module publishes or subscribes.
type EventDirection int

const (
	EventPublished EventDirection = iota
	EventSubscribed
)

func (d EventDirection) String() string {
	if d == EventSubscribed {
		return "subscribes"
	}
	return "publishes"
}

type MigrationSpec struct {
	Version     string
	Description string
	Up          func(ctx context.Context, schema MigrationSchema) error
	Down        func(ctx context.Context, schema MigrationSchema) error
}

type MigrationSchema interface {
	Create(table string, fn func(b Blueprint)) error
	Table(table string, fn func(b Blueprint)) error
	Drop(table string) error
	DropIfExists(table string) error
	HasTable(table string) (bool, error)
	HasColumn(table, column string) (bool, error)
	Raw(sql string) error
}

type Blueprint interface {
	ID() ColumnDef
	UUID(name ...string) ColumnDef
	String(name string, length ...int) ColumnDef
	Text(name string) ColumnDef
	LongText(name string) ColumnDef
	Integer(name string) ColumnDef
	BigInteger(name string) ColumnDef
	UnsignedBigInteger(name string) ColumnDef
	Decimal(name string, precision, scale int) ColumnDef
	Float(name string) ColumnDef
	Double(name string) ColumnDef
	Boolean(name string) ColumnDef
	Date(name string) ColumnDef
	DateTime(name string) ColumnDef
	Timestamp(name string) ColumnDef
	TimestampTz(name string) ColumnDef
	Time(name string) ColumnDef
	JSON(name string) ColumnDef
	JSONB(name string) ColumnDef
	Binary(name string) ColumnDef
	Enum(name string, values []string) ColumnDef

	Timestamps()
	SoftDeletes()
	NullableMorphs(name string)

	Index(columns ...string) IndexDef
	UniqueIndex(columns ...string) IndexDef
	Primary(columns ...string) IndexDef
	DropIndex(name string)
	Foreign(column string) ForeignKeyDef
	DropColumn(names ...string)
}

// ColumnDef is the chainable column modifier surface.
type ColumnDef interface {
	Nullable() ColumnDef
	Default(v any) ColumnDef
	Unique() ColumnDef
	Comment(s string) ColumnDef
	AutoIncrement() ColumnDef
	Unsigned() ColumnDef
	After(col string) ColumnDef
}

// IndexDef is the chainable index modifier surface.
type IndexDef interface {
	Name(name string) IndexDef
	Algorithm(a string) IndexDef
}

// ForeignKeyDef is the chainable FK surface.
type ForeignKeyDef interface {
	References(col string) ForeignKeyDef
	On(table string) ForeignKeyDef
	OnDelete(action string) ForeignKeyDef
	OnUpdate(action string) ForeignKeyDef
	CascadeOnDelete() ForeignKeyDef
	RestrictOnDelete() ForeignKeyDef
	NullOnDelete() ForeignKeyDef
	Name(name string) ForeignKeyDef
}

type Registry struct {
	modules  []Module
	byName   map[string]Module
	provided map[reflect.Type]Module // iface → providing module
	impl     map[reflect.Type]any    // iface → concrete impl (set in Register)
}

type Runtime interface {
	Logger() Logger
	Config() ConfigSnapshot
	DB() DBHandle
	Redis() RedisHandle
	Bus() messaging.Bus
	Audit() Audit
	Sessions() Sessions
	HTTPRoot() HTTPRoot
}

// Logger is the structured logger every module uses. Mirrors slog levels.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

// ConfigSnapshot is a read-only view of the loaded configuration.
type ConfigSnapshot interface {
	AppEnv() string
	AppName() string
	HTTPPort() int
	Get(key string) any // arbitrary key lookup
	GetString(key string) string
	GetInt(key string) int
	GetDuration(key string) (any, error)
}
type DBHandle interface {
	// Raw returns the underlying ac_orm *DB (typed as any to avoid import here).
	// Modules cast: orm := rt.DB().Raw().(*ac_orm.DB)
	Raw() any
	// Ping verifies connectivity.
	Ping(ctx context.Context) error
}

// RedisHandle exposes the global redis client.
type RedisHandle interface {
	Raw() any // *redis.Client or redis.UniversalClient
	Ping(ctx context.Context) error
}

// Audit writes to the kernel-owned, immutable audit_log. Modules MUST use this
// (never their own table) for any security-relevant mutation record.
type Audit interface {
	Record(ctx context.Context, entry AuditEntry) error
}

// AuditEntry is the payload for an audit record.
type AuditEntry struct {
	TenantID  int64
	ActorID   int64          // user id, 0 for system
	Action    string         // e.g. "journal.post"
	Module    string         // module originating the action
	Subject   string         // human label of what changed
	Before    map[string]any // snapshot pre-change (nil for creates)
	After     map[string]any // snapshot post-change (nil for deletes)
	IP        string
	UserAgent string
	Metadata  map[string]string
}

// Sessions manages kernel-owned refresh sessions.
type Sessions interface {
	Create(ctx context.Context, s Session) (string, error)
	Verify(ctx context.Context, token string) (*Session, error)
	Revoke(ctx context.Context, token string) error
	RevokeUser(ctx context.Context, userID int64) error
}

// Session is the kernel-owned session record.
type Session struct {
	ID           int64
	TenantID     int64
	UserID       int64
	RefreshToken string
	IssuedAt     int64
	ExpiresAt    int64
	UserAgent    string
	IP           string
	Revoked      bool
}

// HTTPRoot is where modules mount their sub-router.
type HTTPRoot interface {
	// Mount registers a chi.Router under /v1/<module>. Implementations also
	// record the mount for /healthz introspection.
	Mount(module string, build func(router Router))
	// AddTenantMiddleware registers middleware that runs on every module
	// sub-router (for tenant resolution, auth, etc.). Must be called before
	// any Mount calls.
	AddTenantMiddleware(mw func(http.Handler) http.Handler)
	Router() Router
}

// Router is the minimal chi-like surface modules use to declare routes. The
// concrete type is *chi.Mux; the kernel adapts it.
type Router interface {
	Get(pattern string, h any)
	Post(pattern string, h any)
	Put(pattern string, h any)
	Patch(pattern string, h any)
	Delete(pattern string, h any)
	Route(pattern string, fn func(r Router)) Router
	Group(fn func(r Router)) Router
	Use(mw ...any)
}

// Message is re-exported from messaging for module convenience.
type Message = messaging.Message
