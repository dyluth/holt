# Sett Orchestrator

The Sett orchestrator is a lightweight, event-driven component that watches for new artefacts on the blackboard and creates corresponding claims. It serves as the central coordination engine for the Sett system.

## Overview

**Phase 1 Scope:**
- Subscribes to `artefact_events` Pub/Sub channel
- Creates claims in `pending_review` status for non-Terminal artefacts
- Skips Terminal artefacts (no claim creation)
- Publishes claims to `claim_events` channel
- Provides `/healthz` HTTP endpoint for health checks
- Implements graceful shutdown on SIGTERM/SIGINT

**Not in Phase 1:**
- Agent bidding logic (Phase 2)
- Claim phase transitions (pending_review → pending_parallel → pending_exclusive)
- Agent registry loading from `sett.yml`
- Failure artefact creation

## Architecture

The orchestrator follows an event-driven architecture:

```
[Artefact Created] → [artefact_events] → [Orchestrator]
                                              ↓
                                        [Create Claim]
                                              ↓
                                        [claim_events] → [Agents (Phase 2)]
```

### Key Features

**Idempotency**: Checks for existing claims before creating new ones to handle duplicate events gracefully.

**Terminal Detection**: Correctly identifies Terminal structural types and skips claim creation.

**Fail-Fast**: Exits cleanly on Redis connection failures, relying on container runtime for restart.

**Graceful Shutdown**: Handles SIGTERM/SIGINT signals, cleanly closing Redis connections and HTTP server.

## Configuration

### Environment Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `SETT_INSTANCE_NAME` | Yes | Unique identifier for this Sett instance | `prod`, `dev`, `test` |
| `REDIS_URL` | Yes | Redis connection string | `redis://localhost:6379` |

### Example

```bash
export SETT_INSTANCE_NAME=prod
export REDIS_URL=redis://localhost:6379
./sett-orchestrator
```

## Health Checks

The orchestrator exposes a health check endpoint at `http://localhost:8080/healthz`.

**Responses:**

**200 OK** (Healthy):
```json
{
  "status": "healthy",
  "redis": "connected"
}
```

**503 Service Unavailable** (Unhealthy):
```json
{
  "status": "unhealthy",
  "redis": "disconnected",
  "error": "connection refused"
}
```

## Building

### Local Binary

```bash
make build-orchestrator
./bin/sett-orchestrator
```

### Docker Image

```bash
make docker-orchestrator
docker run -e SETT_INSTANCE_NAME=test -e REDIS_URL=redis://redis:6379 sett-orchestrator:latest
```

## Testing

### Unit Tests

```bash
go test ./internal/orchestrator
```

### Integration Tests

Requires Docker daemon running:

```bash
make test-integration
```

Or manually:

```bash
go test -v -tags=integration ./cmd/orchestrator
```

## Logging

The orchestrator emits structured JSON logs for all significant events:

**Startup:**
```json
{"level":"info","event":"orchestrator_startup","instance":"prod"}
```

**Claim Created:**
```json
{"level":"info","event":"claim_created","artefact_id":"uuid","claim_id":"uuid","status":"pending_review","latency_ms":45}
```

**Terminal Skipped:**
```json
{"level":"info","event":"terminal_skipped","artefact_id":"uuid","type":"FinalReport"}
```

**Duplicate Artefact:**
```json
{"level":"warn","event":"duplicate_artefact","artefact_id":"uuid","existing_claim_id":"uuid"}
```

## Troubleshooting

### Orchestrator exits immediately

**Check environment variables:**
```bash
echo $SETT_INSTANCE_NAME
echo $REDIS_URL
```

**Check Redis connectivity:**
```bash
redis-cli -u $REDIS_URL ping
```

### Claims not being created

**Verify orchestrator is running:**
```bash
docker ps | grep orchestrator
```

**Check health endpoint:**
```bash
curl http://localhost:8080/healthz
```

**Review logs:**
```bash
docker logs sett-orchestrator-{instance}
```

**Verify Redis subscription:**
```bash
redis-cli
PUBSUB CHANNELS sett:*
```

### High latency (> 100ms claim creation)

Check Redis performance:
```bash
redis-cli --latency
redis-cli --latency-history
```

Review orchestrator logs for `latency_ms` field in `claim_created` events.

## Performance

**Target Metrics (Phase 1):**
- Claim creation latency: < 100ms
- Artefact throughput: < 100 artefacts/minute
- Memory usage: < 50 MB baseline
- Startup time: < 5 seconds
- Graceful shutdown: < 10 seconds

## Integration with Sett CLI

**Phase 1**: Orchestrator runs independently with manual setup.

**M1.6 (Future)**: `sett up` will automatically build and start the orchestrator container, replacing the busybox placeholder from M1.4.

## References

- **Design Document**: `design/features/phase-1-heartbeat/M1.5-orchestrator-claim-engine.md`
- **Orchestrator Component Spec**: `design/sett-orchestrator-component.md`
- **Blackboard Client**: `pkg/blackboard/client.go`
- **System Specification**: `design/sett-system-specification.md`
