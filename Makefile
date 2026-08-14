# Dvarpala VPN Makefile

# Variables
GO_VERSION = 1.21
BINARY_DIR = bin
CONFIG_FILE = configs/environment.yaml

# Build targets
.PHONY: all build clean install dev test docker help

# Default target
all: build

# Build all binaries
build:
	@echo "🔨 Building Dvarpala VPN binaries..."
	@mkdir -p $(BINARY_DIR)
	go build -o $(BINARY_DIR)/dvarpala-server ./cmd/dvarpala-server/main.go
	go build -o $(BINARY_DIR)/dvarpala-cli ./cmd/dvarpala-cli/main.go
	go build -o $(BINARY_DIR)/openvpn-auth ./cmd/openvpn-auth/main.go
	go build -o $(BINARY_DIR)/dvarpala-worker ./cmd/dvarpala-worker/main.go
	go build -o $(BINARY_DIR)/install ./scripts/installation/install.go
	go build -o $(BINARY_DIR)/create-test-users ./scripts/development/create-test-users/main.go
	go build -o $(BINARY_DIR)/seed-data ./scripts/development/seed-data/main.go
	@echo "✅ Build completed successfully!"

# Clean build artifacts
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf $(BINARY_DIR)
	go clean
	@echo "✅ Clean completed!"

# Install dependencies
deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy
	@echo "✅ Dependencies installed!"

# Complete installation (creates database tables)
install: build
	@echo "🚀 Installing Dvarpala VPN System..."
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "❌ Configuration file not found: $(CONFIG_FILE)"; \
		echo "Please create the configuration file first."; \
		exit 1; \
	fi
	./$(BINARY_DIR)/install -config $(CONFIG_FILE)
	@echo "✅ Installation completed!"

# Fresh installation (drops existing tables)
install-fresh: build
	@echo "🚀 Fresh installation of Dvarpala VPN System..."
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "❌ Configuration file not found: $(CONFIG_FILE)"; \
		echo "Please create the configuration file first."; \
		exit 1; \
	fi
	./$(BINARY_DIR)/install -config $(CONFIG_FILE) -drop -force
	@echo "✅ Fresh installation completed!"

# Install with test data
install-dev: build
	@echo "🚀 Installing Dvarpala VPN System with development data..."
	@if [ ! -f $(CONFIG_FILE) ]; then \
		echo "❌ Configuration file not found: $(CONFIG_FILE)"; \
		echo "Please create the configuration file first."; \
		exit 1; \
	fi
	./$(BINARY_DIR)/install -config $(CONFIG_FILE) -seed
	@echo "✅ Development installation completed!"

# Development server
dev: build
	@echo "🔥 Starting development server..."
	./$(BINARY_DIR)/dvarpala-server -config $(CONFIG_FILE)

# Run tests
test:
	@echo "🧪 Running tests..."
	go test -v ./...
	@echo "✅ Tests completed!"

# Run tests with coverage
test-coverage:
	@echo "🧪 Running tests with coverage..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✅ Coverage report generated: coverage.html"

# Format code
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...
	@echo "✅ Code formatted!"

# Lint code
lint:
	@echo "🔍 Linting code..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint not installed. Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Create test users
create-test-users: build
	@echo "👥 Creating test users..."
	./$(BINARY_DIR)/create-test-users $(CONFIG_FILE)
	@echo "✅ Test users created!"

# Seed development data
seed-data: build
	@echo "🌱 Seeding development data..."
	./$(BINARY_DIR)/seed-data $(CONFIG_FILE)
	@echo "✅ Data seeded!"

# Docker build
docker:
	@echo "🐳 Building Docker image..."
	docker build -t dvarpala:latest -f deployments/docker/Dockerfile .
	@echo "✅ Docker image built!"

# Generate migration
migrate-create:
	@echo "📝 Create a new migration..."
	@if [ -z "$(NAME)" ]; then \
		echo "❌ Please provide a migration name: make migrate-create NAME=your_migration_name"; \
		exit 1; \
	fi
	@echo "Creating migration: $(NAME)"
	@mkdir -p database/migrations
	@touch database/migrations/$(shell date +%Y%m%d%H%M%S)_$(NAME).up.sql
	@touch database/migrations/$(shell date +%Y%m%d%H%M%S)_$(NAME).down.sql
	@echo "✅ Migration files created!"

# Database status
db-status:
	@echo "📊 Database status..."
	./$(BINARY_DIR)/dvarpala-cli db status

# Server provisioning
provision-server:
	@echo "🚀 Provisioning server with Dvarpala VPN..."
	@echo "This will download and run the server setup script."
	@echo "Make sure you're running this on the target server as root."
	curl -fsSL https://raw.githubusercontent.com/yourcompany/dvarpala/main/scripts/provisioning/setup-server.sh | bash

# Configure OAuth providers
configure-oauth:
	@echo "🔧 Configuring OAuth providers..."
	sudo ./scripts/provisioning/configure-oauth.sh

# Check server status
check-status:
	@echo "📊 Checking server status..."
	sudo ./scripts/provisioning/check-status.sh

# Show help
help:
	@echo "🏗️  Dvarpala VPN Build System"
	@echo ""
	@echo "Available targets:"
	@echo "  build              Build all binaries"
	@echo "  clean              Clean build artifacts"
	@echo "  deps               Install dependencies"
	@echo "  install            Install system (create tables)"
	@echo "  install-fresh      Fresh install (drop and recreate tables)"
	@echo "  install-dev        Install with development data"
	@echo "  dev                Start development server"
	@echo "  test               Run tests"
	@echo "  test-coverage      Run tests with coverage report"
	@echo "  fmt                Format code"
	@echo "  lint               Lint code"
	@echo "  create-test-users  Create test users"
	@echo "  seed-data          Seed development data"
	@echo "  docker             Build Docker image"
	@echo "  migrate-create     Create new migration (use NAME=migration_name)"
	@echo "  db-status          Show database status"
	@echo "  provision-server   Provision server with complete Dvarpala setup"
	@echo "  configure-oauth    Configure OAuth providers"
	@echo "  check-status       Check server status and health"
	@echo "  help               Show this help message"
	@echo ""
	@echo "Example usage:"
	@echo "  make install-dev   # Install with development data"
	@echo "  make dev           # Start development server"
	@echo "  make test          # Run tests"