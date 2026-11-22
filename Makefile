# Makefile: shortcuts for the everyday Go commands used in this project.
# Run `make help` to list the targets.

# staticcheck is installed with `go install`, which puts it in $(go env GOPATH)/bin.
# That folder is not always on PATH, so the full path is spelled out here.
STATICCHECK := $(shell go env GOPATH)/bin/staticcheck

.PHONY: help test test-short cover lint vet fmt build run doc clean

help: ## List the available targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

test: ## Run all tests with the race detector
	go test -race ./...

test-short: ## Run the fast tests only (skips the slow exhaustive solver test)
	go test -short ./...

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
