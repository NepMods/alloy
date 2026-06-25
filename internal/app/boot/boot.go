package boot

import (
	"alloy/internal/app/config"
	"alloy/internal/modules/hello_world"
	platformdb "alloy/internal/platform/db"
	"alloy/internal/platform/kernel"
	"alloy/internal/platform/messaging"
	platformredis "alloy/internal/platform/redis"
	"alloy/models/app"
	"alloy/models/contract"
	server "alloy/models/server"
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

func Modules(cfg config.Config, log func(string)) []contract.Module {
	return []contract.Module{
		hello_world.New(cfg, log),
	}
}

func Build(ctx context.Context, cfg config.Config, log func(string)) (*app.App, error) {
	db, err := platformdb.Open(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var rdb goredis.UniversalClient
	// Redis is optional in some local-dev configs; don't fail the whole boot.
	r, rerr := platformredis.Open(ctx, cfg)
	if rerr != nil {
		log("redis unavailable; continuing without it" + ", err: " + rerr.Error())
	} else {
		log("redis Connected")
		r.Set(ctx, "APP_RUNNING", cfg.App.Name, 10000)
		log(r.Get(ctx, "APP_RUNNING").String())
		rdb = r
	}

	// Messaging bus.
	bus := buildBus(cfg, log)

	bus.Subscribe("log.server", func(ctx context.Context, msg messaging.Message) error {
		log("BUS (log.server): " + msg.Payload.(string))
		return nil
	})

	go func() {
		// 1. Wait for 2 seconds in the background
		time.Sleep(2 * time.Second)

		// 2. Publish the message
		bus.Publish(ctx, messaging.Message{
			Topic:      "log.info",
			Payload:    "The Messaging Bus was just woken up",
			OccurredAt: time.Now(),
			TraceID:    "none",
		})
	}()

	// HTTP server (holds the HTTPRoot modules mount onto).
	srv := server.New(server.Deps{
		Config: cfg, Logger: log, Redis: rdb,
		DBPing: func(c context.Context) error { return db.Ping(c) },
	})
	k, err := kernel.New(kernel.Deps{
		Config: cfg, DB: db, Redis: rdb, Bus: bus, Server: srv, Ctx: ctx,
	})
	reg := contract.NewRegistry()
	for _, m := range Modules(cfg, log) {
		if err := reg.RegisterModule(m); err != nil {
			bus.Publish(ctx, messaging.Message{
				Topic:   "log.server",
				Payload: "[Registering Modules] Error Registering Module: " + m.Manifest().Name,
			})
			return nil, fmt.Errorf("boot: register module %s: %w", m.Manifest().Name, err)
		}
	}

	return &app.App{
		Cfg: cfg, Log: log, DB: db, Server: srv, Redis: rdb, Bus: bus, Registry: reg, Kernel: k,
	}, nil
}

// buildBus constructs the pub/sub bus from config.
func buildBus(cfg config.Config, log func(string)) messaging.Bus {
	switch cfg.Messaging.Bus {
	case "redis":
		// RedisBus needs the redis client, opened in Build(). We construct a
		// LocalBus here and swap in Redis once the client is available; simplest
		// is to default to local unless explicitly configured with a reachable
		// Redis. For now, local is the safe default for the monolith.
		log("messaging bus: local (redis bus wired in cmd when client ready) " + ", configured : " + cfg.Messaging.Bus)
		return messaging.NewLocalBus(localBusOpts(cfg)...)
	case "nats":
		log("messaging bus: nats not yet configured; using local" + " configured: " + cfg.Messaging.Bus)
		return messaging.NewLocalBus(localBusOpts(cfg)...)
	default:

		return messaging.NewLocalBus(localBusOpts(cfg)...)
	}
}

func localBusOpts(cfg config.Config) []messaging.LocalBusOption {
	var opts []messaging.LocalBusOption
	if cfg.Messaging.Async {
		opts = append(opts, messaging.WithAsync(cfg.Messaging.QueueSize, cfg.Messaging.Workers))
	}
	return opts
}
