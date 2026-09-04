# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added
- Distributed rate limiting via Redis (fixed-window, shared across
  replicas), falling back to the in-memory limiter — automatically, with a
  logged warning — when `REDIS_ADDR` is unset or unreachable.
- Distributed tracing via OpenTelemetry: HTTP requests (`otelhttp`) and
  every database call (`otelsql`) produce spans, exported over OTLP/HTTP
  when `OTEL_EXPORTER_OTLP_ENDPOINT` is set (no-op otherwise). Trace IDs
  are also attached to structured logs.
- Refresh tokens with rotation and reuse detection: `/api/v1/auth/refresh`
  and `/api/v1/auth/logout`. Access tokens are now short-lived (15m
  default) since refresh tokens exist to renew them.
- Real database integration tests (`internal/repository/*_integration_test.go`,
  `make test-integration`) using testcontainers-go against actual
  Postgres — unique-constraint violations, cascade deletes, and pagination
  totals are now verified against real SQL, not just in-memory fakes.
- Readiness-aware graceful shutdown: `/readyz` fails immediately (without
  touching the DB) the instant a shutdown signal is received
  (`SHUTDOWN_DRAIN_DELAY`), so a load balancer stops routing here before
  connections are cut.
- Secrets-as-files support (`JWT_SECRET_FILE`, `DB_PASSWORD_FILE`) — the
  same convention the official postgres/mysql Docker images use.
- CI pipeline (`.github/workflows/ci.yml`): lint, unit tests, real-Postgres
  integration tests, `govulncheck`, Docker build + Trivy scan + SBOM.
  `.github/dependabot.yml` for weekly dependency updates.
- Kubernetes manifests (`deploy/k8s/`): Deployment, Service, HPA, PDB,
  ConfigMap/Secret templates.
- OpenAPI 3.0 specification (`api/openapi.yaml`) for every endpoint.
- Operational runbook (`docs/RUNBOOK.md`): signal interpretation, common
  incidents, backup/restore, and documented known limitations.

## [0.2.0] - core hardening pass

### Added
- Machine-readable error `code` field and `request_id` in every JSON error
  response, correlating client-visible errors to server-side logs.
- Request-scoped structured logger (`internal/httpapi/reqctx`) so every log
  line in the request path — including previously-unlabeled 500s — carries
  `request_id`.
- Configurable log level (`LOG_LEVEL`) and leveled access logs (5xx → Error,
  4xx → Warn, else Info) with response size and user agent.
- Build version surfaced in `/healthz`, `/readyz`, and every log line
  (`internal/version`, wired via `-ldflags` in the Makefile and Dockerfile).
- Config validation: refuses to start in `ENV=production` with the default
  or a too-short `JWT_SECRET`, or with non-positive rate-limit/timeout/body
  settings.
- Security headers middleware (`X-Content-Type-Options`, `X-Frame-Options`,
  `Referrer-Policy`, `Cross-Origin-Resource-Policy`).
- Non-root user in the Docker runtime image; build-time `VERSION` arg.
- `golangci-lint` config, `.editorconfig`, `LICENSE` (MIT), `CONTRIBUTING.md`.
- `ReadHeaderTimeout` on the HTTP server (slowloris mitigation).

### Changed
- `docker-compose.yml` secrets/config are now overridable via a git-ignored
  `.env` file instead of hardcoded in the committed file.

## [0.1.0] - pre-hardening baseline

Initial version: layered architecture, JWT auth, versioned REST routing,
middleware stack (request ID, logging, recovery, rate limiting, CORS,
timeouts, body limits), Prometheus metrics, liveness/readiness probes,
Postgres via `database/sql`, migrations, Docker/Compose, unit + router
tests.
