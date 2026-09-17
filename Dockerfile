# Stage 1: Builder
FROM golang:alpine AS builder

# Install build dependencies
RUN apk --no-cache add ca-certificates git

WORKDIR /app

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Install templ CLI matching project dependency
RUN go install github.com/a-h/templ/cmd/templ@v0.3.1020

# Copy application source
COPY . .

# Re-generate templates
RUN templ generate

# Build statically linked binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/bin/server ./cmd/server

# Stage 2: Minimal runtime image
FROM alpine:latest

# Install runtime dependencies for TLS and timezone operations
RUN apk --no-cache add ca-certificates tzdata

# Run as non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

# Copy binary and static assets from builder
COPY --from=builder /app/bin/server /app/server
COPY --from=builder /app/static /app/static

USER appuser:appgroup

ENV PORT=8081
EXPOSE 8081

ENTRYPOINT ["/app/server"]
