# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with CGO for SQLite
RUN CGO_ENABLED=1 go build -trimpath -ldflags "-s -w" -o /mimir-mcp ./cmd/mimir-mcp

# Runtime stage
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs tzdata

# Create non-root user
RUN adduser -D -u 1000 mimir
USER mimir

WORKDIR /app

# Copy binary
COPY --from=builder /mimir-mcp /app/mimir-mcp

# Create data directory
RUN mkdir -p /data

# Environment
ENV MIMIR_DATA_DIR=/data
ENV MIMIR_LOG_FORMAT=json
ENV MIMIR_LOG_LEVEL=info

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run
ENTRYPOINT ["/app/mimir-mcp"]
