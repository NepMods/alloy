package auth

import (
	"alloy/internal/app/config"
	authhttp "alloy/internal/modules/auth/http"
	"alloy/internal/modules/auth/service"
	"alloy/models/contract"

	"github.com/NepMods/ember"
	"alloy/internal/platform/cache"
)

type Module struct {
	cfg config.Config
	log func(string)
	svc *service.Service
}

func New(cfg config.Config, log func(string)) *Module {
	return &Module{cfg: cfg, log: log}
}

func (m *Module) Register(reg *contract.Registry, rt contract.Runtime) error {
	_, ok := rt.DB().Raw().(*ember.DB)
	if !ok {
		return ErrKernelDB{"expected *ember.DB from runtime"}
	}

	m.svc = service.New() // modgen:auto
	ProvideAuth(reg, m.svc) // modgen:auto
	m.svc.BaseService.Runtime = rt // modgen:auto
	m.svc.BaseService.Cache = cache.New("Auth", rt.Redis().RDB) // modgen:auto
	m.svc.Bus = rt.Bus()
	h := authhttp.New(rt, rt.Context(), m.svc)
	rt.HTTPRoot().Mount("auth", func(r contract.Router) {
		h.Mount(r)
	}, func(s string) {
		rt.Logger()(s)
	})
	m.log(m.Manifest().Name + " module registered, version: " + m.Manifest().Version)
	return nil
}

func (m *Module) RequirementRegister(reg *contract.Registry, rt contract.Runtime) error {
	m.svc.BaseService = service.NewBaseService(rt, RequireUserManager(reg)) // modgen:auto
	m.svc.BaseService.Cache = cache.New("Auth", rt.Redis().RDB) // modgen:auto
	return nil // modgen:auto
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "auth: " + e.msg }
