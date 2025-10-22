# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build orchestrator binary
# Use CGO_ENABLED=0 for static binary
# GOARCH will be set by docker buildx or defaults to the build platform
RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o holt-orchestrator ./cmd/orchestrator

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS (if needed in future)
RUN apk --no-cache add ca-certificates

# M3.4: Create holt user with known UID for Docker socket access
# The container will be launched with GroupAdd to match host's docker GID
RUN addgroup -g 1000 holt && \
    adduser -D -u 1000 -G holt holt

# Copy binary from builder
COPY --from=builder /build/holt-orchestrator /usr/local/bin/holt-orchestrator

# M3.4: Run as holt user (non-root)
# Docker socket access granted via GroupAdd in docker run command
USER holt

# Expose health check port
EXPOSE 8080

# Set entrypoint
ENTRYPOINT ["/usr/local/bin/holt-orchestrator"]

# Document required environment variables
ENV HOLT_INSTANCE_NAME=""
ENV REDIS_URL=""
