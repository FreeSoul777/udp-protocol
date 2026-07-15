.PHONY: check run-server run-client test test-unit test-integration test-load test-smoke \
        setup-loss cleanup-loss test-with-loss test-all \
        lint fmt clean \
        docker-build docker-test docker-test-loss

SERVER_BIN = bin/server
CLIENT_BIN = bin/client

SERVER_PORT = 9000
SERVER_HOST = 127.0.0.1
PACKETS_COUNT = 10000
WORKERS_COUNT = 10

check: fmt lint

build:
	go build -o $(SERVER_BIN) ./cmd/server
	go build -o $(CLIENT_BIN) ./cmd/client

run-server: build
	./bin/server --addr=$(SERVER_HOST):$(SERVER_PORT)

run-client: build
	./bin/client --server=$(SERVER_HOST):$(SERVER_PORT) --packets=$(PACKETS_COUNT) --workers=$(WORKERS_COUNT)

test-unit:
	go test ./internal/... -v

test-integration:
	go test ./test/integration/... -v -tags=integration -timeout=3m

test-smoke:
	go test ./test/integration/... -v -tags=integration -timeout=3m -run TestIntegrationSmoke

test-load:
	go test ./test/integration/... -v -tags=integration -timeout=3m -run TestLoad10kPackets

setup-loss:
	./scripts/setup_loss.sh 2

cleanup-loss:
	./scripts/cleanup_loss.sh

test-with-loss: cleanup-loss setup-loss test-integration cleanup-loss

test-all: test-unit test-smoke test-load
	@echo "=== All check tests ==="

docker-dev:
	./scripts/docker-dev.sh

lint:
	golangci-lint run ./...

fmt:
	goimports -w -local github.com/telecom/udp-protocol .

clean:
	rm -rf bin/
	go clean -cache