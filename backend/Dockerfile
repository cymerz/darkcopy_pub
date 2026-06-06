# Multi-stage Dockerfile for Go backend
FROM golang:1.26-alpine AS builder

WORKDIR /app

# Install compilation dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy Go dependency files
COPY go.mod go.sum ./
RUN go mod download

# Copy backend source code
COPY . .

# Build the application statically and strip debug info
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./cmd/server

# Final production stage
FROM alpine:latest

WORKDIR /app

# Install runtime dependencies (ca-certificates needed for HTTPS/S3 communication)
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /app/server .

# Copy web templates and static assets
COPY --from=builder /app/web ./web

# Create a local upload directory fallback
RUN mkdir -p uploads

# Expose backend port
EXPOSE 8080

# Start server
CMD ["./server"]
