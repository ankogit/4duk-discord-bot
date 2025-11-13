# Build stage
FROM golang:1.21-alpine AS builder

# Install build dependencies including opus and opusfile
# This layer will be cached if dependencies don't change
RUN apk add --no-cache \
    git \
    gcc \
    musl-dev \
    opus-dev \
    opusfile-dev \
    pkgconfig

WORKDIR /app

# Copy go mod files first for better caching
# This layer will be cached unless go.mod/go.sum change
COPY go.mod go.sum ./

# Download dependencies (cached unless go.mod/go.sum change)
RUN go mod download

# Copy source code (only this layer rebuilds on code changes)
COPY . .

# Build the application with optimizations
# -ldflags="-s -w" strips debug info to reduce binary size
# -mod=mod ignores vendor directory and uses go.mod directly
RUN CGO_ENABLED=1 GOOS=linux go build \
    -mod=mod \
    -ldflags="-s -w" \
    -o bot ./cmd/bot

# Runtime stage
FROM alpine:latest

# Install runtime dependencies in one layer for better caching
RUN apk add --no-cache \
    ffmpeg \
    ca-certificates \
    opus \
    opusfile && \
    addgroup -g 1000 bot && \
    adduser -D -u 1000 -G bot bot

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bot .

# Create data directory for persistent storage
RUN mkdir -p /app/data && \
    chown -R bot:bot /app

USER bot

CMD ["./bot"]
