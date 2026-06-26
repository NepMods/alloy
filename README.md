# Alloy

A modular, event-driven Go web API backend with code generation, pluggable storage, and an optional terminal UI dashboard.

Built as a modular monolith — each bounded context is a self-contained module with its own service layer, data store, migrations, HTTP handlers, and test harness.

## Prerequisites

- **Go 1.26+**
- **MySQL** (or PostgreSQL/SQLite — ember ORM supports all three)
- **Redis** (optional — degrades gracefully for caching and pub/sub; local in-process bus works without it)

## Quick Start

```bash
# 1. Clone and enter the repo
git clone https://github.com/NepMods/alloy.git
cd alloy

# 2. Copy the example env and edit for your setup
cp .env.example .env

# 3. Run (starts HTTP server on :8080 with TUI dashboard)
go run ./cmd/api/
```

Point your browser to `http://localhost:8080/health` to verify.

## Configuration

All configuration is through environment variables (`.env` file):

| Variable | Default | Description |
|---|---|---|
| `APP_ENV` | `development` | Environment name |
| `APP_NAME` | `alloy` | Application name |
| `APP_PORT` | `8080` | HTTP listen port |
| `LOG_LEVEL` | `debug` | Log level (debug, info, warn, error) |
| `LOG_FORMAT` | `text` | Log format (text, json) |
| `DB_DRIVER` | `mysql` | Database driver (mysql, postgres, sqlite3) |
| `DB_DSN` | — | Database connection string |
| `DB_POOL_MAX_OPEN` | `25` | Max open connections |
| `DB_POOL_MAX_IDLE` | `10` | Max idle connections |
| `REDIS_ADDR` | — | Redis address (omit to disable Redis) |
| `REDIS_PASSWORD` | — | Redis password |
| `MESSAGING_BUS` | `local` | Bus backend (local, redis) |
| `MESSAGING_ASYNC` | `true` | Process messages asynchronously |

A full reference is at `internal/app/config/config.go`.

## Running

### With TUI (default)

```bash
go run ./cmd/api/
```

Opens a four-pane terminal dashboard: Logs, Server Logs, Server Info, API Docs.
Navigate with `Tab`, scroll with `PgUp`/`PgDn`, copy pane content with `c`, quit with `q`.

### Without TUI

```bash
go run ./cmd/api/ --no-tui
```

### Flags

| Flag | Description |
|---|---|
| `--no-tui` | Run without terminal UI (plain stdout) |
| `--no-verbose` | Suppress verbose log output |
| `--zero-logs` | Suppress all log output |
| `--max-cpu-cores` | Limit CPU cores (e.g. `2`) |
| `--max-ram-usage` | Limit memory (e.g. `512MB`, `2GB`) |

## Module Development with `modgen`

`modgen` is a code generation tool that creates and manages modules.

### Build

```bash
go build -o modgen ./cmd/modgen/
```

### Commands

**Create a new module:**

```bash
./modgen new <name>
```

Scaffolds a complete module with:
- `module.json` — manifest (name, version, provides, requires, events, HTTP mount)
- `module.go` — module registration with `Register` and `RequirementRegister`
- `service/` — service layer with BaseService, constructor, interface stubs
- `http/` — HTTP handlers and route docs
- `store/` — data access layer with ember ORM
- `models/` — database models
- `migrations/` — schema migrations
- `tests/` — test harness and fakes

**Delete a module:**

```bash
./modgen delete <name>
```

Removes the module directory and regenerates the boot module list.

**Regenerate generated files:**

```bash
./modgen
```

Without arguments, `modgen` reads all modules in `internal/modules/` and regenerates:
- `module_gen.go` for each module — manifest, `Provide*()`, `Require*()` functions
- `service/base_gen.go` — `BaseService` struct with `Runtime`, `Cache`, and dependency fields
- `service/service_gen.go` — `New()` constructor
- `internal/app/boot/modules_gen.go` — dependency-sorted module list

**Add scaffold components to existing module:**

```bash
./modgen <name>:<component>
```

Components: `domain`, `dto`, `fakes`, `migrations`, `models`, `store`, `testharness`, `test`, `all`

### Module Lifecycle

Modules follow a two-phase initialization:

1. **`RequirementRegister`** — resolves dependencies from other modules (injected into `BaseService`)
2. **`Register`** — sets up runtime, cache, HTTP routes, and event subscriptions

Dependencies are declared in `module.json` as `requires` and resolved via `contract.Registry`.

## Project Structure

```
├── cmd/
│   ├── api/              # Application entry point
│   └── modgen/           # Code generation tool
├── internal/
│   ├── app/
│   │   ├── boot/         # Application bootstrap & wiring
│   │   └── config/       # Configuration loader
│   ├── modules/
│   │   ├── auth/         # Authentication module
│   │   └── usermanager/  # User management module
│   ├── platform/
│   │   ├── cache/        # Redis-backed cache
│   │   ├── kernel/       # Runtime implementation
│   │   ├── messaging/    # Pub/sub bus (local + Redis)
│   │   ├── alloy/        # HTTP JSON helpers
│   │   ├── db/           # Database connection
│   │   ├── redis/        # Redis connection
│   │   └── audit/        # Audit logging
│   ├── server/           # Server documentation
│   └── tui/              # Terminal UI dashboard
├── models/               # Shared interfaces & types (ports)
│   ├── app/              # App composition root
│   ├── contract/         # Module, Registry, Runtime contracts
│   ├── auth/             # Auth service interface + events
│   ├── usermanager/      # UserManager interface + User model
│   ├── server/           # HTTP server model
│   └── apidocs/          # API documentation model
└── module_jsons/         # Reference module definitions
```

## Architecture

- **Modular Monolith** — bounded contexts as self-contained modules under `internal/modules/`
- **Ports & Adapters** — modules depend on interfaces in `models/`, not on other modules directly
- **Dependency Injection** — `contract.Registry` with `Provide()`/`RequireT()` for wiring
- **Event-Driven** — cross-module communication via pub/sub `Bus` (local in-process or Redis)
- **Code Generation** — `modgen` scaffolds modules, generates manifests, base services, and boot wiring
- **Pluggable Storage** — SQLite, MySQL, PostgreSQL via ember ORM
- **Optional TUI** — same binary can run headless or with a Bubble Tea terminal UI

## Testing

```bash
# Run all tests
go test ./...

# Run tests for a specific module
go test ./internal/modules/auth/...
go test ./internal/modules/usermanager/...

# With coverage
go test -cover ./...
```

Modules include test harnesses and fakes for isolated testing.
