.PHONY: run build up down test flush quota benchmark

run-backend:
	go run ./cmd/demo-api

run-gateway:
	go run ./cmd/gateway

build:
	go build -o bin/demo-api ./cmd/demo-api
	go build -o bin/gateway ./cmd/gateway

up:
	docker compose up --build

up-detached:
	docker compose up --build -d

down:
	docker compose down -v

flush:
	docker compose exec redis redis-cli FLUSHALL

quota:
	k6 run loadtest/quota-test.js

benchmark:
	k6 run loadtest/basic.js

test:
	go test ./...

api-key-test:
	k6 run loadtest/api-key-test.js