# Build stage - compile agent pup
FROM golang:1.24-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY cmd/pup ./cmd/pup
COPY internal/pup ./internal/pup
COPY internal/config ./internal/config
COPY pkg/blackboard ./pkg/blackboard

RUN CGO_ENABLED=0 GOOS=linux go build -o pup ./cmd/pup

# Runtime stage
FROM alpine:latest

# Add git for commits and jq for parsing context
RUN apk --no-cache add git jq

WORKDIR /app

# Copy pup binary
COPY --from=builder /build/pup /app/pup

# Copy your tool script
COPY demos/recipe-generator/agents/drafter/run.sh /app/run.sh
RUN chmod +x /app/run.sh

# Copy the bid script
COPY demos/recipe-generator/agents/drafter/bid.sh /app/bid.sh
RUN chmod +x /app/bid.sh

# Run as non-root
RUN adduser -D -u 1000 agent
USER agent

ENTRYPOINT ["/app/pup"]
