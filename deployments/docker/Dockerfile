# Multi-stage build for Go application
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dvarpala-server cmd/dvarpala-server/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o dvarpala-cli cmd/dvarpala-cli/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o openvpn-auth cmd/openvpn-auth/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy binaries
COPY --from=builder /app/dvarpala-server .
COPY --from=builder /app/dvarpala-cli .
COPY --from=builder /app/openvpn-auth .

# Copy configuration files
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/web ./web

# Create logs directory
RUN mkdir -p /var/log/dvarpala

EXPOSE 8080

CMD ["./dvarpala-server"]
