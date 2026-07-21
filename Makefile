## Tool Binaries
LOCALBIN ?= $(shell pwd)/.bin
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint
PKGSITE ?= $(LOCALBIN)/pkgsite

## Tool Versions
GOLANGCI_LINT_VERSION ?= v2.8.0
PKGSITE_VERSION ?= v0.1.0

## Docs server
DOCS_PORT ?= 8089

GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS ?= -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)

.PHONY: test build clean install task-ui tidy

test:
	go test -v ./...

# Run OpenAPI integration tests (builds binary, starts server)
test\:openapi:
	go test -v -tags integration -timeout 60s ./rpc/ -run TestOpenAPIServe_E2E

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o clicky ./cmd/clicky/

# Serve Go package documentation locally and open a browser.
# Override the port with `make docs DOCS_PORT=9000` if it is in use.
# pkgsite is installed to $(LOCALBIN) (pinned + cached) and run directly, so it
# starts fast and shuts down cleanly on Ctrl-C instead of orphaning the port.
.PHONY: docs
docs: pkgsite
	$(PKGSITE) -http=:$(DOCS_PORT) -open .

.PHONY: pkgsite
pkgsite: $(PKGSITE) ## Download pkgsite locally if necessary.
$(PKGSITE): $(LOCALBIN)
	test -s $(PKGSITE) || GOBIN=$(LOCALBIN) go install golang.org/x/pkgsite/cmd/pkgsite@$(PKGSITE_VERSION)

# Build the task-ui frontend bundle (Preact + Vite → single IIFE)
task-ui:
	cd task/ui && npm ci && npm run build

# Clean build artifacts
clean:
	rm -f clicky
	rm -rf task/ui/dist task/ui/node_modules
	go clean

# Install dependencies
install: build
	mv clicky /usr/local/bin/clicky

# Run linter
.PHONY: lint lint-clicky-ui
lint: golangci-lint
	$(GOLANGCI_LINT) run ./...
	go vet ./...

lint-clicky-ui:
	cd examples/enitity/webapp && pnpm install --frozen-lockfile && pnpm run lint:clicky-ui

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)


# Go module directories (each has its own go.mod)
GO_MODULES := . aichat valkey examples examples/enitity examples/uber_demo

# Run go mod tidy in every module
.PHONY: tidy
tidy:
	@for dir in $(GO_MODULES); do \
		echo "go mod tidy: $$dir"; \
		(cd $$dir && go mod tidy) || exit 1; \
	done

# Format code and tidy all modules
.PHONY: fmt
fmt: tidy
	gofmt -s -w .

# Run all checks
check: fmt lint test

# Run goreleaser in snapshot mode (local testing)
release-snapshot:
	goreleaser release --snapshot --clean

# Run goreleaser check
release-check:
	goreleaser check

# Build Docker image locally
docker-build:
	docker build -t clicky:latest .

# Run Docker container
docker-run:
	docker run --rm clicky:latest --help


# Default target
all: install fmt test build

$(LOCALBIN):
	mkdir -p $(LOCALBIN)
