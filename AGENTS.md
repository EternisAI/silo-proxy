# AGENTS.md

Guidance for coding agents working in `github.com/EternisAI/silo-proxy`.

## Scope
- Prefer minimal, safe changes that preserve behavior.
- Follow existing repo patterns over generic best practices.
- Keep API, service/domain, and infrastructure concerns separated.
- Avoid unrelated refactors in focused tasks.

## Stack Snapshot
- Go `1.25.0`.
- HTTP: Gin.
- RPC: gRPC/protobuf (`proto/proxy.proto`).
- DB: PostgreSQL via `pgx/v5`, migrations via goose, queries via sqlc.
- Auth: JWT + API key middleware.
- Logging: `log/slog`.
- Tests: Go `testing` + `testify` + testcontainers system tests.

## Repository Layout
- `cmd/silo-proxy-server`: server entrypoint, config, logger.
- `cmd/silo-proxy-agent`: agent entrypoint, config, logger.
- `internal/api/http`: router, handlers, middleware, per-agent HTTP management.
- `internal/grpc/server`: stream server and connection manager.
- `internal/grpc/client`: agent stream client and local forwarder.
- `internal/auth`, `internal/users`: business/domain services.
- `internal/db`: connection init, migrations, sqlc output.
- `proto`: protobuf schema and generated stubs.
- `systemtest`: integration tests against ephemeral Postgres.

## Build / Run / Test Commands

Prefer `make` targets when available.

### Primary Make Targets
- `make build` - build both binaries.
- `make build-server` - build server binary.
- `make build-agent` - build agent binary.
- `make run` - run server locally.
- `make run-agent` - run agent locally.
- `make test` - run all tests (`go test -v ./...`).
- `make clean` - clean test cache and `bin/*`.
- `make generate` - run `sqlc generate`.
- `make protoc-gen` - regenerate protobuf/gRPC code.
- `make generate-certs` - generate local TLS certs.

### Running Focused Tests (Important)
- Single package:
  - `go test -v ./internal/grpc/server`
- Single test function:
  - `go test -v ./internal/grpc/server -run '^TestConnectionManager_Register_WithServerManager$'`
- Single subtest:
  - `go test -v ./systemtest -run '^TestSystemIntegration$/^Login$'`
- Re-run without cache:
  - `go test -v -count=1 ./internal/api/http`
- Benchmarks:
  - `go test -bench . ./internal/grpc/server`
- Compile-only check:
  - `go test -run '^$' ./...`

### Lint / Format / Validation
- No dedicated `make lint` target is currently defined.
- Format changed files with `gofmt -w <files>`.
- Run `go test ./...` as baseline verification.
- Optional static pass: `go vet ./...`.
- Prefer formatting only touched files to avoid noisy diffs.

## Test Selection Guidance
- Start with the narrowest package/test that covers your change.
- Use exact `-run` regex anchors (`^...$`) to avoid accidental matches.
- Use `-count=1` if caching may hide flakiness.
- `systemtest` requires Docker/testcontainers available locally.
- Run full `make test` before finalizing broad changes.

## Code Style Guidelines

### Imports
- Keep imports `gofmt`-organized.
- Standard library first, then external/internal packages.
- Use aliases only when clarity improves (e.g. `grpcserver`).
- Avoid dot imports and unused imports.

### Formatting and Structure
- Enforce `gofmt`.
- Prefer early returns over nested conditionals.
- Keep functions focused; avoid mixing concerns.
- Add comments only for non-obvious invariants/behavior.

### Types and API Shapes
- Use explicit structs for service/DTO outputs (`RegisterResult`, `UserInfo`).
- Keep struct fields cohesive to a single responsibility.
- Prefer typed constants for repeated timing/protocol values.
- Use typed config structs with `mapstructure` tags.

### Naming Conventions
- Exported identifiers: `PascalCase`.
- Unexported identifiers: `camelCase`.
- Constructors follow `NewXxx(...) *Xxx`.
- Handler methods should be verb-driven (`Login`, `Register`, `DeleteUser`).
- Sentinel errors use `ErrXxx` naming.

### Error Handling
- Return errors instead of panicking (except startup-fatal bootstrap failures).
- Wrap lower-level errors with context using `%w`.
- Branch on known causes using `errors.Is` / `errors.As`.
- Keep domain-level error contracts stable for handler mapping.
- Do not expose sensitive internals in HTTP error payloads.

### Logging
- Use structured `slog` logs (`Info/Warn/Error/Debug`).
- Include stable keys like `agent_id`, `port`, `message_id`, `error`.
- Log lifecycle and recovery events (start/stop/retry/failure).
- Avoid noisy logs in hot loops unless debug-level is warranted.

### Concurrency and Context
- Guard shared maps/state with `sync.RWMutex` where applicable.
- Use buffered channels for producer/consumer coordination.
- Use `context.WithTimeout` for shutdown/network operations.
- Preserve existing cancellation and cleanup semantics.

### HTTP Layer Rules
- Keep handlers thin: bind/validate input, call service, map response.
- Return JSON errors in consistent shape: `{"error":"..."}`.
- Put cross-cutting concerns in middleware (auth, API key, logging).
- Treat read-only/status endpoints as side-effect free.

### Service and Domain Rules
- Keep business logic in services, not in handlers/middleware.
- Keep DB-specific behavior at DB/service boundaries.
- Keep contracts explicit for auth/user and other domain services.

### DB, Migrations, SQLC
- SQL files belong in `internal/db/queries`.
- sqlc-generated files live in `internal/db/sqlc` and must not be hand-edited.
- After SQL query/schema changes, run `make generate` and migrations.
- Keep migrations idempotent and ordered.

### Proto / gRPC Changes
- Edit `proto/proxy.proto`, then run `make protoc-gen`.
- Update both server and agent paths for schema/message changes.
- Preserve compatibility expectations when possible.

## Configuration Conventions
- Config is loaded from `application.yml` with env overrides.
- Nested key env mapping uses underscore replacement.
- Server and agent keep separate config roots under `cmd/...`.
- Keep examples/defaults aligned with checked-in app configs.

## CI Notes
- CI runs `make test` and `make build` on pull requests.
- Docker publish jobs execute only on `main` and version tags.
- If packaging/runtime behavior changes, validate make/docker targets.

## Cursor / Copilot Rules Check
- No `.cursorrules` file found.
- No `.cursor/rules/` directory found.
- No `.github/copilot-instructions.md` file found.
- If added later, treat those files as higher-priority local instructions.

## Practical Agent Workflow
1. Make the smallest viable change.
2. Run targeted tests first, then broaden scope.
3. Keep diffs tight and architecture boundaries intact.
4. Add/adjust logs and errors for operability.
