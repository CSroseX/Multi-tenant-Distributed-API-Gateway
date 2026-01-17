# Multi-stage build for optimized image
FROM golang:1.25.5-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build only the gateway binary (Option 1: single container)
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway/main.go

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/gateway .

# Copy static files if needed
COPY static ./static

EXPOSE 8080 9001 9002

# Default command (can be overridden)
CMD ["./gateway"]
