# grid — less for your data
#
# Run `make` on its own to list the targets.

BINARY := grid
MODULE := github.com/farrellm/grid

# Version comes from git, falling back to "dev" outside a checkout. It is
# stamped into internal/cli.version, which `grid --version` reports.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X '$(MODULE)/internal/cli.version=$(VERSION)'
GOFLAGS := -trimpath -ldflags "$(LDFLAGS)"

# Where `go install` puts things: GOBIN if set, else GOPATH/bin.
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(shell go env GOPATH)/bin
endif

.DEFAULT_GOAL := help

.PHONY: help
help: ## list the available targets
	@echo "grid $(VERSION)"
	@echo
	@awk 'BEGIN {FS = ":.*## "} /^[a-z][a-zA-Z0-9_-]*:.*## / \
		{printf "  \033[1m%-12s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo
	@echo "  install puts the binary in $(GOBIN)"

.PHONY: build
build: ## build ./grid
	go build $(GOFLAGS) -o $(BINARY) .

.PHONY: install
install: ## build and install grid (see the path below)
	go install $(GOFLAGS) .
	@echo "installed $(BINARY) $(VERSION) -> $(GOBIN)/$(BINARY)"
	@command -v $(BINARY) >/dev/null 2>&1 || \
		echo "note: $(GOBIN) is not on your PATH"

.PHONY: uninstall
uninstall: ## remove the installed grid
	rm -f $(GOBIN)/$(BINARY)
	@echo "removed $(GOBIN)/$(BINARY)"

.PHONY: test
test: ## run the tests
	go test ./...

.PHONY: race
race: ## run the tests under the race detector
	go test -race ./...

.PHONY: bench
bench: ## run the benchmarks
	go test -run '^$$' -bench . -benchmem ./...

.PHONY: benchstat
benchstat: ## compare bench.old against bench.new (see `make bench-new`)
	benchstat bench.old bench.new

.PHONY: bench-old bench-new
bench-old: ## record a benchmark baseline in bench.old
	go test -run '^$$' -bench . -benchmem -count=10 ./... > bench.old
bench-new: ## record benchmark results in bench.new, then compare
	go test -run '^$$' -bench . -benchmem -count=10 ./... > bench.new
	$(MAKE) benchstat

.PHONY: cover
cover: ## run the tests and open a coverage report
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

.PHONY: fmt
fmt: ## format the source
	gofmt -w .

.PHONY: vet
vet: ## run go vet
	go vet ./...

.PHONY: lint
lint: ## run golangci-lint (see .golangci.yml)
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint is not installed; get it with"; \
		echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest"; \
		exit 1; }
	golangci-lint run

.PHONY: tidy
tidy: ## tidy go.mod and go.sum
	go mod tidy

.PHONY: check
check: ## everything CI runs: format check, vet, lint, race tests
	@test -z "$$(gofmt -l .)" || { echo "unformatted files:"; gofmt -l .; exit 1; }
	go vet ./...
	$(MAKE) lint
	go test -race ./...

.PHONY: fixtures
fixtures: ## regenerate the files in testdata
	go run testdata/gen.go

.PHONY: run
run: build ## build, then browse testdata/sample.csv
	./$(BINARY) testdata/sample.csv

.PHONY: clean
clean: ## remove build output
	rm -f $(BINARY) coverage.out
