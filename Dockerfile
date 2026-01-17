# Multi-stage build for optimized image
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./cmd/gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-user ./cmd/mock-user
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-order ./cmd/mock-order
RUN CGO_ENABLED=0 GOOS=linux go build -o mock-backend ./cmd/mock-backend

# Final stage
FROM alpine:latest

WORKDIR /root/

# Copy binaries from builder
COPY --from=builder /app/gateway .
COPY --from=builder /app/mock-user .
COPY --from=builder /app/mock-order .
COPY --from=builder /app/mock-backend .

# Copy static files if needed
COPY static ./static

EXPOSE 8080 9000 9001 9002

# Default command (can be overridden)
CMD ["./gateway"]
