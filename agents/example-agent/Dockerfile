# Example Sett agent for M2.2 testing
# This is a minimal agent that demonstrates the cub architecture

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

# Runtime stage - minimal Alpine image
FROM alpine:latest

# Install ca-certificates for HTTPS connections
RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy cub binary from builder
COPY --from=builder /build/cub /app/cub

# Copy run script
COPY agents/example-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Run as non-root user (security best practice)
RUN adduser -D -u 1000 agent
USER agent

# Entrypoint is the cub binary
ENTRYPOINT ["/app/cub"]
