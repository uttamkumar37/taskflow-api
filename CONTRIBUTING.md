# Contributing

## Dev setup

```bash
cp .env.example .env
docker compose up --build   # Postgres + API, migrations run automatically
# or, against a Postgres you already have running:
export $(cat .env | xargs) && go run ./cmd/api
```

## Before opening a PR

```bash
make fmt     # gofmt
make vet     # go vet
make lint    # golangci-lint (install: https://golangci-lint.run/welcome/install/)
make test    # go test ./... -v
```

All four must be clean. CI (if configured for your fork/remote) runs the
same checks.

## Code style

- Follow the existing layering: `handler -> service -> repository`. Handlers
  parse/validate HTTP input and map errors to status codes; services hold
  business rules and authorization checks; repositories are the only code
  that talks SQL.
- Errors cross layers as the sentinel values in `internal/domain/errors.go`
  (wrapped with `fmt.Errorf("...: %w", ...)` for context), not as ad hoc
  strings — `handler/errors.go` is the single place they're mapped to HTTP
  status codes and machine-readable codes.
- New handlers must accept `*http.Request` and use `response.Error(w, r,
  status, code, message)`, not a bare status/message — the request is what
  lets the response carry a `request_id` for support/debugging correlation.
- Log via `reqctx.Logger(r.Context())` inside request-handling code paths
  (it's pre-tagged with `request_id`), not a bare `slog.Error`/`slog.Info`.

## Commit messages

Short, imperative subject line (`fix: ...`, `feat: ...`, `chore: ...`,
`docs: ...`); explain *why* in the body when the change isn't self-evident.
