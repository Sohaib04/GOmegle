# Multi-stage build for GOmegle
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install git and ca-certificates
RUN apk add --no-cache git ca-certificates

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gomegle ./cmd/server/main.go

# Minimal runtime image
FROM alpine:3.19

WORKDIR /app

RUN apk --no-cache add ca-certificates tzdata

# Copy binary and static web assets
COPY --from=builder /app/gomegle .
COPY --from=builder /app/web ./web

# Expose default HTTP/WS port
EXPOSE 8080

ENV PORT=8080

CMD ["./gomegle"]

