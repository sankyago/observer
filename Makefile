.PHONY: build test test-short lint migrate up down

ENV ?= OBSERVER_DB_DSN="postgres://observer:observer@localhost:5432/observer?sslmode=disable" \
       OBSERVER_MQTT_URL="tcp://localhost:1883"

build:
	go build ./...

test:
	go test ./...

test-short:
	go test -short ./...

lint:
	golangci-lint run ./...

migrate:
	$(ENV) go run ./cmd/migrate up

up:
	docker compose -f deploy/docker-compose.yml up -d

down:
	docker compose -f deploy/docker-compose.yml down
