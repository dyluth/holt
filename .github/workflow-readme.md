# GitHub Actions Workflows

This directory contains the CI/CD workflows for the Holt project.

## Workflows

### `ci.yml` - Continuous Integration

**Triggers:** Push to `main`/`develop`, Pull Requests

This is the primary CI workflow that runs on every push and pull request. It runs tests in parallel for faster feedback:

**Jobs:**
- **unit-tests**: Fast unit tests and pup tests (no Docker required)
- **lint**: Code quality checks with `go vet` and `staticcheck`
- **integration-tests**: Integration tests and E2E tests (requires Docker)
- **all-tests**: Summary job that confirms all tests passed

**Typical runtime:** 3-5 minutes

### `full-test-suite.yml` - Complete Test Suite

**Triggers:**
- Daily schedule (2 AM UTC)
- Manual trigger via workflow_dispatch
- Release tags (`v*`)

This workflow runs the complete test suite using `make test-all`, which includes:
- Unit tests
- Pup tests
- Orchestrator integration tests
- E2E tests (all Phase 1 and Phase 2 tests)
- Performance tests

**Typical runtime:** 10-15 minutes

## Running Tests Locally

To run the same tests locally:

```bash
# Quick unit tests
make test

# All pup tests
make test-pup

# Integration tests (requires Docker)
make test-integration

# E2E tests (requires Docker)
make test-e2e

# Complete test suite
make test-all
```

## Requirements

The integration and E2E tests require:
- Docker (for running Redis, orchestrator, and agent containers)
- Docker Buildx (for building images)
- Go 1.23+

## Test Environment

The workflows use Ubuntu runners with Docker-in-Docker support, which matches the development environment in Claude Code and ensures consistent test behavior across local and CI environments.
