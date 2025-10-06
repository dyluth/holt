.PHONY: help test test-verbose test-integration coverage coverage-html lint build build-orchestrator docker-orchestrator clean install

# Use Go 1.24 if available in /usr/local/go, otherwise use system go
GO := $(shell [ -x /usr/local/go/bin/go ] && echo /usr/local/go/bin/go || echo go)

# Default target
help:
	@echo "Sett Development Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  test                - Run all unit tests"
	@echo "  test-verbose        - Run all unit tests with verbose output"
	@echo "  test-integration    - Run integration tests (requires Docker)"
	@echo "  coverage            - Run tests and show coverage report"
	@echo "  coverage-html       - Generate HTML coverage report"
	@echo "  lint                - Run go vet and staticcheck"
	@echo "  build               - Build the sett CLI binary"
	@echo "  build-orchestrator  - Build the orchestrator binary"
	@echo "  docker-orchestrator - Build orchestrator Docker image"
	@echo "  install             - Install sett binary to GOPATH/bin"
	@echo "  clean               - Remove build artifacts"

# Run all tests
test:
	@echo "Running tests..."
	@$(GO) test ./...

# Run all tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	@$(GO) test -v ./...

# Run tests with coverage
coverage:
	@echo "Running tests with coverage..."
	@$(GO) test -coverprofile=coverage.out ./...
	@echo ""
	@echo "Coverage by package:"
	@$(GO) tool cover -func=coverage.out
	@echo ""
	@echo "To view HTML coverage report, run: make coverage-html"

# Generate HTML coverage report
coverage-html: coverage
	@echo "Generating HTML coverage report..."
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@echo "Open in browser: open coverage.html (macOS) or xdg-open coverage.html (Linux)"

# Run linters
lint:
	@echo "Running go vet..."
	@$(GO) vet ./...
	@echo "✓ go vet passed"
	@if command -v staticcheck >/dev/null 2>&1; then \
		echo "Running staticcheck..."; \
		staticcheck ./...; \
		echo "✓ staticcheck passed"; \
	else \
		echo "⚠️  staticcheck not installed (optional)"; \
		echo "   Install with: $(GO) install honnef.co/go/tools/cmd/staticcheck@latest"; \
	fi

# Run integration tests (requires Docker)
test-integration:
	@echo "Running integration tests..."
	@$(GO) test -v -tags=integration ./cmd/orchestrator

# Build the sett binary
build:
	@echo "Building sett CLI..."
	@mkdir -p bin
	@$(GO) build -o bin/sett ./cmd/sett
	@echo "✓ Built: bin/sett"

# Build the orchestrator binary
build-orchestrator:
	@echo "Building orchestrator..."
	@mkdir -p bin
	@$(GO) build -o bin/sett-orchestrator ./cmd/orchestrator
	@echo "✓ Built: bin/sett-orchestrator"

# Build orchestrator Docker image
docker-orchestrator:
	@echo "Building orchestrator Docker image..."
	@docker build -f Dockerfile.orchestrator -t sett-orchestrator:latest .
	@echo "✓ Built: sett-orchestrator:latest"

# Install sett binary
install:
	@echo "Installing sett..."
	@$(GO) install ./cmd/sett
	@echo "✓ Installed to: $$($(GO) env GOPATH)/bin/sett"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✓ Clean complete"
