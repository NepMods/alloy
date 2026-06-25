package counter

import (
	"alloy/internal/app/config"
	counterhttp "alloy/internal/modules/counter/http"
	"alloy/internal/modules/counter/service"
	"alloy/internal/modules/hello_world"
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
		Name:    "Counter",
		Version: "0.0.1",
		Summary: "Helllo world Module",
		Provides: []contract.PortSpec{
			{Name: "CountManager", Iface: ifaceOf[CountManager]()},
		},
		Requires: []contract.PortSpec{
			{Name: "HelloWorldLogger", Iface: ifaceOf[hello_world.HelloWorldLogger]()},
		},
		Permissions: []contract.Permission{
			{Key: "count.manage", Description: "Manage Conuter"},
		},

		Events: []contract.EventSpec{
			{Name: EvenCounterStarted, Direction: contract.EventPublished, Payload: ifaceOf[CounterData]()},
			{Name: EvenCounterDone, Direction: contract.EventPublished, Payload: ifaceOf[CounterData]()},
		},

		Migrations: nil,
	}
}

func (m *Module) Register(reg *contract.Registry, rt contract.Runtime) error {

	_, ok := rt.DB().Raw().(*ember.DB)
	if !ok {
		return ErrKernelDB{"expected *ember.DB from runtime"}
	}

	hwlogger := contract.RequireT[hello_world.HelloWorldLogger](reg)
	svc := service.New(hwlogger)
	reg.Provide(ifaceOf[CountManager](), svc)

	h := counterhttp.New(rt, rt.Context(), *svc)
	rt.HTTPRoot().Mount("counter", func(r contract.Router) {
		h.Mount(r)
	}, func(s string) {
		rt.Logger()(s)
	})
	m.log(m.Manifest().Name + " module registered, " + " version: " + m.Manifest().Version)
	return nil
}

func (m *Module) RouteDocs() []apidocs.RouteDoc {
	return counterhttp.RouteDocs()
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "core: " + e.msg }

// ifaceOf returns the reflect.Type of an interface T (the Elem of *T).
func ifaceOf[T any]() reflect.Type {
	return reflect.TypeOf((*T)(nil)).Elem()
}
