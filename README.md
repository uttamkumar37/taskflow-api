# TaskFlow — a production-shaped Go backend microservice

A JWT-authenticated Task/Todo REST API, built to demonstrate what a
production Go service actually looks like: layered architecture, versioned
routing, middleware (auth, distributed rate limiting, CORS, timeouts,
request tracing), database access, migrations, observability (structured
logs, metrics, distributed tracing, health probes), refresh-token auth with
rotation, graceful shutdown, containerization, CI/CD, Kubernetes manifests,
and testing (unit + real-Postgres integration tests).

## Architecture

```
cmd/api/main.go        entrypoint: wires everything together, starts the server
internal/
  config/               env-based configuration + fail-fast validation
  database/             Postgres connection + migrations (otelsql-instrumented)
  domain/                core entities (User, Task, RefreshToken) and shared error types
  repository/           database/sql queries — implements interfaces the service layer depends on
  service/               business logic: auth, refresh-token rotation, tasks, JWT, pagination
  httpapi/
    handler/             HTTP handlers (parse request -> call service -> write response)
    middleware/          request ID, logging, panic recovery, JWT auth, CORS,
                          rate limiting (in-memory or Redis), timeouts, metrics, security headers
    dto/                 request/response JSON shapes + validation
    response/            consistent JSON response helpers (error envelope: message + code + request_id)
    reqctx/              request-scoped context helpers (request ID, trace ID, pre-tagged logger)
    router.go             route table
  logging/               process-wide structured logger construction (level, JSON/text)
  tracing/               OpenTelemetry TracerProvider setup (OTLP/HTTP export)
  version/               build version, injected via -ldflags
api/openapi.yaml         OpenAPI 3.0 spec for every endpoint
deploy/k8s/              Kubernetes manifests (Deployment, Service, HPA, PDB, ConfigMap)
docs/RUNBOOK.md          on-call reference: what the signals mean, common incidents, backup/restore
.github/workflows/ci.yml lint, unit tests, Postgres integration tests, govulncheck, Docker build + scan + SBOM
```

Request flow: `router -> middleware -> handler -> service -> repository -> Postgres`.
Each layer only knows about the one directly below it, and depends on an
**interface** (`UserRepository`, `TaskRepository`, `RefreshTokenRepository`)
rather than a concrete type — that's why `internal/service/*_test.go` can
test all the business logic (password hashing, ownership checks, JWT
expiry, refresh rotation, pagination) with in-memory fakes and no real
database, while `internal/repository/*_integration_test.go` separately
verifies the real SQL against real Postgres.

## Concepts this project demonstrates

| Concept | Where |
|---|---|
| Layered/clean architecture | overall package layout |
| Dependency injection (by hand, no framework) | [cmd/api/main.go](cmd/api/main.go) |
| Environment-based config (12-factor) + fail-fast validation | [internal/config/config.go](internal/config/config.go) |
| Secrets as files (Docker/K8s secret-mount convention, `*_FILE` env vars) | [internal/config/config.go](internal/config/config.go) |
| Versioned REST routing (`/api/v1`), no 3rd-party router | [internal/httpapi/router.go](internal/httpapi/router.go) |
| Middleware chaining, layered per route tier | [internal/httpapi/middleware/chain.go](internal/httpapi/middleware/chain.go), [router.go](internal/httpapi/router.go) |
| Request tracing (`X-Request-ID` correlation) | [internal/httpapi/middleware/request_id.go](internal/httpapi/middleware/request_id.go) |
| Distributed tracing (OpenTelemetry, OTLP/HTTP export, HTTP + DB spans) | [internal/tracing/tracing.go](internal/tracing/tracing.go), [internal/database/database.go](internal/database/database.go) |
| Structured, leveled logging (`log/slog`), JSON in prod / text in dev, configurable `LOG_LEVEL` | [internal/logging/logging.go](internal/logging/logging.go), [internal/httpapi/middleware/logging.go](internal/httpapi/middleware/logging.go) |
| Request-scoped logger (every log line carries `request_id` + `trace_id`, including 500s and panics) | [internal/httpapi/reqctx/reqctx.go](internal/httpapi/reqctx/reqctx.go) |
| Consistent error envelope (`error` + machine-readable `code` + `request_id`) | [internal/httpapi/response/response.go](internal/httpapi/response/response.go) |
| Build version surfaced in `/healthz`/`/readyz`/logs | [internal/version/version.go](internal/version/version.go) |
| Security headers (nosniff, frame-deny, no-referrer, CORP) | [internal/httpapi/middleware/security_headers.go](internal/httpapi/middleware/security_headers.go) |
| Panic recovery | [internal/httpapi/middleware/recover.go](internal/httpapi/middleware/recover.go) |
| Distributed rate limiting (Redis, fixed-window), in-memory fallback | [internal/httpapi/middleware/ratelimit.go](internal/httpapi/middleware/ratelimit.go), [ratelimit_redis.go](internal/httpapi/middleware/ratelimit_redis.go) |
| CORS for cross-origin clients | [internal/httpapi/middleware/cors.go](internal/httpapi/middleware/cors.go) |
| Request timeouts & body-size limits | [internal/httpapi/middleware/timeout.go](internal/httpapi/middleware/timeout.go), [bodylimit.go](internal/httpapi/middleware/bodylimit.go) |
| Prometheus metrics (RED: rate/errors/duration) | [internal/httpapi/middleware/metrics.go](internal/httpapi/middleware/metrics.go) |
| Liveness vs. readiness probes, with shutdown-aware draining | [internal/httpapi/handler/health_handler.go](internal/httpapi/handler/health_handler.go), [cmd/api/main.go](cmd/api/main.go) |
| Password hashing (bcrypt) | [internal/service/auth_service.go](internal/service/auth_service.go) |
| JWT issuing & validation (short-lived access tokens) | [internal/service/token.go](internal/service/token.go) |
| Refresh tokens: rotation on every use + reuse-detection (revokes all sessions) | [internal/service/auth_service.go](internal/service/auth_service.go) |
| Auth middleware + request-scoped context | [internal/httpapi/middleware/auth.go](internal/httpapi/middleware/auth.go) |
| Authorization (ownership checks) | [internal/service/task_service.go](internal/service/task_service.go) |
| SQL access with `database/sql` + Postgres | [internal/repository/](internal/repository/) |
| Pagination (limit/offset, capped, with total count) | [internal/service/task_service.go](internal/service/task_service.go), [internal/repository/task_repository.go](internal/repository/task_repository.go) |
| Schema migrations | [internal/database/migrations/](internal/database/migrations/) |
| Consistent error -> HTTP status mapping | [internal/httpapi/handler/errors.go](internal/httpapi/handler/errors.go) |
| Graceful shutdown with readiness draining | [cmd/api/main.go](cmd/api/main.go) |
| Containerization (multi-stage, non-root, build-time version) | [Dockerfile](Dockerfile), [docker-compose.yml](docker-compose.yml) |
| Kubernetes manifests (Deployment, Service, HPA, PDB) | [deploy/k8s/](deploy/k8s/) |
| CI/CD: lint, unit + integration tests, vuln scan, image scan, SBOM | [.github/workflows/ci.yml](.github/workflows/ci.yml) |
| OpenAPI 3.0 specification | [api/openapi.yaml](api/openapi.yaml) |
| Unit tests with fakes + real-Postgres integration tests (testcontainers) | [internal/service/*_test.go](internal/service/), [internal/repository/*_integration_test.go](internal/repository/) |

## Running it

### Option A — Docker Compose (recommended, matches production shape)

```bash
docker compose up --build
# or: make up
```

This starts Postgres, Redis, Jaeger, and the API together. The API waits
for Postgres/Redis to report healthy, then runs migrations automatically
on boot. `docker-compose.yml` reads secrets/config from a git-ignored
`.env` file in this directory if one exists (`cp .env.example .env` and
edit it) — override `JWT_SECRET` there for anything beyond a throwaway
local run. Once it's up:

- API: `http://localhost:8080`
- Jaeger UI (distributed traces): `http://localhost:16686`

### Option B — locally against a Postgres you already have running

```bash
cp .env.example .env   # edit if your DB creds differ
export $(cat .env | xargs)
go run ./cmd/api
```

Redis and tracing are optional in this mode — leave `REDIS_ADDR` and
`OTEL_EXPORTER_OTLP_ENDPOINT` unset and the service falls back to
in-memory rate limiting and no-op tracing, respectively.

### Running tests

```bash
go test ./... -v
# or: make test

# Real Postgres via testcontainers-go — requires Docker:
make test-integration
```

### Development checks

```bash
make fmt   # gofmt
make vet   # go vet
make lint  # golangci-lint — install: https://golangci-lint.run/welcome/install/
make coverage
```

## API reference

Full request/response schemas: [api/openapi.yaml](api/openapi.yaml). All
endpoints are versioned under `/api/v1`. Protected routes require
`Authorization: Bearer <token>`.

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | no | liveness — is the process up |
| GET | `/readyz` | no | readiness — is the DB reachable, and not draining for shutdown |
| GET | `/metrics` | no | Prometheus metrics |
| POST | `/api/v1/auth/signup` | no | returns access + refresh token |
| POST | `/api/v1/auth/login` | no | returns access + refresh token |
| POST | `/api/v1/auth/refresh` | no (refresh token in body) | rotates to a new access + refresh pair |
| POST | `/api/v1/auth/logout` | no (refresh token in body) | revokes the given refresh token |
| POST | `/api/v1/tasks` | yes | |
| GET | `/api/v1/tasks` | yes | `?status=`, `?limit=`, `?offset=` |
| GET | `/api/v1/tasks/{id}` | yes | |
| PUT | `/api/v1/tasks/{id}` | yes | |
| DELETE | `/api/v1/tasks/{id}` | yes | |

## Trying the API (curl)

```bash
# 1. Sign up (returns an access token + refresh token)
curl -s -X POST localhost:8080/api/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"password123"}' | tee /tmp/signup.json

TOKEN=$(jq -r .token /tmp/signup.json)
REFRESH=$(jq -r .refresh_token /tmp/signup.json)

# 2. Create a task
curl -s -X POST localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"Learn backend concepts","description":"build a microservice"}'

# 3. List your tasks (paginated: {"data": [...], "total", "limit", "offset"})
curl -s "localhost:8080/api/v1/tasks?limit=10&offset=0" -H "Authorization: Bearer $TOKEN"

# 4. Update a task (replace 1 with the real id)
curl -s -X PUT localhost:8080/api/v1/tasks/1 \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"title":"Learn backend concepts","status":"done"}'

# 5. Delete it
curl -s -X DELETE localhost:8080/api/v1/tasks/1 -H "Authorization: Bearer $TOKEN"

# 6. Refresh the access token (rotates the refresh token — the old one stops working)
curl -s -X POST localhost:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH\"}"

# 7. Log out (revokes the refresh token)
curl -s -X POST localhost:8080/api/v1/auth/logout \
  -H 'Content-Type: application/json' -d "{\"refresh_token\":\"$REFRESH\"}"

# 8. Liveness / readiness / metrics (no auth)
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s localhost:8080/metrics | head
```

## Operational behavior worth knowing

- **Rate limiting**: 5 req/s per client IP by default (`RATE_LIMIT_RPS`,
  `RATE_LIMIT_BURST`), enforced only on `/api/v1/*` — probes and metrics are
  exempt. Exceeding it returns `429`. Backed by Redis (shared and accurate
  across every replica) when `REDIS_ADDR` is set, falling back to an
  in-memory, single-instance-only limiter otherwise — and falling back
  automatically (with a logged warning) if Redis is configured but
  unreachable at startup, rather than refusing to start.
- **Refresh tokens**: access tokens (`JWT_EXPIRES_IN`, default 15m) are
  short-lived by design; refresh tokens (`REFRESH_TOKEN_EXPIRES_IN`,
  default 720h) are long-lived, single-use, and rotated on every call to
  `/api/v1/auth/refresh`. Presenting an already-used refresh token is
  treated as a compromise signal and revokes every session for that user —
  see [docs/RUNBOOK.md](docs/RUNBOOK.md) for what that looks like in practice.
- **Distributed tracing**: set `OTEL_EXPORTER_OTLP_ENDPOINT` to export
  spans (HTTP requests via `otelhttp`, database calls via `otelsql`) to
  Jaeger/an OTel Collector/Tempo. Unset, tracing is a no-op — spans are
  created but immediately dropped, at negligible cost.
- **Request timeout**: every `/api/v1/*` request is aborted after
  `REQUEST_TIMEOUT` (default 15s) if it hasn't responded.
- **Body size limit**: requests larger than `MAX_BODY_BYTES` (default 1MB)
  are rejected.
- **Request tracing**: every response carries an `X-Request-ID` header
  (echoing one you send, or a generated one), and every log line for that
  request — at any layer, via `reqctx.Logger(ctx)` — includes `request_id`
  (and `trace_id` when tracing is enabled), so you can grep logs by it to
  follow one request through the system.
- **Error responses** are a consistent envelope:
  `{"error": "resource not found", "code": "NOT_FOUND", "request_id": "..."}`.
  `code` is the stable, machine-readable value to branch on; `error` is a
  human-readable message that may change wording; `request_id` matches the
  `X-Request-ID` response header, for support/debugging correlation.
- **Access logs are leveled by outcome**: 5xx → `Error`, 4xx → `Warn`,
  everything else → `Info`, so log-based alerting can filter on level alone.
  Set `LOG_LEVEL=debug` for verbose local debugging (adds source file:line).
- **Config validation is fail-fast**: with `ENV=production`, the process
  refuses to start if `JWT_SECRET` is the default value, shorter than 32
  characters, or if any rate-limit/timeout/body-size setting is non-positive.
  `JWT_SECRET`/`DB_PASSWORD` can also be supplied via a mounted file
  (`JWT_SECRET_FILE=/path`), the same convention the official
  postgres/mysql Docker images use.
- **Liveness vs. readiness**: `/healthz` never touches the DB (so
  orchestrators don't restart a healthy process just because Postgres
  blipped); `/readyz` does, and also fails fast (without touching the DB)
  the instant a shutdown signal is received — see `SHUTDOWN_DRAIN_DELAY`.
  Both report the running build's `version`.

## Deploying

- **Kubernetes**: manifests in [deploy/k8s/](deploy/k8s/) (Deployment,
  Service, HorizontalPodAutoscaler, PodDisruptionBudget, ConfigMap). See
  that directory's README for apply order and what's deliberately left out
  (Postgres/Redis/Jaeger — use managed services or dedicated charts, not
  raw Deployments, for stateful services).
- **CI/CD**: [.github/workflows/ci.yml](.github/workflows/ci.yml) runs
  lint, unit tests, real-Postgres integration tests, `govulncheck`, and a
  Docker build with a Trivy vulnerability scan + SPDX SBOM on every push/PR.
  It builds and scans the image but doesn't push anywhere yet — there's no
  registry configured.

## Suggested next steps to deepen your understanding

1. **Break something on purpose** — remove the `Auth` middleware from one
   route and observe how ownership checks in the service layer *don't* save
   you (they only run once a handler calls them). This shows why defense
   needs to exist at the right layer.
2. **Swap migrations** for [golang-migrate](https://github.com/golang-migrate/migrate)
   to see how "real" migration tooling (up/down, versioning) differs from
   this project's simple idempotent-SQL-on-boot approach.
3. **Push to a real GitHub repo and a container registry** — the CI
   pipeline and Kubernetes manifests are ready for both but intentionally
   left dormant (no repo, no registry configured yet).
4. **Add a grace window to refresh-token rotation** (accept the
   immediately-prior token for a few seconds) to tolerate concurrent
   refresh calls from the same client without tripping reuse detection —
   a real trade-off between strict security and client-side robustness.
5. **Swap the Redis fixed-window limiter for a true token bucket** (GCRA
   via a more elaborate Lua script) if the ~2x-burst-at-window-boundary
   approximation ever actually matters for your traffic shape.
6. **Add JWT key rotation** (`kid` header + multiple valid signing keys) so
   rotating `JWT_SECRET` doesn't invalidate every outstanding access token
   at once — see the "known limitations" section of
   [docs/RUNBOOK.md](docs/RUNBOOK.md).
7. **Wire up Prometheus + Grafana** (not just Jaeger) locally to see the
   RED metrics this service already exposes, graphed alongside the traces.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Licensed under [MIT](LICENSE).
