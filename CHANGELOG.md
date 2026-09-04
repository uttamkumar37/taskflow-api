# Changelog

All notable changes to this project are documented here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

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
