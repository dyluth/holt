# Example Git Agent Dockerfile for M2.5
# Demonstrates CodeCommit workflow with Git integration

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

# Runtime stage - Alpine with Git
FROM alpine:latest

# Install git and ca-certificates
RUN apk --no-cache add git ca-certificates

WORKDIR /app

# Copy cub binary from builder
COPY --from=builder /build/cub /app/cub

# Copy run script
COPY agents/example-git-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Run as non-root user (security best practice)
RUN adduser -D -u 1000 agent
USER agent

# Configure git (required for commits) - MUST be done AFTER switching to agent user
# so the config is written to /home/agent/.gitconfig instead of /root/.gitconfig
RUN git config --global user.email "agent@sett.local" && \
    git config --global user.name "Sett Git Agent" && \
    git config --global --add safe.directory /workspace

# Entrypoint is the cub binary
ENTRYPOINT ["/app/cub"]
