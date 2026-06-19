package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config is the fully-resolved application configuration.
type Config struct {
	App AppConfig
	DB  DBConfig
	raw map[string]string
}

type AppConfig struct {
	Env       string // development | staging | production | test
	Name      string
	Port      int
	LogLevel  string // debug | info | warn | error
	LogFormat string // text | json
}

type DBConfig struct {
	Driver   string
	DSN      string
	Replicas []string
	Pool     PoolConfig
}

type PoolConfig struct {
	MaxOpen         int
	MaxIdle         int
	ConnMaxLifetime time.Duration
	ConnMaxIdle     time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()
	return parse(os.Environ())
}

func parse(env []string) (Config, error) {
	m := map[string]string{}
	for _, kv := range env {
		i := strings.IndexByte(kv, '=')
		if i < 0 {
			continue
		}
		m[kv[:i]] = kv[i+1:]
	}

	cfg := Config{
		App: AppConfig{
			Env:       get(m, "APP_ENV", "development"),
			Name:      get(m, "APP_NAME", "AllyApp"),
			Port:      getInt(m, "APP_PORT", 8080),
			LogLevel:  get(m, "LOG_LEVEL", "info"),
			LogFormat: get(m, "LOG_FORMAT", "text"),
		},
		DB: DBConfig{
			Driver:   get(m, "DB_DRIVER", "sqlite"),
			DSN:      get(m, "DB_DSN", "local.db"),
			Replicas: csv(get(m, "DB_REPLICAS", "")),
			Pool: PoolConfig{
				MaxOpen:         getInt(m, "DB_POOL_MAX_OPEN", 25),
				MaxIdle:         getInt(m, "DB_POOL_MAX_IDLE", 10),
				ConnMaxLifetime: getDuration(m, "DB_POOL_CONN_MAX_LIFETIME", 30*time.Minute),
				ConnMaxIdle:     getDuration(m, "DB_POOL_CONN_MAX_IDLE", 15*time.Minute),
			},
		},
	}
	return cfg, nil
}

func get(m map[string]string, k, def string) string {
	if v, ok := m[k]; ok && v != "" {
		return v
	}
	return def
}
func getInt(m map[string]string, k string, def int) int {
	if v, ok := m[k]; ok && v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func csv(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func getDuration(m map[string]string, k string, def time.Duration) time.Duration {
	if v, ok := m[k]; ok && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
