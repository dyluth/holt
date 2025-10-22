.PHONY: help test test-verbose test-integration test-e2e test-all coverage coverage-html lint build build-orchestrator build-pup docker-orchestrator build-all clean install test-pup

# Use Go 1.24 if available in /usr/local/go, otherwise use system go
GO := $(shell [ -x /usr/local/go/bin/go ] && echo /usr/local/go/bin/go || echo go)

# Default target
help:
	@echo "Holt Development Makefile"
	@echo ""
	@echo "Targets:"
	@echo ""
	@echo "Common workflows:"
	@echo "  build-all           - Build everything (CLI + orchestrator + pup)"
	@echo "  build               - Build the holt CLI binary for current platform"
	@echo "  docker-orchestrator - Build orchestrator Docker image (required for 'holt up')"
	@echo ""
	@echo "Cross-compilation:"
	@echo "  build-darwin-arm64  - Build for macOS ARM64 (M1/M2/M3 Macs)"
	@echo "  build-darwin-amd64  - Build for macOS Intel"
	@echo "  build-linux-arm64   - Build for Linux ARM64"
	@echo "  build-linux-amd64   - Build for Linux AMD64"
	@echo ""
	@echo "Testing:"
	@echo "  test                - Run all unit tests"
	@echo "  test-verbose        - Run all unit tests with verbose output"
	@echo "  test-pup            - Run pup unit and integration tests"
	@echo "  test-integration    - Run orchestrator integration tests (requires Docker)"
	@echo "  test-e2e            - Run Phase 2 E2E test suite (requires Docker)"
	@echo "  test-all            - Run ALL tests (unit + pup + integration + e2e)"
	@echo "  coverage            - Run tests and show coverage report"
	@echo "  coverage-html       - Generate HTML coverage report"
	@echo "  lint                - Run go vet and staticcheck"
	@echo ""
	@echo "Development:"
	@echo "  build-orchestrator  - Build orchestrator binary (for debugging only)"
	@echo "  build-pup           - Build agent pup binary"
	@echo "  install             - Install holt binary to GOPATH/bin"
	@echo "  clean               - Remove build artifacts"

# Run all tests (depends on binaries being built)
test: build build-pup
	@echo "Running tests..."
	@$(GO) test ./...

# Run all tests with verbose output
test-verbose: build build-pup
	@echo "Running tests (verbose)..."
	@$(GO) test -v ./...

# Run tests with coverage
coverage: build build-pup
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
test-integration: build-orchestrator
	@echo "Running orchestrator integration tests..."
	@$(GO) test -v -tags=integration ./cmd/orchestrator

# Run Phase 2 E2E test suite (requires Docker)
# M3.4: Now depends on docker-orchestrator to ensure latest orchestrator image is available
test-e2e: build build-pup docker-orchestrator
	@echo "Running Phase 2 E2E test suite..."
	@echo "Building example-git-agent Docker image..."
	@docker build -q -t example-git-agent:latest -f agents/example-git-agent/Dockerfile . > /dev/null
	@docker build -q -t example-agent:latest -f agents/example-agent/Dockerfile . > /dev/null
	@echo "Running E2E tests..."
	@$(GO) test -v -timeout 15m -tags=integration -run="TestE2E|TestPerformance" ./cmd/holt/commands
	@echo "✓ All E2E tests passed"

# Run all tests (unit + pup + integration + e2e)
test-all: test test-pup test-integration test-e2e
	@echo ""
	@echo "========================================"
	@echo "✓ ALL TESTS PASSED"
	@echo "========================================"
	@echo "  Unit tests:        ✓"
	@echo "  Pup tests:         ✓"
	@echo "  Integration tests: ✓"
	@echo "  E2E tests:         ✓"
	@echo ""

# Build the holt binary
build:
	@echo "Building holt CLI..."
	@mkdir -p bin
	@$(GO) build -o bin/holt ./cmd/holt
	@echo "✓ Built: bin/holt"

# Cross-compile for macOS ARM64 (M1/M2/M3 Macs)
build-darwin-arm64:
	@echo "Building holt CLI for macOS ARM64..."
	@mkdir -p bin
	@GOOS=darwin GOARCH=arm64 $(GO) build -o bin/holt-darwin-arm64 ./cmd/holt
	@echo "✓ Built: bin/holt-darwin-arm64"

# Cross-compile for macOS Intel
build-darwin-amd64:
	@echo "Building holt CLI for macOS Intel..."
	@mkdir -p bin
	@GOOS=darwin GOARCH=amd64 $(GO) build -o bin/holt-darwin-amd64 ./cmd/holt
	@echo "✓ Built: bin/holt-darwin-amd64"

# Cross-compile for Linux ARM64
build-linux-arm64:
	@echo "Building holt CLI for Linux ARM64..."
	@mkdir -p bin
	@GOOS=linux GOARCH=arm64 $(GO) build -o bin/holt-linux-arm64 ./cmd/holt
	@echo "✓ Built: bin/holt-linux-arm64"

# Cross-compile for Linux AMD64
build-linux-amd64:
	@echo "Building holt CLI for Linux AMD64..."
	@mkdir -p bin
	@GOOS=linux GOARCH=amd64 $(GO) build -o bin/holt-linux-amd64 ./cmd/holt
	@echo "✓ Built: bin/holt-linux-amd64"

# Build the orchestrator binary
build-orchestrator:
	@echo "Building orchestrator..."
	@mkdir -p bin
	@$(GO) build -o bin/holt-orchestrator ./cmd/orchestrator
	@echo "✓ Built: bin/holt-orchestrator"

# Build the agent pup binary
build-pup:
	@echo "Building agent pup..."
	@mkdir -p bin
	@$(GO) build -o bin/holt-pup ./cmd/pup
	@echo "✓ Built: bin/holt-pup"

# Run pup unit and integration tests
test-pup: build-pup
	@echo "Running pup tests..."
	@$(GO) test -v -race ./internal/pup
	@$(GO) test -v -timeout 60s ./cmd/pup
	@echo "✓ All pup tests passed"

# Build orchestrator Docker image
docker-orchestrator:
	@echo "Building orchestrator Docker image..."
	@docker build -f Dockerfile.orchestrator -t holt-orchestrator:latest .
	@echo "✓ Built: holt-orchestrator:latest"

# Build everything (CLI + orchestrator Docker image + pup)
build-all: build build-pup docker-orchestrator
	@echo ""
	@echo "✓ Build complete!"
	@echo "  - CLI binary: bin/holt"
	@echo "  - Pup binary: bin/holt-pup"
	@echo "  - Orchestrator image: holt-orchestrator:latest"
	@echo ""
	@echo "Ready to use: ./bin/holt up"

# Install holt binary
install:
	@echo "Installing holt..."
	@$(GO) install ./cmd/holt
	@echo "✓ Installed to: $$($(GO) env GOPATH)/bin/holt"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html
	@echo "✓ Clean complete"
