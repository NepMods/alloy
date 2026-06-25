package auth

import (
	"alloy/internal/app/config"
	"alloy/models/apidocs"
	"alloy/models/contract"
	"reflect"

	authhttp "alloy/internal/modules/auth/http"

	"github.com/NepMods/ember"
	"github.com/aws/smithy-go/auth"
)

type Module struct {
	cfg config.Config
	log func(string)
}

func New(cfg config.Config, log func(string)) *Module { return &Module{cfg: cfg, log: log} }

func (m *Module) Manifest() contract.Manifest {
	return contract.Manifest{
		Name:    "Auth",
		Version: "0.1.0",
		Summary: "Authentication and authorization module.",
		Provides: []contract.PortSpec{
			{Name: "IdentityResolver", Iface: ifaceOf[auth.IdentityResolver]()},
			{Name: "MembershipResolver", Iface: ifaceOf[auth.MembershipResolver]()},
		},
	}
}

// Register wires the module's store→service→http and mounts routes. It receives
// the kernel Runtime (global infra) and the Registry (for resolving Requires).
func (m *Module) Register(reg *contract.Registry, rt contract.Runtime) error {
	// The global DB pool, typed. Modules cast the opaque handle to *ember.DB.
	_, ok := rt.DB().Raw().(*ember.DB)
	if !ok {
		return ErrKernelDB{"expected *ember.DB from runtime"}
	}

	h := authhttp.New()
	rt.HTTPRoot().Mount("auth", func(r contract.Router) {
		h.Mount(r)
	})
	m.log(m.Manifest().Name + " module registered, " + " version: " + m.Manifest().Version)
	return nil
}

func (m *Module) RouteDocs() []apidocs.RouteDoc {
	return authhttp.RouteDocs()
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "core: " + e.msg }

// ifaceOf returns the reflect.Type of an interface T (the Elem of *T).
func ifaceOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
