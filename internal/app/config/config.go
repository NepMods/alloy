package config

import "time"

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
