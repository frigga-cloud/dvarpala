#!/bin/bash
# Dvarpala VPN - Go Project Structure Setup
# Run this from your dvarpala project directory

echo "Creating Dvarpala Go project structure..."

# Initialize Go module
go mod init github.com/yourcompany/dvarpala

# Create main project directories following Go conventions
mkdir -p {cmd,internal,pkg,api,web,scripts,deployments,configs,test,docs,tools}

# Main application entry points
mkdir -p cmd/{dvarpala-server,dvarpala-cli,dvarpala-worker,openvpn-auth}

# Internal application code (private to this project)
mkdir -p internal/{app,config,database,redis,auth,vpn,access,api,web,services,utils}

# Auth module
mkdir -p internal/auth/{oauth,middleware,handlers}
mkdir -p internal/auth/oauth/{google,microsoft,github}

# VPN management
mkdir -p internal/vpn/{openvpn,client,network,session,certificate}

# Access control
mkdir -p internal/access/{permissions,groups,resources,policies}

# API layers
mkdir -p internal/api/{v1,middleware,validators}
mkdir -p internal/api/v1/{auth,users,groups,resources,permissions,vpn,external}

# Web interface
mkdir -p internal/web/{auth,admin,user,middleware,handlers,forms}

# Services (business logic)
mkdir -p internal/services/{user,group,resource,permission,vpn,audit,notification}

# Database and models
mkdir -p internal/database/{migrations,models,repositories}

# Utilities
mkdir -p internal/utils/{crypto,validation,email,network}

# Public packages (can be imported by other projects)
mkdir -p pkg/{dvarpala-client,api-client,auth-utils}

# API definitions (OpenAPI/gRPC)
mkdir -p api/{openapi,proto,graphql}

# Web assets and templates
mkdir -p web/{templates,static}
mkdir -p web/templates/{auth,admin,user,components,layouts}
mkdir -p web/static/{css,js,images,fonts}

# Configuration files
mkdir -p configs/{environments,openvpn,nginx,monitoring}
mkdir -p configs/openvpn/{auth-scripts,templates}

# Scripts and tools
mkdir -p scripts/{setup,migration,deployment,monitoring,development}

# Deployment configurations
mkdir -p deployments/{docker,kubernetes,terraform,ansible}
mkdir -p deployments/kubernetes/{base,overlays}

# Testing
mkdir -p test/{unit,integration,e2e,fixtures,mocks}

# Documentation
mkdir -p docs/{api,deployment,architecture,user-guides,development}

# Development tools
mkdir -p tools/{linting,security,development,generators}

echo "Creating Go source files..."

# Main application entry points
cat > cmd/dvarpala-server/main.go << 'EOF'
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourcompany/dvarpala/internal/app"
	"github.com/yourcompany/dvarpala/internal/config"
)

func main() {
	var configPath = flag.String("config", "configs/environments/development.yaml", "Config file path")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize application
	app, err := app.NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	// Start server
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: app.Router(),
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Dvarpala server starting on port %d", cfg.Server.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed to start: %v", err)
	}
}
EOF

cat > cmd/dvarpala-cli/main.go << 'EOF'
package main

import (
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/yourcompany/dvarpala/internal/config"
	"github.com/yourcompany/dvarpala/internal/services"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "dvarpala-cli",
		Short: "Dvarpala VPN management CLI",
		Long:  "Command line interface for managing Dvarpala VPN users, groups, and resources",
	}

	// Add subcommands
	rootCmd.AddCommand(userCmd())
	rootCmd.AddCommand(groupCmd())
	rootCmd.AddCommand(vpnCmd())
	rootCmd.AddCommand(adminCmd())

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func userCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}
}

func groupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "group",
		Short: "Group management commands",
	}
}

func vpnCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "vpn",
		Short: "VPN management commands",
	}
}

func adminCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "admin",
		Short: "Administrative commands",
	}
}
EOF

cat > cmd/openvpn-auth/main.go << 'EOF'
package main

import (
	"log"
	"os"

	"github.com/yourcompany/dvarpala/internal/vpn/openvpn"
)

func main() {
	// OpenVPN authentication script
	// Called by OpenVPN server for user authentication
	
	username := os.Getenv("username")
	password := os.Getenv("password")
	clientIP := os.Getenv("untrusted_ip")

	auth := openvpn.NewAuthenticator()
	
	if auth.Authenticate(username, password, clientIP) {
		os.Exit(0) // Success
	} else {
		os.Exit(1) // Failure
	}
}
EOF

# Go module files
cat > go.mod << 'EOF'
module github.com/yourcompany/dvarpala

go 1.21

require (
	github.com/gin-gonic/gin v1.9.1
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.16.0
	github.com/golang-migrate/migrate/v4 v4.16.2
	github.com/lib/pq v1.10.9
	github.com/go-redis/redis/v8 v8.11.5
	github.com/golang-jwt/jwt/v5 v5.0.0
	github.com/google/uuid v1.3.0
	golang.org/x/crypto v0.12.0
	golang.org/x/oauth2 v0.10.0
	github.com/stretchr/testify v1.8.4
	github.com/gorilla/sessions v1.2.1
	github.com/gorilla/csrf v1.7.1
	gorm.io/gorm v1.25.4
	gorm.io/driver/postgres v1.5.2
)
EOF

# Configuration files
cat > configs/environments/development.yaml << 'EOF'
server:
  port: 8080
  mode: debug
  read_timeout: 30s
  write_timeout: 30s

database:
  host: localhost
  port: 5432
  name: dvarpala
  user: dvarpala
  password: password
  ssl_mode: disable
  max_open_conns: 25
  max_idle_conns: 5

redis:
  addr: localhost:6379
  password: ""
  db: 0
  pool_size: 10

auth:
  session_duration: 28800 # 8 hours
  captive_portal_timeout: 300 # 5 minutes
  jwt_secret: your-jwt-secret-key
  allowed_domains:
    - yourcompany.com
    - subsidiary.com

oauth:
  google:
    client_id: ${GOOGLE_CLIENT_ID}
    client_secret: ${GOOGLE_CLIENT_SECRET}
    redirect_url: http://localhost:8080/auth/oauth/google/callback
  microsoft:
    client_id: ${MICROSOFT_CLIENT_ID}
    client_secret: ${MICROSOFT_CLIENT_SECRET}
    redirect_url: http://localhost:8080/auth/oauth/microsoft/callback
  github:
    client_id: ${GITHUB_CLIENT_ID}
    client_secret: ${GITHUB_CLIENT_SECRET}
    redirect_url: http://localhost:8080/auth/oauth/github/callback

openvpn:
  management:
    host: localhost
    port: 7505
  networks:
    captive_portal: 10.8.0.0/24
    full_access: 10.8.1.0/24

security:
  failed_login_threshold: 3
  ip_block_duration: 1200 # 20 minutes
  bcrypt_cost: 12

logging:
  level: debug
  format: json
  output: stdout
EOF

# Basic internal package structure
cat > internal/app/app.go << 'EOF'
package app

import (
	"github.com/gin-gonic/gin"
	"internal/config"
	"internal/database"
	"internal/redis"
	"internal/api"
	"internal/web"
)

type App struct {
	config *config.Config
	db     *database.DB
	redis  *redis.Client
	router *gin.Engine
}

func NewApp(cfg *config.Config) (*App, error) {
	// Initialize database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		return nil, err
	}

	// Initialize Redis
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return nil, err
	}

	// Initialize router
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	app := &App{
		config: cfg,
		db:     db,
		redis:  redisClient,
		router: router,
	}

	// Setup routes
	app.setupRoutes()

	return app, nil
}

func (a *App) Router() *gin.Engine {
	return a.router
}

func (a *App) setupRoutes() {
	// API routes
	apiGroup := a.router.Group("/api/v1")
	api.SetupRoutes(apiGroup, a.db, a.redis, a.config)

	// Web routes
	webGroup := a.router.Group("")
	web.SetupRoutes(webGroup, a.db, a.redis, a.config)
}
EOF

cat > internal/config/config.go << 'EOF'
package config

import (
	"time"
	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OAuth    OAuthConfig    `mapstructure:"oauth"`
	OpenVPN  OpenVPNConfig  `mapstructure:"openvpn"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type DatabaseConfig struct {
	Host         string `mapstructure:"host"`
	Port         int    `mapstructure:"port"`
	Name         string `mapstructure:"name"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	SSLMode      string `mapstructure:"ssl_mode"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
}

type AuthConfig struct {
	SessionDuration        int      `mapstructure:"session_duration"`
	CaptivePortalTimeout   int      `mapstructure:"captive_portal_timeout"`
	JWTSecret             string   `mapstructure:"jwt_secret"`
	AllowedDomains        []string `mapstructure:"allowed_domains"`
}

type OAuthConfig struct {
	Google    OAuthProvider `mapstructure:"google"`
	Microsoft OAuthProvider `mapstructure:"microsoft"`
	GitHub    OAuthProvider `mapstructure:"github"`
}

type OAuthProvider struct {
	ClientID     string `mapstructure:"client_id"`
	ClientSecret string `mapstructure:"client_secret"`
	RedirectURL  string `mapstructure:"redirect_url"`
}

type OpenVPNConfig struct {
	Management OpenVPNManagement `mapstructure:"management"`
	Networks   OpenVPNNetworks   `mapstructure:"networks"`
}

type OpenVPNManagement struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type OpenVPNNetworks struct {
	CaptivePortal string `mapstructure:"captive_portal"`
	FullAccess    string `mapstructure:"full_access"`
}

type SecurityConfig struct {
	FailedLoginThreshold int `mapstructure:"failed_login_threshold"`
	IPBlockDuration      int `mapstructure:"ip_block_duration"`
	BcryptCost          int `mapstructure:"bcrypt_cost"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

func Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
EOF

# Makefile
cat > Makefile << 'EOF'
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
EOF

# Docker files
cat > deployments/docker/Dockerfile << 'EOF'
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
EOF

# Basic gitignore
cat > .gitignore << 'EOF'
# Go
bin/
vendor/
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
coverage.html

# IDE
.vscode/
.idea/
*.swp
*.swo

# Environment files
.env
.env.local
.env.production

# Logs
*.log
logs/

# Database
*.db
*.sqlite

# OS
.DS_Store
Thumbs.db

# Certificates and keys
*.crt
*.key
*.pem
*.p12

# Temporary files
tmp/
temp/
EOF

# README
cat > README.md << 'EOF'
# Dvarpala VPN

Dvarpala (द्वारपाल) - Sanskrit for "gatekeeper" - is an enterprise-grade VPN solution with Zero Trust Network Access (ZTNA) and granular access control.

## Features

- OpenVPN-based secure connection with captive portal
- Multi-provider OAuth authentication (Google, Microsoft, GitHub)
- Group-based access control with hierarchical permissions
- Dynamic route assignment based on user groups
- RESTful API for external tool integration
- Comprehensive admin dashboard
- Audit logging and session management

## Quick Start

1. **Setup Development Environment**
   ```bash
   make setup-dev
   ```

2. **Build the Application**
   ```bash
   make build
   ```

3. **Run Development Server**
   ```bash
   make run
   ```

4. **Run Tests**
   ```bash
   make test
   ```

## Project Structure

- `cmd/` - Main application entry points
- `internal/` - Private application code
- `pkg/` - Public packages
- `api/` - API definitions (OpenAPI, gRPC)
- `web/` - Web templates and static assets
- `configs/` - Configuration files
- `deployments/` - Docker, Kubernetes, Terraform configs
- `scripts/` - Build and deployment scripts
- `test/` - Test files and fixtures

## Documentation

See the [docs/](docs/) directory for detailed documentation.

## License

MIT License
EOF

echo "✅ Dvarpala Go project structure created successfully!"
echo ""
echo "Next steps:"
echo "1. Run: make setup-dev"
echo "2. Run: go mod tidy"
echo "3. Configure your environment variables"
echo "4. Run: make build"
echo "5. Run: make run"