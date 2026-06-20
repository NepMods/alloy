package boot

import (
	"alloy/internal/app/config"
	"alloy/internal/modules/auth"
	platformdb "alloy/internal/platform/db"
	"alloy/models/app"
	"alloy/models/contract"
	server "alloy/models/server"
	"context"
	"fmt"
)

func Modules(cfg config.Config, log func(string)) []contract.Module {
	return []contract.Module{
		auth.New(cfg, log),
	}
}

func Build(ctx context.Context, cfg config.Config, log func(string)) (*app.App, error) {
	db, err := platformdb.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	reg := contract.NewRegistry()
	for _, m := range Modules(cfg, log) {
		if err := reg.RegisterModule(m); err != nil {
			log(fmt.Sprintf("register module %s: %v", m.Manifest().Name, err))
			return nil, fmt.Errorf("boot: register module %s: %w", m.Manifest().Name, err)
		}
	}

	srv := server.New(server.Deps{
		Config: cfg, Logger: log,
		DBPing: func(c context.Context) error { return db.Ping(c) },
	})

	return &app.App{
		Cfg: cfg, Log: log, DB: db, Server: srv,
	}, nil
}
