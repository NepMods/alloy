package app

import (
	"alloy/internal/app/config"
	"context"
	"strconv"

	server "alloy/models/server"

	"github.com/NepMods/ember"
)

type App struct {
	Cfg    config.Config
	Log    func(string)
	DB     *ember.DB
	Server *server.Server
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
func (a *App) Run(ctx context.Context, log func(string)) error {
	log("starting http server: " + "port: " + strconv.Itoa(a.Cfg.App.Port))
	return nil
}
