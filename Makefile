# AgentGuard developer Makefile.
#
# Conventions:
#   - Every target is .PHONY.
#   - Output binaries land under ./bin/.
#   - Use `make install-tools` once on a fresh checkout to fetch lint/format
#     helpers; everything else only needs the Go toolchain.

SHELL := /bin/sh
GO    ?= go
BIN   := bin
PKG   := github.com/agentguard/agentguard

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X $(PKG)/internal/version.Version=$(VERSION) \
           -X $(PKG)/internal/version.Commit=$(COMMIT) \
           -X $(PKG)/internal/version.Date=$(DATE)

.PHONY: all build test lint bench clean install-tools mock e2e fmt vet

all: build

build: ## Build the agentguard binary into ./bin
	@mkdir -p $(BIN)
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN)/agentguard ./cmd/agentguard

mock: ## Build the mock MCP server used by e2e tests
	@mkdir -p $(BIN)
	$(GO) build -trimpath -o $(BIN)/mock_mcp_server ./e2e/mock_mcp_server

# -race needs CGO. modernc.org/sqlite is pure-Go so we want CGO off in
# release builds; CI runs Linux where enabling -race is free. On dev hosts
# without a C compiler (typical Windows), call `make test-norace` instead.
test: ## Run unit + integration tests with race detector (Linux/macOS)
	CGO_ENABLED=1 $(GO) test -race -timeout 120s ./...

test-norace: ## Run unit + integration tests without -race (Windows-friendly)
	$(GO) test -timeout 120s ./...

e2e: build mock ## Run the shell-driven e2e test (POSIX only)
	bash ./e2e/wrap_test.sh

fmt: ## Format all Go code
	$(GO) fmt ./...

vet: ## Run go vet
	$(GO) vet ./...

lint: vet ## Run golangci-lint (falls back to go vet if not installed)
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "(golangci-lint not installed; running go vet only — run 'make install-tools' to add it)"; \
	fi

bench: ## Run the benchmark suite
	$(GO) test -bench=. -benchmem -run=^$$ ./bench/...

clean: ## Remove build outputs
	rm -rf $(BIN) coverage.out coverage.html

install-tools: ## Install golangci-lint and goose to $GOBIN
	$(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
	$(GO) install github.com/pressly/goose/v3/cmd/goose@latest

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'
