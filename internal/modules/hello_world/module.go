package hello_world

import (
	"alloy/internal/app/config"
	helloworldhttp "alloy/internal/modules/hello_world/http"
	"alloy/models/apidocs"
	"alloy/models/contract"
	"reflect"

	"github.com/NepMods/ember"
)

type Module struct {
	cfg config.Config
	log func(string)
}

func New(cfg config.Config, log func(string)) *Module { return &Module{cfg: cfg, log: log} }

func (m *Module) Manifest() contract.Manifest {
	return contract.Manifest{
		Name:    "HelloWorld",
		Version: "0.0.1",
		Summary: "Helllo world Module",
		Provides: []contract.PortSpec{
			{Name: "HelloWorldLogger", Iface: ifaceOf[HelloWorldLogger]()},
		},
		Requires: nil, // auth depends only on kernel-owned interfaces (always available)

		Permissions: []contract.Permission{
			{Key: "helloworld.log", Description: "Log Hello world and get back a random number"},
		},

		Events: []contract.EventSpec{
			{Name: EvenHelloWorldLogStarted, Direction: contract.EventPublished, Payload: ifaceOf[HelloWoldData]()},
			{Name: EvenHelloWorldLogDone, Direction: contract.EventPublished, Payload: ifaceOf[HelloWoldData]()},
		},

		Migrations: nil,
	}
}

func (m *Module) Register(reg *contract.Registry, rt contract.Runtime) error {
	_, ok := rt.DB().Raw().(*ember.DB)
	if !ok {
		return ErrKernelDB{"expected *ember.DB from runtime"}
	}

	h := helloworldhttp.New(rt, rt.Context())
	rt.HTTPRoot().Mount("auth", func(r contract.Router) {
		h.Mount(r)
	})
	m.log(m.Manifest().Name + " module registered, " + " version: " + m.Manifest().Version)
	return nil
}

func (m *Module) RouteDocs() []apidocs.RouteDoc {
	return helloworldhttp.RouteDocs()
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "core: " + e.msg }

// ifaceOf returns the reflect.Type of an interface T (the Elem of *T).
func ifaceOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
