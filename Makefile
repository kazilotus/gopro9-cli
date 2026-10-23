SHELL := /bin/bash
BIN   := bin/gopro9
PKG   := github.com/kazilotus/gopro9-cli
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X $(PKG)/internal/buildinfo.Version=$(VERSION) -X $(PKG)/internal/buildinfo.Commit=$(COMMIT) -X $(PKG)/internal/buildinfo.Date=$(DATE)

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary into bin/
	go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/gopro9

.PHONY: install
install: ## go install with version metadata
	go install -trimpath -ldflags '$(LDFLAGS)' ./cmd/gopro9

.PHONY: generate
generate: ## Regenerate code from spec/ and the command reference docs
	go generate ./...
	rm -rf docs/commands && go run ./cmd/gopro9 docs markdown --dir docs/commands

.PHONY: generate-check
generate-check: generate ## Fail if generated code or docs are stale
	@git diff --exit-code -- pkg docs/commands docs/CAPABILITIES.md || (echo "generated files are stale: run make generate" && exit 1)
	@test -z "$$(git ls-files --others --exclude-standard docs/commands)" || (echo "untracked generated docs: run make generate and commit" && git ls-files --others --exclude-standard docs/commands && exit 1)

.PHONY: test
test: ## Unit + integration tests with race detector
	go test -race -shuffle=on -count=1 ./...

.PHONY: test-cover
test-cover: ## Tests with coverage report
	go test -race -count=1 -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out | tail -1

.PHONY: test-e2e
test-e2e: ## Hardware tests against a connected HERO9 (read-only)
	GOPRO9_E2E=1 go test -tags e2e -count=1 -v ./test/e2e/...

.PHONY: test-e2e-mutating
test-e2e-mutating: ## Hardware tests that change camera state (settings snapshot/restore)
	GOPRO9_E2E=1 GOPRO9_E2E_MUTATING=1 go test -tags e2e -count=1 -v ./test/e2e/...

.PHONY: fuzz
fuzz: ## Run fuzz targets briefly
	go test -run=^$$ -fuzz=Fuzz -fuzztime=20s ./pkg/gopro/labs/...
	go test -run=^$$ -fuzz=Fuzz -fuzztime=20s ./pkg/gopro/gpmf/...

.PHONY: lint
lint: ## golangci-lint
	golangci-lint run ./...

.PHONY: vuln
vuln: ## govulncheck
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...


.PHONY: snapshot
snapshot: ## GoReleaser snapshot build
	goreleaser release --snapshot --clean --skip=publish,sign

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf bin dist coverage.out coverage.html
