## Tool Binaries
LOCALBIN ?= $(shell pwd)/.bin
GOLANGCI_LINT ?= $(LOCALBIN)/golangci-lint

## Tool Versions
GOLANGCI_LINT_VERSION ?= v2.8.0

.PHONY: test build clean install


test: build
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Build the binary
build:
	go build -o clicky ./cmd/clicky/

# Clean build artifacts
clean:
	rm -f clicky
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


# Format code
fmt:
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
