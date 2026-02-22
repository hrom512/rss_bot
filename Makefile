.PHONY: build run fmt test lint lint-basic cover check tools \
       migrate-up migrate-down migrate-status migrate-reset

GO ?= go
GOLANGCI_LINT ?= golangci-lint
DB_PATH ?= ./data/bot.db

build:
	$(GO) build -o bin/bot ./cmd/bot
	$(GO) build -o bin/migrate ./cmd/migrate

run: build
	./bin/bot

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

lint:
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || ( \
		echo "$(GOLANGCI_LINT) is not installed. Run: make tools" >&2; \
		exit 1; \
	)
	$(GOLANGCI_LINT) run ./...

cover:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

check: fmt lint test

tools:
	$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

migrate-up:
	$(GO) run ./cmd/migrate -db $(DB_PATH) up

migrate-down:
	$(GO) run ./cmd/migrate -db $(DB_PATH) down

migrate-status:
	$(GO) run ./cmd/migrate -db $(DB_PATH) status

migrate-reset:
	$(GO) run ./cmd/migrate -db $(DB_PATH) reset

clean:
	rm -rf bin/ coverage.out
