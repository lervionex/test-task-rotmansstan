APP_BIN := bin/withdrawals-service

.PHONY: run build test test-unit test-integration

run:
	go run ./cmd/api

build:
	mkdir -p bin
	go build -o $(APP_BIN) ./cmd/api

test:
	go test ./...

test-unit:
	go test ./tests/unit/...

test-integration:
	go test ./tests/integration/...
