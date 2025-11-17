# Build stage
FROM golang:1.25-alpine AS builder

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

# Install required packages for system monitoring and Docker client
# RUN apk add --no-cache ca-certificates docker-cli

WORKDIR /app

# Copy binary from builder stage
COPY --from=builder /app/monic .

# Create log directory
RUN mkdir -p /var/log 

# Run the application
CMD ["./monic"]
