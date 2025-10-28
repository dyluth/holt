# Build stage - compile agent pup
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY pkg/blackboard ./pkg/blackboard

RUN CGO_ENABLED=0 GOOS=linux go build -o pup ./cmd/pup

# Runtime stage - use Node.js for markdownlint-cli2
FROM node:20-alpine

# Install git and jq
RUN apk --no-cache add git jq

# Install markdownlint-cli2 globally
RUN npm install -g markdownlint-cli2

WORKDIR /app

# Copy pup binary from builder
COPY --from=builder /build/pup /app/pup

# Copy agent scripts
COPY demos/terraform-generator/agents/markdown-lint/run.sh /app/run.sh
COPY demos/terraform-generator/agents/markdown-lint/bid.sh /app/bid.sh
RUN chmod +x /app/run.sh /app/bid.sh

# Run as non-root user (node image already has UID 1000, use existing node user)
USER node

ENTRYPOINT ["/app/pup"]
