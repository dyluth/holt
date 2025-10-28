# Build stage - compile agent pup
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY pkg/blackboard ./pkg/blackboard

RUN CGO_ENABLED=0 GOOS=linux go build -o pup ./cmd/pup

# Runtime stage
FROM alpine:latest

# Install dependencies: git for commits, jq for JSON parsing
RUN apk --no-cache add git jq

WORKDIR /app

# Copy pup binary
COPY --from=builder /build/pup /app/pup

# Copy agent scripts
COPY demos/terraform-generator/agents/doc-generator/run.sh /app/run.sh
COPY demos/terraform-generator/agents/doc-generator/bid.sh /app/bid.sh
RUN chmod +x /app/run.sh /app/bid.sh

# Run as non-root user
RUN adduser -D -u 1000 agent
USER agent

ENTRYPOINT ["/app/pup"]
