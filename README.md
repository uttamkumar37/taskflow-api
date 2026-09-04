# TaskFlow — a production-shaped Go backend microservice

A JWT-authenticated Task/Todo REST API, built to demonstrate what a
production Go service actually looks like: layered architecture, versioned
routing, middleware (auth, rate limiting, CORS, timeouts, request tracing),
database access, migrations, observability (structured logs, metrics,
health probes), graceful shutdown, containerization, and testing.

## Architecture

```
cmd/api/main.go        entrypoint: wires everything together, starts the server
internal/
  config/               env-based configuration
  database/             Postgres connection + migrations
  domain/                core entities (User, Task) and shared error types
  repository/           database/sql queries — implements interfaces the service layer depends on
  service/               business logic: auth, tasks, JWT issuing/validation, pagination rules
  httpapi/
    handler/             HTTP handlers (parse request -> call service -> write response)
    middleware/          request ID, logging, panic recovery, JWT auth, CORS, rate limiting, timeouts, metrics
    dto/                 request/response JSON shapes + validation
    response/            consistent JSON response helpers (error envelope: message + code + request_id)
    reqctx/              request-scoped context helpers (request ID, pre-tagged logger)
    router.go             route table
  logging/               process-wide structured logger construction (level, JSON/text)
  version/               build version, injected via -ldflags
```

Request flow: `router -> middleware -> handler -> service -> repository -> Postgres`.
Each layer only knows about the one directly below it, and depends on an
**interface** (`UserRepository`, `TaskRepository`) rather than a concrete
type — that's why `internal/service/*_test.go` can test all the business
logic (password hashing, ownership checks, JWT expiry, pagination) with
in-memory fakes and no real database.

## Concepts this project demonstrates

| Concept | Where |
|---|---|
| Layered/clean architecture | overall package layout |
| Dependency injection (by hand, no framework) | [cmd/api/main.go](cmd/api/main.go) |
| Environment-based config (12-factor) | [internal/config/config.go](internal/config/config.go) |
| Versioned REST routing (`/api/v1`), no 3rd-party router | [internal/httpapi/router.go](internal/httpapi/router.go) |
| Middleware chaining, layered per route tier | [internal/httpapi/middleware/chain.go](internal/httpapi/middleware/chain.go), [router.go](internal/httpapi/router.go) |
| Request tracing (`X-Request-ID` correlation) | [internal/httpapi/middleware/request_id.go](internal/httpapi/middleware/request_id.go) |
| Structured, leveled logging (`log/slog`), JSON in prod / text in dev, configurable `LOG_LEVEL` | [internal/logging/logging.go](internal/logging/logging.go), [internal/httpapi/middleware/logging.go](internal/httpapi/middleware/logging.go) |
| Request-scoped logger (every log line in the request path carries `request_id`, including 500s and panics) | [internal/httpapi/reqctx/reqctx.go](internal/httpapi/reqctx/reqctx.go) |
| Consistent error envelope (`error` + machine-readable `code` + `request_id`) | [internal/httpapi/response/response.go](internal/httpapi/response/response.go) |
| Fail-fast config validation (refuses to start in prod with an unsafe JWT secret or invalid limits) | [internal/config/config.go](internal/config/config.go) |
| Build version surfaced in `/healthz`/`/readyz`/logs | [internal/version/version.go](internal/version/version.go) |
| Security headers (nosniff, frame-deny, no-referrer, CORP) | [internal/httpapi/middleware/security_headers.go](internal/httpapi/middleware/security_headers.go) |
| Panic recovery | [internal/httpapi/middleware/recover.go](internal/httpapi/middleware/recover.go) |
| Per-IP rate limiting (token bucket) | [internal/httpapi/middleware/ratelimit.go](internal/httpapi/middleware/ratelimit.go) |
| CORS for cross-origin clients | [internal/httpapi/middleware/cors.go](internal/httpapi/middleware/cors.go) |
| Request timeouts & body-size limits | [internal/httpapi/middleware/timeout.go](internal/httpapi/middleware/timeout.go), [bodylimit.go](internal/httpapi/middleware/bodylimit.go) |
| Prometheus metrics (RED: rate/errors/duration) | [internal/httpapi/middleware/metrics.go](internal/httpapi/middleware/metrics.go) |
| Liveness vs. readiness probes | [internal/httpapi/handler/health_handler.go](internal/httpapi/handler/health_handler.go) |
| Password hashing (bcrypt) | [internal/service/auth_service.go](internal/service/auth_service.go) |
| JWT issuing & validation | [internal/service/token.go](internal/service/token.go) |
| Auth middleware + request-scoped context | [internal/httpapi/middleware/auth.go](internal/httpapi/middleware/auth.go) |
| Authorization (ownership checks) | [internal/service/task_service.go](internal/service/task_service.go) |
| SQL access with `database/sql` + Postgres | [internal/repository/](internal/repository/) |
| Pagination (limit/offset, capped, with total count) | [internal/service/task_service.go](internal/service/task_service.go), [internal/repository/task_repository.go](internal/repository/task_repository.go) |
| Schema migrations | [internal/database/migrations/](internal/database/migrations/) |
| Consistent error -> HTTP status mapping | [internal/httpapi/handler/errors.go](internal/httpapi/handler/errors.go) |
| Graceful shutdown | [cmd/api/main.go](cmd/api/main.go) |
| Containerization (multi-stage Docker build + healthcheck) | [Dockerfile](Dockerfile), [docker-compose.yml](docker-compose.yml) |
| Unit + router-level testing with fakes | [internal/service/*_test.go](internal/service/), [internal/httpapi/router_test.go](internal/httpapi/router_test.go) |

## Running it

### Option A — Docker Compose (recommended, matches production shape)

```bash
docker compose up --build
# or: make up
```

This starts Postgres and the API together, with a container healthcheck on
the API. The API waits for Postgres to report healthy, then runs migrations
automatically on boot. `docker-compose.yml` reads secrets/config from a
git-ignored `.env` file in this directory if one exists (`cp .env.example
.env` and edit it) — override `JWT_SECRET` there for anything beyond a
throwaway local run.

### Option B — locally against a Postgres you already have running

```bash
cp .env.example .env   # edit if your DB creds differ
export $(cat .env | xargs)
go run ./cmd/api
```

### Running tests

```bash
go test ./... -v
# or: make test
```

### Development checks

```bash
make fmt   # gofmt
make vet   # go vet
make lint  # golangci-lint — install: https://golangci-lint.run/welcome/install/
make coverage
```

## API reference

All endpoints are versioned under `/api/v1`. Protected routes require
`Authorization: Bearer <token>`.

| Method | Path | Auth | Notes |
|---|---|---|---|
| GET | `/healthz` | no | liveness — is the process up |
| GET | `/readyz` | no | readiness — is the DB reachable |
| GET | `/metrics` | no | Prometheus metrics |
| POST | `/api/v1/auth/signup` | no | |
| POST | `/api/v1/auth/login` | no | |
| POST | `/api/v1/tasks` | yes | |
| GET | `/api/v1/tasks` | yes | `?status=`, `?limit=`, `?offset=` |
| GET | `/api/v1/tasks/{id}` | yes | |
| PUT | `/api/v1/tasks/{id}` | yes | |
| DELETE | `/api/v1/tasks/{id}` | yes | |

## Trying the API (curl)

```bash
# 1. Sign up (returns a JWT)
curl -s -X POST localhost:8080/api/v1/auth/signup \
  -H 'Content-Type: application/json' \
  -d '{"email":"you@example.com","password":"password123"}' | tee /tmp/signup.json

TOKEN=$(jq -r .token /tmp/signup.json)

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

# 6. Liveness / readiness / metrics (no auth)
curl -s localhost:8080/healthz
curl -s localhost:8080/readyz
curl -s localhost:8080/metrics | head
```

## Operational behavior worth knowing

- **Rate limiting**: 5 req/s per client IP by default (`RATE_LIMIT_RPS`,
  `RATE_LIMIT_BURST`), enforced only on `/api/v1/*` — probes and metrics are
  exempt. Exceeding it returns `429`.
- **Request timeout**: every `/api/v1/*` request is aborted after
  `REQUEST_TIMEOUT` (default 15s) if it hasn't responded.
- **Body size limit**: requests larger than `MAX_BODY_BYTES` (default 1MB)
  are rejected.
- **Request tracing**: every response carries an `X-Request-ID` header
  (echoing one you send, or a generated one), and every log line for that
  request — at any layer, via `reqctx.Logger(ctx)` — includes `request_id`,
  so you can grep logs by it to follow one request through the system.
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
- **Liveness vs. readiness**: `/healthz` never touches the DB (so
  orchestrators don't restart a healthy process just because Postgres
  blipped); `/readyz` does, and is what should gate traffic/load-balancer
  routing. Both report the running build's `version`.

## Suggested next steps to deepen your understanding

1. **Break something on purpose** — remove the `Auth` middleware from one
   route and observe how ownership checks in the service layer *don't* save
   you (they only run once a handler calls them). This shows why defense
   needs to exist at the right layer.
2. **Swap migrations** for [golang-migrate](https://github.com/golang-migrate/migrate)
   to see how "real" migration tooling (up/down, versioning) differs from
   this project's simple idempotent-SQL-on-boot approach.
3. **Add an integration test** that runs against a real Postgres via
   [testcontainers-go](https://golang.testcontainers.org/), instead of only
   the in-memory fakes used in `internal/service/*_test.go` and
   `internal/httpapi/router_test.go`.
4. **Add OpenTelemetry tracing** across the handler -> service -> repository
   call chain — you already have request IDs; distributed tracing is the
   natural next step, especially once there's more than one service.
5. **Add refresh tokens** alongside the current short-lived access token.
6. **Move rate limiting to the edge** (API gateway / load balancer) and keep
   the in-process limiter as a defense-in-depth backstop — the usual
   production split.
7. **Wire `/metrics` into Prometheus + Grafana** locally to see the RED
   metrics (rate, errors, duration) this service already exposes, graphed.
8. **Add a CI pipeline** (lint, vet, test, `govulncheck`, Docker build) and
   an OpenAPI spec — deliberately left out of this hardening pass to keep it
   to Go code + repo hygiene, but the natural next layer.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Licensed under [MIT](LICENSE).
