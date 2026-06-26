package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Cache struct {
	namespace string
	rdb       goredis.UniversalClient
}

func New(namespace string, rdb goredis.UniversalClient) *Cache {
	return &Cache{namespace: namespace, rdb: rdb}
}

func (c *Cache) key(k string) string {
	return fmt.Sprintf("cache:%s:%s", c.namespace, k)
}

func (c *Cache) Set(ctx context.Context, key string, val any, ttl time.Duration) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.key(key), data, ttl).Err()
}

func Get[T any](c *Cache, ctx context.Context, key string) (T, error) {
	var zero T
	if c == nil || c.rdb == nil {
		return zero, errors.New("redis unavailable")
	}
	data, err := c.rdb.Get(ctx, c.key(key)).Bytes()
	if err != nil {
		return zero, err
	}
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return zero, err
	}
	return val, nil
}

func (c *Cache) Del(ctx context.Context, key string) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	return c.rdb.Del(ctx, c.key(key)).Err()
}

func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	if c == nil || c.rdb == nil {
		return false, nil
	}
	n, err := c.rdb.Exists(ctx, c.key(key)).Result()
	return n > 0, err
}

func (c *Cache) Flush(ctx context.Context) error {
	if c == nil || c.rdb == nil {
		return nil
	}
	keys, err := c.rdb.Keys(ctx, fmt.Sprintf("cache:%s:*", c.namespace)).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return c.rdb.Del(ctx, keys...).Err()
}
