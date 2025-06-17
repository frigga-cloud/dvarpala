.PHONY: build run test clean docker-build docker-run setup-dev migrate

# Build settings
BINARY_NAME=dvarpala-server
CLI_BINARY=dvarpala-cli
AUTH_BINARY=openvpn-auth
BUILD_DIR=bin
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.version=$(VERSION)"

# Build all binaries
build:
	@echo "Building binaries..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) cmd/dvarpala-server/main.go
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(CLI_BINARY) cmd/dvarpala-cli/main.go
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(AUTH_BINARY) cmd/openvpn-auth/main.go

# Run development server
run:
	go run cmd/dvarpala-server/main.go -config configs/environments/development.yaml

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html

# Setup development environment
setup-dev:
	@echo "Setting up development environment..."
	go mod download
	go mod tidy
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/swaggo/swag/cmd/swag@latest

# Database migrations
migrate-up:
	./$(BUILD_DIR)/$(CLI_BINARY) migrate up

migrate-down:
	./$(BUILD_DIR)/$(CLI_BINARY) migrate down

migrate-create:
	./$(BUILD_DIR)/$(CLI_BINARY) migrate create $(name)

# Docker commands
docker-build:
	docker build -t dvarpala:$(VERSION) -f deployments/docker/Dockerfile .

docker-run:
	docker-compose -f deployments/docker/docker-compose.yml up --build

# Linting and formatting
lint:
	golangci-lint run ./...

fmt:
	go fmt ./...

# Security scanning
security:
	gosec ./...

# Generate API documentation
docs:
	swag init -g cmd/dvarpala-server/main.go -o api/openapi

# Cross-platform builds
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 cmd/dvarpala-server/main.go
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe cmd/dvarpala-server/main.go
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 cmd/dvarpala-server/main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 cmd/dvarpala-server/main.go
