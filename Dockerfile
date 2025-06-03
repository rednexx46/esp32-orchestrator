# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .

# Install necessary packages
RUN go env -w GODEBUG=netdns=go && go build -o orchestrator .

# Runtime stage
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/orchestrator .

CMD ["./orchestrator"]