.PHONY: run test build up down logs

run:
	go run ./cmd/api

test:
	go test ./... -v

build:
	go build -o bin/api ./cmd/api

up:
	docker compose up --build

down:
	docker compose down -v
