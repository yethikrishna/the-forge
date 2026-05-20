# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git gcc musl-dev

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o forge .

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates curl tmux bash

# Install agentapi for forge serve
RUN curl -fsSL https://github.com/coder/agentapi/releases/latest/download/agentapi-linux-arm64 -o /usr/local/bin/agentapi && \
    chmod +x /usr/local/bin/agentapi || true

COPY --from=builder /build/forge /usr/local/bin/forge

# Default config
ENV FORGE_PORT=3284
ENV FORGE_MODEL=anthropic/claude-sonnet-4-20250514

EXPOSE 3284 8080

ENTRYPOINT ["forge"]
CMD ["version"]
