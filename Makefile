.PHONY: run test build up down logs lint fmt vet tidy coverage

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X taskflow/internal/version.Version=$(VERSION)

run:
	go run -ldflags "$(LDFLAGS)" ./cmd/api

test:
	go test ./... -v

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
