# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .

# Install necessary packages
ENV GODEBUG=netdns=go

# Build the Go application
RUN go build -o orchestrator .

# Runtime stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/orchestrator .

CMD ["./orchestrator"]