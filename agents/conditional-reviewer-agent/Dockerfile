# Conditional Reviewer Agent Dockerfile for M3.3 Testing
# Rejects version 1, approves version 2+
# Enables testing of automated feedback loop iteration

# Build stage - compile the pup binary
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Copy Go module files
COPY go.mod go.sum ./
RUN go mod download

# Copy pup source code
COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY pkg/blackboard ./pkg/blackboard

# Build the pup binary
RUN CGO_ENABLED=0 GOOS=linux go build -o pup ./cmd/pup

# Runtime stage - Alpine with wget for health checks
FROM alpine:latest

# Install ca-certificates, wget (for health checks), and grep (for version parsing)
RUN apk --no-cache add ca-certificates wget grep

WORKDIR /app

# Copy pup binary from builder
COPY --from=builder /build/pup /app/pup

# Copy run script
COPY agents/conditional-reviewer-agent/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Run as non-root user (security best practice)
RUN adduser -D -u 1000 agent
USER agent

# Entrypoint is the pup binary
ENTRYPOINT ["/app/pup"]
