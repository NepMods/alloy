package app

import (
	"alloy/internal/app/contract"
	"alloy/internal/platform/messaging"

	server "alloy/models/server"

	"github.com/NepMods/ember"
	goredis "github.com/redis/go-redis/v9"
)

type App struct {
	Log      contract.Logger
	DB       *ember.DB
	Redis    goredis.UniversalClient
	Bus      messaging.Bus
	Registry *contract.Registry
	Kernel   contract.Runtime
	Server   *server.Server
}

func (a *App) Close() error {
	var first error
	if a.Bus != nil {
		if err := a.Bus.Close(); err != nil && first == nil {
			first = err
		}
	}
	if a.Redis != nil {
		if err := a.Redis.Close(); err != nil && first == nil {
			first = err
		}
	}
	if a.DB != nil {
		if err := a.DB.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
