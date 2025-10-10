.PHONY: help test test-verbose test-integration test-e2e test-all coverage coverage-html lint build build-orchestrator build-cub docker-orchestrator build-all clean install test-cub

# Use Go 1.24 if available in /usr/local/go, otherwise use system go
GO := $(shell [ -x /usr/local/go/bin/go ] && echo /usr/local/go/bin/go || echo go)

# Default target
help:
	@echo "Sett Development Makefile"
	@echo ""
	@echo "Targets:"
	@echo ""
	@echo "Common workflows:"
	@echo "  build-all           - Build everything (CLI + orchestrator + cub)"
	@echo "  build               - Build the sett CLI binary"
	@echo "  docker-orchestrator - Build orchestrator Docker image (required for 'sett up')"
	@echo ""
	@echo "Testing:"
	@echo "  test                - Run all unit tests"
	@echo "  test-verbose        - Run all unit tests with verbose output"
	@echo "  test-cub            - Run cub unit and integration tests"
	@echo "  test-integration    - Run orchestrator integration tests (requires Docker)"
	@echo "  test-e2e            - Run Phase 2 E2E test suite (requires Docker)"
	@echo "  test-all            - Run ALL tests (unit + cub + integration + e2e)"
	@echo "  coverage            - Run tests and show coverage report"
	@echo "  coverage-html       - Generate HTML coverage report"
	@echo "  lint                - Run go vet and staticcheck"
	@echo ""
	@echo "Development:"
	@echo "  build-orchestrator  - Build orchestrator binary (for debugging only)"
	@echo "  build-cub           - Build agent cub binary"
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

# Run orchestrator integration tests (requires Docker)
test-integration:
	@echo "Running orchestrator integration tests..."
	@$(GO) test -v -tags=integration ./cmd/orchestrator

# Run Phase 2 E2E test suite (requires Docker)
test-e2e:
	@echo "Running Phase 2 E2E test suite..."
	@echo "Building example-git-agent Docker image..."
	@docker build -q -t example-git-agent:latest -f agents/example-git-agent/Dockerfile . > /dev/null
	@docker build -q -t example-agent:latest -f agents/example-agent/Dockerfile . > /dev/null
	@echo "Running E2E tests..."
	@$(GO) test -v -timeout 15m -tags=integration ./cmd/sett/commands/e2e_*
	@echo "✓ All E2E tests passed"

# Run all tests (unit + cub + integration + e2e)
test-all: test test-cub test-integration test-e2e
	@echo ""
	@echo "========================================"
	@echo "✓ ALL TESTS PASSED"
	@echo "========================================"
	@echo "  Unit tests:        ✓"
	@echo "  Cub tests:         ✓"
	@echo "  Integration tests: ✓"
	@echo "  E2E tests:         ✓"
	@echo ""

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

# Build the agent cub binary
build-cub:
	@echo "Building agent cub..."
	@mkdir -p bin
	@$(GO) build -o bin/sett-cub ./cmd/cub
	@echo "✓ Built: bin/sett-cub"

# Run cub unit and integration tests
test-cub:
	@echo "Running cub tests..."
	@$(GO) test -v -race ./internal/cub
	@$(GO) test -v -timeout 60s ./cmd/cub
	@echo "✓ All cub tests passed"

# Build orchestrator Docker image
docker-orchestrator:
	@echo "Building orchestrator Docker image..."
	@docker build -f Dockerfile.orchestrator -t sett-orchestrator:latest .
	@echo "✓ Built: sett-orchestrator:latest"

# Build everything (CLI + orchestrator Docker image + cub)
build-all: build build-cub docker-orchestrator
	@echo ""
	@echo "✓ Build complete!"
	@echo "  - CLI binary: bin/sett"
	@echo "  - Cub binary: bin/sett-cub"
	@echo "  - Orchestrator image: sett-orchestrator:latest"
	@echo ""
	@echo "Ready to use: ./bin/sett up"

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
