package boot

import (
	"alloy/internal/app/config"
	platformdb "alloy/internal/platform/db"
	"alloy/models/app"
	server "alloy/models/server"
	"context"
)

func Build(ctx context.Context, cfg config.Config, log func(string)) (*app.App, error) {
	db, err := platformdb.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	srv := server.New(server.Deps{
		Config: cfg, Logger: log,
		DBPing: func(c context.Context) error { return db.Ping(c) },
	})

	return &app.App{
		Cfg: cfg, Log: log, DB: db, Server: srv,
	}, nil
}
