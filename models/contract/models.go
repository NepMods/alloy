package contract

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
)

// Module is implemented by every bounded context.
type Module interface {
	Manifest() Manifest
	Log() func(string)
	Register(reg *Registry, rt Runtime) error
}

// Manifest is a module's contract with the rest of the system.
type Manifest struct {
	Name        string       // unique id, also route prefix (/v1/<name>) + migration namespace
	Version     string       // semver of THIS MODULE'S contract (its Provides/Requires)
	Summary     string       // one-line description
	Provides    []PortSpec   // interfaces this module exports
	Requires    []PortSpec   // interfaces this module consumes
	Permissions []Permission // RBAC keys this module declares
	Events      []EventSpec  // events it publishes / subscribes to
	Migrations  []MigrationSpec
}

// PortSpec describes a port interface (a contract boundary).
type PortSpec struct {
	Name       string       // human label, e.g. "LedgerReader"
	Iface      reflect.Type // reflect.TypeOf((*YourIface)(nil)).Elem()
	MinVersion string       // optional: minimum provider contract version
}

// Registry holds all loaded modules, resolves Requires → Provides, and stores
// the concrete implementations registered during Register(). It is the DI spine.
type Registry struct {
	modules []Module
	byName  map[string]Module
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		byName: map[string]Module{},
	}
}

// RegisterModule adds a module.
func (r *Registry) RegisterModule(m Module) error {
	mf := m.Manifest()

	m.Log()(fmt.Sprintf("registering module: %s, version: %s", mf.Name, mf.Version))
	if mf.Name == "" {
		return fmt.Errorf("contract: module with empty Name")
	}
	if _, dup := r.byName[mf.Name]; dup {
		return fmt.Errorf("contract: duplicate module name %q", mf.Name)
	}

	r.modules = append(r.modules, m)
	r.byName[mf.Name] = m
	m.Log()(fmt.Sprintf("module %s registered successfully", mf.Name))
	return nil
}

type Runtime interface {
	Logger() func(string)
	Config() ConfigSnapshot
	DB() DBHandle       // the global ac_orm pool, for a module's OWN tables only
	HTTPRoot() HTTPRoot // mount the module's routes here
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

// DBHandle exposes the global DB pool. ac_orm's *DB is passed as-is via DB();
// modules cast and use it for their own tables. The contract package avoids a
// hard dep on ac_orm by passing it as any — the kernel's concrete Runtime
// returns the typed *ac_orm.DB.
type DBHandle interface {
	// Raw returns the underlying ac_orm *DB (typed as any to avoid import here).
	// Modules cast: orm := rt.DB().Raw().(*ac_orm.DB)
	Raw() any
	// Ping verifies connectivity.
	Ping(ctx context.Context) error
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

// Permission is an RBAC capability declared by a module.
type Permission struct {
	Key          string // "<module>.<resource>.<action>"
	Description  string
	DefaultRoles []string // roles that get this by default on a new tenant
}

// EventSpec documents a pub/sub relationship.
type EventSpec struct {
	Name      string         // dotted event name, e.g. "sales.invoice.issued"
	Direction EventDirection // published | subscribed
	Payload   reflect.Type   // Go payload type carried
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

// MigrationSpec is a namespaced migration owned by a module. The kernel prefixes
// Version with the module name at run time. Up/Down receive the kernel's
// migration surface (an opaque handle the kernel interprets as its ac_orm
// *Schema). This indirection keeps manifests free of ac_orm imports.
type MigrationSpec struct {
	Version     string
	Description string
	Up          func(ctx context.Context, schema MigrationSchema) error
	Down        func(ctx context.Context, schema MigrationSchema) error
}

// MigrationSchema is the migration surface a module sees. The kernel supplies a
// concrete value whose methods mirror ac_orm's Blueprint DSL. Modules call
// schema.Create("table", func(b Blueprint){...}) etc.
type MigrationSchema interface {
	Create(table string, fn func(b Blueprint)) error
	Table(table string, fn func(b Blueprint)) error
	Drop(table string) error
	DropIfExists(table string) error
	HasTable(table string) (bool, error)
	HasColumn(table, column string) (bool, error)
	Raw(sql string) error
}

// Blueprint is the column-DSL surface during a migration. It mirrors the subset
// of ac_orm's Blueprint that modules need; the kernel bridges each call to the
// real ac_orm.Blueprint.
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
