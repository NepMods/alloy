package db

import (
	"alloy/internal/app/config"
	"context"
	"fmt"
	"time"

	"github.com/NepMods/ember"
)

// Open builds the global ac_orm.DB from config and pings it.
func Open(ctx context.Context, cfg config.Config) (*ember.DB, error) {

	ormCfg := ember.Config{
		Driver:   cfg.DB.Driver,
		Master:   cfg.DB.DSN,
		Replicas: cfg.DB.Replicas,
		Pool: ember.PoolConfig{
			MaxOpenConns:    cfg.DB.Pool.MaxOpen,
			MaxIdleConns:    cfg.DB.Pool.MaxIdle,
			ConnMaxLifetime: cfg.DB.Pool.ConnMaxLifetime,
			ConnMaxIdleTime: cfg.DB.Pool.ConnMaxIdle,
		},
	}

	ddb, err := ember.Open(ormCfg)
	if err != nil {
		return nil, fmt.Errorf("db: open ember: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ddb.Ping(pingCtx); err != nil {
		_ = ddb.Close()
		return nil, fmt.Errorf("db: ping master: %w", err)
	}
	return ddb, nil
}
