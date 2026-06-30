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
	App   AppConfig
	DB    DBConfig
	Redis RedisConfig
	R2    R2Config

	Messaging MessagingConfig
	raw       map[string]string
}

type AppConfig struct {
	Env              string // development | staging | production | test
	Name             string
	Port             int
	LogLevel         string // debug | info | warn | error
	LogFormat        string // text | json
	JWTSecret        string
	AdminID          string
	BaseURL          string
	TelegramBotToken string
	ResetKeySecret   string
}

type R2Config struct {
	AccountID       string
	AccessKeyID     string
	SecretAccessKey string
	BucketName      string
	PublicURL       string
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

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}
type MessagingConfig struct {
	// Bus is "local" (in-process) or "redis" (Redis Pub/Sub) or "nats".
	Bus string
	// Async turns on the LocalBus worker pool.
	Async bool
	// QueueSize/Workers configure the async LocalBus.
	QueueSize int
	Workers   int
	// ChannelPrefix namespaces Redis/NATS channels.
	ChannelPrefix string
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
			Env:              get(m, "APP_ENV", "development"),
			Name:             get(m, "APP_NAME", "AllyApp"),
			Port:             getInt(m, "APP_PORT", 8080),
			LogLevel:         get(m, "LOG_LEVEL", "info"),
			LogFormat:        get(m, "LOG_FORMAT", "text"),
			JWTSecret:        get(m, "JWT_SECRET", "change-me"),
			AdminID:          get(m, "ADMIN_ID", ""),
			BaseURL:          get(m, "BASE_URL", "http://localhost:8080"),
			TelegramBotToken: get(m, "TELEGRAM_BOT_TOKEN", ""),
			ResetKeySecret:   get(m, "RESET_KEY_SECRET", ""),
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
		Redis: RedisConfig{
			Addr:     get(m, "REDIS_ADDR", "127.0.0.1:6379"),
			Password: get(m, "REDIS_PASSWORD", ""),
			DB:       getInt(m, "REDIS_DB", 0),
		},

		R2: R2Config{
			AccountID:       get(m, "R2_ACCOUNT_ID", ""),
			AccessKeyID:     get(m, "R2_ACCESS_KEY_ID", ""),
			SecretAccessKey: get(m, "R2_SECRET_ACCESS_KEY", ""),
			BucketName:      get(m, "R2_BUCKET_NAME", ""),
			PublicURL:       get(m, "R2_PUBLIC_URL", ""),
		},
		Messaging: MessagingConfig{
			Bus:           get(m, "MESSAGING_BUS", "local"),
			Async:         getBool(m, "MESSAGING_ASYNC", false),
			QueueSize:     getInt(m, "MESSAGING_QUEUE_SIZE", 1024),
			Workers:       getInt(m, "MESSAGING_WORKERS", 4),
			ChannelPrefix: get(m, "MESSAGING_CHANNEL_PREFIX", ""),
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
func getBool(m map[string]string, k string, def bool) bool {
	if v, ok := m[k]; ok && v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
