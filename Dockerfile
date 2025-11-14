FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go files
COPY main.go .

# Initialize go module
RUN go mod init trader_bot && \
    go mod tidy

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o trading-bot main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/trading-bot .

# Expose port
EXPOSE 8080

# Run
CMD ["./trading-bot"]
