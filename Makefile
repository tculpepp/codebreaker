# Makefile: shortcuts for the everyday Go commands used in this project.
# Run `make help` to list the targets.

# staticcheck is installed with `go install`, which puts it in $(go env GOPATH)/bin.
# That folder is not always on PATH, so the full path is spelled out here.
STATICCHECK := $(shell go env GOPATH)/bin/staticcheck

# The packages the web version uses, which must also work as WebAssembly
# (docs/web-plan.md §8). The terminal-only packages (input, ui, app) are
# left out.
WASM_PKGS := ./internal/code ./internal/game ./internal/solver ./internal/player ./internal/message ./internal/web ./internal/web/bridge

# go_js_wasm_exec runs a WebAssembly test binary under Node.js. It ships
# with Go, so it always matches the Go version.
WASM_EXEC := $(shell go env GOROOT)/lib/wasm/go_js_wasm_exec

# Where the web version is built: the files in web/, plus the compiled Go
# program and Go's wasm_exec.js. It's never committed.
DIST := dist

.PHONY: help test test-short test-wasm cover lint vet fmt build run web serve doc clean

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

vet: ## Run go vet, the built-in checker for suspicious code (also on the WebAssembly-only code)
	go vet ./...
	GOOS=js GOARCH=wasm go vet ./internal/web/bridge ./cmd/web

lint: vet ## Run go vet and staticcheck (including the doc-comment checks)
	$(STATICCHECK) ./...
	GOOS=js GOARCH=wasm $(STATICCHECK) ./internal/web/bridge ./cmd/web

fmt: ## Format all code with gofmt
	gofmt -w .

build: ## Build the binary into ./bin/
	go build -o bin/codebreaker ./cmd/codebreaker

run: ## Build and run the game
	go run ./cmd/codebreaker

web: ## Build the browser version into dist/
	rm -rf $(DIST)
	mkdir -p $(DIST)
	cp -R web/. $(DIST)/
	cp "$(shell go env GOROOT)/lib/wasm/wasm_exec.js" $(DIST)/
	GOOS=js GOARCH=wasm go build -ldflags="-s -w" -o $(DIST)/codebreaker.wasm ./cmd/web
	@ls -l $(DIST)/codebreaker.wasm | awk '{printf "codebreaker.wasm: %.1f MB\n", $$5/1048576}'

serve: web ## Build the browser version and serve it at http://localhost:8080
	go run ./tools/serve -dir $(DIST)

doc: ## Print the docs for every package, as `go doc` shows them
	@for p in $$(go list ./...); do echo "=== $$p"; go doc -all $$p; done

clean: ## Remove build and coverage output
	rm -rf bin $(DIST) coverage.out coverage.html
