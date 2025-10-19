# Always-Reject Reviewer Agent Dockerfile for M3.3 Testing
# Always rejects with feedback regardless of version
# Enables testing of max iterations termination

# Build stage - compile the cub binary
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy cub source code
COPY cmd/cub ./cmd/cub
COPY internal/cub ./internal/cub
COPY pkg/blackboard ./pkg/blackboard

# Build the cub binary
RUN CGO_ENABLED=0 GOOS=linux go build -o cub ./cmd/cub

# Runtime stage - Alpine with wget for health checks
FROM alpine:latest

# Install ca-certificates, wget (for health checks), and grep (for version parsing)
RUN apk --no-cache add ca-certificates wget grep

WORKDIR /app

# Copy cub binary from builder
COPY --from=builder /build/cub /app/cub

# Copy run script
COPY agents/always-reject-reviewer-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Run as non-root user (security best practice)
RUN adduser -D -u 1000 agent
USER agent

# Entrypoint is the cub binary
ENTRYPOINT ["/app/cub"]
