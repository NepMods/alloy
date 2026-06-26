package usermanager

import (
	"context"
	"fmt"

	"alloy/internal/app/config"
	"alloy/internal/modules/usermanager/migrations"
	"alloy/internal/modules/usermanager/service"
	"alloy/internal/modules/usermanager/store"
	"alloy/internal/platform/messaging"
	"alloy/models/auth"
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
	db, ok := rt.DB().Raw().(*ember.DB)
	if !ok {
		return ErrKernelDB{"expected *ember.DB from runtime"}
	}

	migrator := ember.NewMigrator(db, migrations.EmberMigrations()...)
	if err := migrator.Run(rt.Context()); err != nil {
		return fmt.Errorf("usermanager: run migrations: %w", err)
	}

	m.svc = service.New() // modgen:auto
	ProvideUserManager(reg, m.svc) // modgen:auto
	m.svc.BaseService.Runtime = rt // modgen:auto
	m.svc.BaseService.Cache = cache.New("UserManager", rt.Redis().RDB) // modgen:auto
	m.svc.Store = store.New(db, rt.Context())

	bus := rt.Bus()
	_, err := bus.Subscribe("auth.user.logged_in", func(ctx context.Context, msg messaging.Message) error {
		evt, ok := msg.Payload.(auth.LoginEvent)
		if !ok {
			return fmt.Errorf("unexpected payload type: %T", msg.Payload)
		}
		bus.Publish(context.Background(), messaging.Message{
			Topic:   "log.info",
			Payload: fmt.Sprintf("email notification to %s <%s>", evt.Name, evt.Email),
		})
		return nil
	})
	if err != nil {
		return fmt.Errorf("usermanager: subscribe to auth.user.logged_in: %w", err)
	}

	m.log(m.Manifest().Name + " module registered, version: " + m.Manifest().Version)
	return nil
}

func (m *Module) RequirementRegister(reg *contract.Registry, rt contract.Runtime) error {
	m.svc.BaseService = service.NewBaseService(rt) // modgen:auto
	m.svc.BaseService.Cache = cache.New("UserManager", rt.Redis().RDB) // modgen:auto
	return nil // modgen:auto
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "usermanager: " + e.msg }
