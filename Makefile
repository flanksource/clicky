## Tool Binaries
LOCALBIN ?= $(shell pwd)/.bin
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
GOLANGCI_LINT_VERSION ?= v2.8.0

GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS ?= -X main.commit=$(GIT_COMMIT) -X main.date=$(BUILD_DATE)

.PHONY: test build clean install task-ui


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
.PHONY: lint
lint: golangci-lint
	$(GOLANGCI_LINT) run ./...
	go vet ./...

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	test -s $(LOCALBIN)/golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(LOCALBIN) $(GOLANGCI_LINT_VERSION)


# Go module directories (each has its own go.mod)
GO_MODULES := . valkey examples examples/uber_demo

# Format code and tidy all modules
fmt:
	gofmt -s -w .
	@for dir in $(GO_MODULES); do \
		echo "go mod tidy: $$dir"; \
		(cd $$dir && go mod tidy) || exit 1; \
	done

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
