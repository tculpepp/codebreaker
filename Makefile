# Makefile: shortcuts for the everyday Go commands used in this project.
# Run `make help` to list the targets.

# staticcheck is installed with `go install`, which puts it in $(go env GOPATH)/bin.
# That folder is not always on PATH, so the full path is spelled out here.
STATICCHECK := $(shell go env GOPATH)/bin/staticcheck

# The packages the web version uses, which must also work as WebAssembly
# (docs/web-plan.md §8). The terminal-only packages (input, ui, app) are
# left out.
WASM_PKGS := ./internal/code ./internal/game ./internal/solver ./internal/player ./internal/message ./internal/web

# go_js_wasm_exec runs a WebAssembly test binary under Node.js. It ships
# with Go, so it always matches the Go version.
WASM_EXEC := $(shell go env GOROOT)/lib/wasm/go_js_wasm_exec

.PHONY: help test test-short test-wasm cover lint vet fmt build run doc clean

help: ## List the available targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

test: ## Run all tests with the race detector
	go test -race ./...

test-short: ## Run the fast tests only (skips the slow exhaustive solver test)
	go test -short ./...

test-wasm: ## Run the web version's packages as WebAssembly under Node.js (needs node)
	GOOS=js GOARCH=wasm go test -exec="$(WASM_EXEC)" $(WASM_PKGS)

cover: ## Run the tests with coverage and open the HTML report
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1
	go tool cover -html=coverage.out -o coverage.html

vet: ## Run go vet, the built-in checker for suspicious code
	go vet ./...

lint: vet ## Run go vet and staticcheck (including the doc-comment checks)
	$(STATICCHECK) ./...

fmt: ## Format all code with gofmt
	gofmt -w .

build: ## Build the binary into ./bin/
	go build -o bin/codebreaker ./cmd/codebreaker

run: ## Build and run the game
	go run ./cmd/codebreaker

doc: ## Print the docs for every package, as `go doc` shows them
	@for p in $$(go list ./...); do echo "=== $$p"; go doc -all $$p; done

clean: ## Remove build and coverage output
	rm -rf bin coverage.out coverage.html
