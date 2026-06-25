package app

import (
	"alloy/internal/app/config"
	"alloy/internal/platform/messaging"
	"context"
	"strconv"

	"alloy/models/contract"
	server "alloy/models/server"

	"github.com/NepMods/ember"

	goredis "github.com/redis/go-redis/v9"
)

type App struct {
	Cfg   config.Config
	Log   func(string)
	DB    *ember.DB
	Redis goredis.UniversalClient
	Bus   messaging.Bus

	Kernel   contract.Runtime
	Registry *contract.Registry
	Server   *server.Server
}

func (a *App) Close() error {
	var first error
	if a.DB != nil {
		if err := a.DB.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
func (a *App) Run(ctx context.Context) error {
	a.Log("starting http server: " + "port: " + strconv.Itoa(a.Cfg.App.Port))
	return a.Server.Run(ctx)
}
