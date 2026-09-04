# Runbook

Operational reference: what the signals mean, and what to do about them.

## Reading the signals

### `/metrics` (RED: Rate, Errors, Duration)

- `http_requests_total{method,route,status}` — a sudden rise in `5xx` for
  one route means that handler (or something it calls — DB, Redis) is
  failing; a rise in `429` means clients are hitting the rate limiter,
  which is either abuse or the limit being set too low for real traffic.
- `http_request_duration_seconds` — watch p99 per route. A slow `/api/v1/tasks`
  with fast everything else usually means a slow query or missing index,
  not a systemic problem.

### Log levels (structured, `request_id`/`trace_id`-tagged)

- `ERROR` — a 5xx or a panic. Every `ERROR` line has `request_id`; grep
  logs for it to see the exact request, and use `trace_id` (present when
  `OTEL_EXPORTER_OTLP_ENDPOINT` is set) to pull the full trace — including
  the DB spans — in Jaeger/whatever OTLP backend is configured.
- `WARN` — a 4xx, or the rate limiter falling back from Redis to in-memory
  (meaning Redis is unreachable — rate limiting still works, but is no
  longer accurate across replicas until Redis recovers).
- `INFO` — normal request/lifecycle logging.

### `/healthz` vs `/readyz`

- `/healthz` failing = the process itself is wedged; the orchestrator
  should restart the pod/container.
- `/readyz` returning `503` with `"status":"unavailable"` = the database is
  unreachable — check Postgres, not the API process.
- `/readyz` returning `503` with `"status":"draining"` = normal shutdown in
  progress (SIGTERM received); this is not an incident.

## Common incidents

### Spike in 401s on `/api/v1/auth/refresh`

Expected if a client is retrying a refresh token it already used (or an
attacker replayed a stolen one) — reuse detection revokes the whole
session on purpose (see `AuthService.Refresh`). If a *specific* user
reports being logged out unexpectedly and repeatedly, check whether their
client is calling `/refresh` from two places concurrently (e.g. a race
between a background refresh and a foreground one) — that legitimately
trips reuse detection today; there's no grace window. That's a known,
deliberate trade-off (simplicity/security over convenience), not a bug.

### Elevated latency, DB spans dominate the trace

Check `pg_stat_activity` for long-running queries or lock contention.
`internal/repository/task_repository.go`'s `ListByUser` does a `COUNT(*)`
and a paged `SELECT` on every list call — under heavy load with many rows
per user, that COUNT is the first thing to get expensive.

### Rate limiter stuck in-memory mode across replicas

Log line: `"rate limiting backed by in-memory token bucket (single-instance
only)"` on every replica means `REDIS_ADDR` is unset or Redis was
unreachable at startup (the check only happens once, at boot — a Redis
outage that resolves later does *not* self-heal the limiter back to
distributed mode without a restart). Fix Redis, then roll the deployment.

## Database backup / restore

```bash
# Backup
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME -Fc -f taskflow-$(date +%Y%m%d).dump

# Restore into a fresh database
pg_restore -h $DB_HOST -U $DB_USER -d $DB_NAME --clean --if-exists taskflow-YYYYMMDD.dump
```

Migrations are idempotent SQL applied in filename order on every boot
(`internal/database/database.go`), so a restore followed by a normal
deploy will safely re-apply anything already present.

## Rollback

Redeploy the previous image tag/version — there is no destructive schema
migration in this codebase today (migrations are additive `CREATE TABLE IF
NOT EXISTS`), so rolling back the app version is safe without a
corresponding DB rollback.

## Known limitations (deliberate, not oversights)

- **JWT secret rotation invalidates every outstanding access token
  immediately** — there's no multi-key/`kid` support, so rotating
  `JWT_SECRET` logs everyone out at once (refresh tokens are unaffected
  and will mint new, valid access tokens on next use). Plan a rotation
  around that, or treat it as a reason to keep access tokens short-lived
  (already the default: 15m).
- **Redis-backed rate limiting is a fixed 1-second window**, not a true
  token bucket — it allows up to ~2x `RATE_LIMIT_BURST` right at a window
  boundary. See `internal/httpapi/middleware/ratelimit_redis.go`.
- **Refresh token rotation has no grace window** — seeing this take down a
  legitimate client means that client is calling `/refresh` concurrently
  from two places; fix the client, don't work around it server-side.
