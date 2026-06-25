package redis

import (
	"alloy/internal/app/config"
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Open builds and pings a redis client.
func Open(ctx context.Context, cfg config.Config) (redis.UniversalClient, error) {
	cli := redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Addr,
		Password:     cfg.Redis.Password,
		DB:           cfg.Redis.DB,
		PoolSize:     20,
		MinIdleConns: 4,
	})
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := cli.Ping(pingCtx).Err(); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("redis: ping: %w", err)
	}
	return cli, nil
}
