# Example Git Agent Dockerfile for M2.5
# Demonstrates CodeCommit workflow with Git integration

# Build stage - compile the pup binary
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy pup source code
COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY internal/config ./internal/config
COPY pkg/blackboard ./pkg/blackboard

# Build the pup binary
RUN CGO_ENABLED=0 GOOS=linux go build -o pup ./cmd/pup

# Runtime stage - Alpine with Git
FROM alpine:latest

# Install git and ca-certificates
RUN apk --no-cache add git ca-certificates

WORKDIR /app

# Copy pup binary from builder
COPY --from=builder /build/pup /app/pup

# Copy run script
COPY agents/example-git-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Run as non-root user (security best practice)
RUN adduser -D -u 1000 agent
USER agent

# Configure git (required for commits) - MUST be done AFTER switching to agent user
# so the config is written to /home/agent/.gitconfig instead of /root/.gitconfig
RUN git config --global user.email "agent@holt.local" && \
    git config --global user.name "Holt Git Agent" && \
    git config --global --add safe.directory /workspace

# Entrypoint is the pup binary
ENTRYPOINT ["/app/pup"]
