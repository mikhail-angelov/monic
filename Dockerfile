# Build stage
FROM golang:1.25-alpine AS builder

# Install required packages for gopsutil
RUN apk add --no-cache gcc musl-dev

WORKDIR /app

# Copy go mod files and vendor directory
COPY go.mod go.sum ./
COPY vendor ./vendor/

# Copy source code
COPY . .

# Build the application using vendored dependencies
ARG VERSION=dev
RUN CGO_ENABLED=1 GOOS=linux go build -mod=vendor -a -installsuffix cgo -ldflags="-X main.version=${VERSION}" -o monic main.go

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/monic .

# Create log directory
RUN mkdir -p /var/log 

# Run the application
CMD ["./monic"]
