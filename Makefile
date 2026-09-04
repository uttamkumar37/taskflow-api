.PHONY: run seed test test-integration build up down logs lint fmt vet tidy coverage

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X taskflow/internal/version.Version=$(VERSION)

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/api

# Populates the DB with a few dummy users and tasks for local dev/demoing.
# Safe to re-run — existing users/tasks are detected and skipped.
seed:
	go run ./cmd/seed

test:
	go test ./... -v

# Requires Docker: spins up real Postgres containers via testcontainers-go
# and runs the repository layer against them (not in-memory fakes).
test-integration:
	go test -tags=integration ./... -v

build:
	go build -ldflags "$(LDFLAGS)" -o bin/api ./cmd/api

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -w .

vet:
	go vet ./...

tidy:
	go mod tidy

coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

up:
	VERSION=$(VERSION) docker compose up --build

down:
	docker compose down -v
