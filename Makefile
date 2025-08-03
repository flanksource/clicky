.PHONY: test build clean install

# Run tests (focus on main functionality)
test:
	go test -v -run "Test(FormatterMatrix|DateFormatting|NestedMaps)" ./...

# Run all tests (including legacy)
test-all:
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
install:
	go mod download
	go mod tidy

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

# Setup development environment
dev-setup:
	@echo "Installing development tools..."
	go install github.com/goreleaser/goreleaser@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Default target
all: install fmt test build