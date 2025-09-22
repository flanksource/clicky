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

# Run linter (if available)
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed, skipping lint"; \
	fi


# Format code
fmt:
	go fmt ./...

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
