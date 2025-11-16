# Build stage
FROM golang:1.23-alpine AS builder

# Install required packages for gopsutil
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags="-X main.version=${VERSION}" -o monic main.go

# Runtime stage
FROM alpine:latest

# Install required packages for system monitoring
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -S monic && adduser -S monic -G monic

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/monic .
COPY --from=builder /app/config.json .

# Create log directory
RUN mkdir -p /var/log && chown monic:monic /var/log

# Switch to non-root user
USER monic

# Expose metrics port (if you add metrics endpoint in the future)
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
CMD ["./monic"]
