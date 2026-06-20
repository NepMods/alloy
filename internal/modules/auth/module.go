package auth

import (
	"alloy/internal/app/config"
	"alloy/models/contract"

	"github.com/NepMods/ember"
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

	m.log(m.Manifest().Name + " module registered, " + " version: " + m.Manifest().Version)
	return nil
}

func (m *Module) Log() func(string) { return m.log }

type ErrKernelDB struct{ msg string }

func (e ErrKernelDB) Error() string { return "core: " + e.msg }
