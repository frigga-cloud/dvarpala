# Dvarpala VPN Project - Go Development Context

## Project Overview

**Project Name**: Dvarpala VPN  
**Etymology**: Dvarpala (द्वारपाल) - Sanskrit for "gatekeeper" or "door guardian"  
**Purpose**: Enterprise-grade VPN solution with Zero Trust Network Access (ZTNA) and granular access control  
**Language**: Go 1.21+  
**Architecture**: Microservices-ready, single binary distribution

## Core Technology Stack

### Backend Framework
- **HTTP Router**: Gin (high-performance HTTP web framework)
- **Database ORM**: GORM (Go ORM with PostgreSQL driver)
- **Configuration**: Viper (configuration management)
- **CLI Framework**: Cobra (command-line interface)
- **Authentication**: golang-jwt/jwt (JSON Web Tokens)
- **OAuth**: golang.org/x/oauth2 (OAuth 2.0 client)

### Infrastructure
- **Database**: PostgreSQL with GORM migrations
- **Cache/Session**: Redis with go-redis/redis client
- **VPN**: OpenVPN with custom Go authentication scripts
- **Web Templates**: html/template (Go standard library)
- **Deployment**: Docker, Kubernetes
- **Monitoring**: Prometheus metrics, structured logging

### Key Go Packages
```go
// HTTP and Web
"github.com/gin-gonic/gin"
"html/template"
"net/http"

// Database and ORM
"gorm.io/gorm"
"gorm.io/driver/postgres"
"github.com/golang-migrate/migrate/v4"

// Configuration and CLI
"github.com/spf13/viper"
"github.com/spf13/cobra"

// Authentication and Security
"github.com/golang-jwt/jwt/v5"
"golang.org/x/crypto/bcrypt"
"golang.org/x/oauth2"
"golang.org/x/oauth2/google"
"golang.org/x/oauth2/microsoft"
"golang.org/x/oauth2/github"

// Redis and Caching
"github.com/go-redis/redis/v8"

// Utilities
"github.com/google/uuid"
"github.com/gorilla/sessions"
"github.com/gorilla/csrf"
```

## Project Structure (Go Conventions)

```
dvarpala/
├── cmd/                              # Main application entry points
│   ├── dvarpala-server/              # Web server binary
│   ├── dvarpala-cli/                 # CLI tool binary
│   ├── dvarpala-worker/              # Background worker binary
│   └── openvpn-auth/                 # OpenVPN auth script binary
├── internal/                         # Private application code
│   ├── app/                          # Application setup and configuration
│   ├── config/                       # Configuration management
│   ├── database/                     # Database connection and models
│   ├── redis/                        # Redis client and utilities
│   ├── auth/                         # Authentication system
│   ├── vpn/                          # VPN management
│   ├── access/                       # Access control system
│   ├── api/                          # REST API layer
│   ├── web/                          # Web interface
│   ├── services/                     # Business logic services
│   └── utils/                        # Utility functions
├── pkg/                              # Public packages (reusable)
│   ├── dvarpala-client/              # Go client library
│   ├── api-client/                   # API client utilities
│   └── auth-utils/                   # Authentication utilities
├── api/                              # API definitions
│   ├── openapi/                      # OpenAPI 3.0 specifications
│   └── proto/                        # gRPC definitions (future)
├── web/                              # Web assets
│   ├── templates/                    # HTML templates
│   └── static/                       # CSS, JS, images
├── configs/                          # Configuration files
├── deployments/                      # Docker, K8s, Terraform
├── scripts/                          # Build and deployment scripts
├── test/                             # Test files
└── docs/                             # Documentation
```

## Authentication Flow (Go Implementation)

### Two-Stage VPN Authentication
1. **Initial Connection**: Static credentials → Captive portal access
2. **Web Authentication**: OAuth validation → Database check → Full access

### Go Authentication Components
```go
// Main authentication interface
type Authenticator interface {
    ValidateOAuth(provider, token string) (*UserInfo, error)
    CheckDatabase(email string) (*User, error)
    CreateSession(userID uint, clientIP string) error
    ValidateSession(clientIP string) (*Session, error)
}

// OAuth provider interface
type OAuthProvider interface {
    GetAuthURL(state string) string
    ExchangeCodeForToken(code string) (*oauth2.Token, error)
    GetUserInfo(token *oauth2.Token) (*UserInfo, error)
}

// VPN authentication interface
type VPNAuthenticator interface {
    AuthenticateClient(username, password, clientIP string) bool
    GrantCaptiveAccess(clientIP string) error
    GrantFullAccess(clientIP string, userGroups []Group) error
    DisconnectClient(clientIP string) error
}
```

## Database Schema (GORM Models)

### Core Models
```go
// User model with GORM tags
type User struct {
    ID           uint      `gorm:"primaryKey"`
    Email        string    `gorm:"uniqueIndex;not null"`
    FullName     string    `gorm:"size:255"`
    Department   string    `gorm:"size:100"`
    Status       UserStatus `gorm:"default:active"`
    OAuthProvider string   `gorm:"size:50"`
    LastLogin    *time.Time
    CreatedAt    time.Time
    UpdatedAt    time.Time
    
    // Associations
    Groups       []Group   `gorm:"many2many:user_groups;"`
    VPNSessions  []VPNSession
    AuditLogs    []AuditLog
}

type Group struct {
    ID          uint    `gorm:"primaryKey"`
    Name        string  `gorm:"uniqueIndex;not null;size:100"`
    Description string  `gorm:"type:text"`
    ParentID    *uint   `gorm:"index"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    
    // Self-referencing for hierarchy
    Parent      *Group  `gorm:"foreignKey:ParentID"`
    Children    []Group `gorm:"foreignKey:ParentID"`
    
    // Associations
    Users       []User     `gorm:"many2many:user_groups;"`
    Permissions []Permission `gorm:"many2many:group_permissions;"`
}

type Resource struct {
    ID          uint         `gorm:"primaryKey"`
    Name        string       `gorm:"not null;size:255"`
    Type        ResourceType `gorm:"not null"`
    URL         string       `gorm:"size:500"`
    IPAddress   string       `gorm:"size:45"` // IPv4/IPv6
    Port        int
    Description string       `gorm:"type:text"`
    CreatedAt   time.Time
    UpdatedAt   time.Time
    
    // Associations
    Permissions []Permission `gorm:"many2many:group_permissions;"`
}
```

### Enums and Types
```go
type UserStatus string
const (
    UserStatusActive    UserStatus = "active"
    UserStatusInactive  UserStatus = "inactive"
    UserStatusSuspended UserStatus = "suspended"
)

type ResourceType string
const (
    ResourceTypeDashboard ResourceType = "dashboard"
    ResourceTypeVM        ResourceType = "vm"
    ResourceTypeDatabase  ResourceType = "database"
    ResourceTypeService   ResourceType = "service"
)

type PermissionType string
const (
    PermissionRead  PermissionType = "read"
    PermissionWrite PermissionType = "write"
    PermissionAdmin PermissionType = "admin"
    PermissionSSH   PermissionType = "ssh"
    PermissionFull  PermissionType = "full"
)
```

## Configuration Management (Viper)

### Configuration Structure
```go
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
    Mode         string        `mapstructure:"mode"` // debug, release
    ReadTimeout  time.Duration `mapstructure:"read_timeout"`
    WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type AuthConfig struct {
    SessionDuration      int      `mapstructure:"session_duration"`
    CaptivePortalTimeout int      `mapstructure:"captive_portal_timeout"`
    JWTSecret           string   `mapstructure:"jwt_secret"`
    AllowedDomains      []string `mapstructure:"allowed_domains"`
}
```

### Environment Variables (.env)
```bash
# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=dvarpala
DATABASE_USER=dvarpala
DATABASE_PASSWORD=password
DATABASE_SSL_MODE=disable

# Redis
REDIS_ADDR=localhost:6379
REDIS_PASSWORD=
REDIS_DB=0

# OAuth Providers
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
MICROSOFT_CLIENT_ID=your-microsoft-client-id
MICROSOFT_CLIENT_SECRET=your-microsoft-client-secret
GITHUB_CLIENT_ID=your-github-client-id
GITHUB_CLIENT_SECRET=your-github-client-secret

# Security
JWT_SECRET=your-jwt-secret-key
ALLOWED_DOMAINS=yourcompany.com,subsidiary.com

# OpenVPN
OPENVPN_MGMT_HOST=localhost
OPENVPN_MGMT_PORT=7505

# Application
SERVER_PORT=8080
GIN_MODE=debug
SESSION_DURATION=28800
```

## API Design Patterns (Gin)

### Router Setup
```go
func SetupRouter(cfg *config.Config, db *gorm.DB, redis *redis.Client) *gin.Engine {
    r := gin.New()
    r.Use(gin.Logger(), gin.Recovery())
    
    // API v1 routes
    v1 := r.Group("/api/v1")
    {
        auth := v1.Group("/auth")
        auth.POST("/validate", handlers.ValidateAuth)
        auth.POST("/ssh/authorize", handlers.AuthorizeSSH)
        
        users := v1.Group("/users", middleware.Auth())
        users.GET("", handlers.ListUsers)
        users.POST("", handlers.CreateUser)
        users.POST("/bulk", handlers.BulkCreateUsers)
    }
    
    // Web routes
    web := r.Group("")
    web.LoadHTMLGlob("web/templates/**/*")
    web.Static("/static", "web/static")
    
    return r
}
```

### Request/Response Patterns
```go
// Standard API response structure
type APIResponse struct {
    Success bool        `json:"success"`
    Data    interface{} `json:"data,omitempty"`
    Error   string      `json:"error,omitempty"`
    Meta    *MetaData   `json:"meta,omitempty"`
}

type MetaData struct {
    Page       int `json:"page"`
    PerPage    int `json:"per_page"`
    Total      int `json:"total"`
    TotalPages int `json:"total_pages"`
}

// Request validation with struct tags
type CreateUserRequest struct {
    Email      string `json:"email" binding:"required,email"`
    FullName   string `json:"full_name" binding:"required,min=2,max=255"`
    Department string `json:"department" binding:"required,max=100"`
}
```

## Service Layer Pattern

### User Service Example
```go
type UserService struct {
    db    *gorm.DB
    redis *redis.Client
    audit *AuditService
}

func NewUserService(db *gorm.DB, redis *redis.Client, audit *AuditService) *UserService {
    return &UserService{db: db, redis: redis, audit: audit}
}

func (s *UserService) CreateUser(req CreateUserRequest) (*User, error) {
    // Validate email domain
    if !s.isAllowedDomain(req.Email) {
        return nil, ErrInvalidDomain
    }
    
    // Check if user exists
    var existingUser User
    if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
        return nil, ErrUserExists
    }
    
    // Create user
    user := User{
        Email:      req.Email,
        FullName:   req.FullName,
        Department: req.Department,
        Status:     UserStatusActive,
    }
    
    if err := s.db.Create(&user).Error; err != nil {
        return nil, err
    }
    
    // Audit log
    s.audit.LogAction(AuditLog{
        Action:       "user_created",
        ResourceType: "user",
        ResourceID:   user.ID,
        Details:      map[string]interface{}{"email": user.Email},
    })
    
    return &user, nil
}
```

## VPN Integration (OpenVPN + Go)

### OpenVPN Authentication Script
```go
// cmd/openvpn-auth/main.go
func main() {
    username := os.Getenv("username")
    password := os.Getenv("password")
    clientIP := os.Getenv("untrusted_ip")
    
    auth := openvpn.NewAuthenticator(getConfig())
    
    if auth.Authenticate(username, password, clientIP) {
        os.Exit(0) // Success
    } else {
        os.Exit(1) // Failure
    }
}

// VPN authenticator implementation
func (a *Authenticator) Authenticate(username, password, clientIP string) bool {
    // Always allow temp credentials for captive portal
    if username == "temp_user" && password == "temp_portal_access" {
        status := a.checkUserAuthorization(clientIP)
        switch status {
        case "authorized":
            return a.grantFullAccess(clientIP)
        case "pending_oauth":
            return a.grantCaptiveAccess(clientIP)
        default:
            return false
        }
    }
    return false
}
```

## Error Handling Patterns

### Custom Errors
```go
var (
    ErrUserNotFound     = errors.New("user not found")
    ErrInvalidDomain    = errors.New("email domain not allowed")
    ErrUserExists       = errors.New("user already exists")
    ErrInvalidSession   = errors.New("invalid session")
    ErrPermissionDenied = errors.New("permission denied")
)

// Error response helper
func HandleError(c *gin.Context, err error, status int) {
    c.JSON(status, APIResponse{
        Success: false,
        Error:   err.Error(),
    })
}
```

## Testing Patterns

### Unit Test Example
```go
func TestUserService_CreateUser(t *testing.T) {
    db := setupTestDB(t)
    redis := setupTestRedis(t)
    audit := &MockAuditService{}
    
    service := NewUserService(db, redis, audit)
    
    req := CreateUserRequest{
        Email:      "test@yourcompany.com",
        FullName:   "Test User",
        Department: "Engineering",
    }
    
    user, err := service.CreateUser(req)
    
    assert.NoError(t, err)
    assert.Equal(t, req.Email, user.Email)
    assert.Equal(t, UserStatusActive, user.Status)
}
```

## CLI Commands (Cobra)

### CLI Structure
```go
// User management commands
var userCmd = &cobra.Command{
    Use:   "user",
    Short: "User management commands",
}

var createUserCmd = &cobra.Command{
    Use:   "create",
    Short: "Create a new user",
    RunE: func(cmd *cobra.Command, args []string) error {
        email, _ := cmd.Flags().GetString("email")
        name, _ := cmd.Flags().GetString("name")
        dept, _ := cmd.Flags().GetString("department")
        
        // Initialize services
        cfg := loadConfig()
        db := connectDB(cfg)
        service := services.NewUserService(db, nil, nil)
        
        user, err := service.CreateUser(CreateUserRequest{
            Email:      email,
            FullName:   name,
            Department: dept,
        })
        
        if err != nil {
            return err
        }
        
        fmt.Printf("User created: %s (ID: %d)\n", user.Email, user.ID)
        return nil
    },
}
```

## Build and Deployment

### Makefile Targets
```make
build:
    go build -o bin/dvarpala-server cmd/dvarpala-server/main.go
    go build -o bin/dvarpala-cli cmd/dvarpala-cli/main.go
    go build -o bin/openvpn-auth cmd/openvpn-auth/main.go

run:
    go run cmd/dvarpala-server/main.go -config configs/environments/development.yaml

test:
    go test -v ./...

docker-build:
    docker build -t dvarpala:latest -f deployments/docker/Dockerfile .
```

### Docker Multi-stage Build
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o dvarpala-server cmd/dvarpala-server/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/dvarpala-server .
COPY --from=builder /app/configs ./configs
COPY --from=builder /app/web ./web
EXPOSE 8080
CMD ["./dvarpala-server"]
```

## Development Workflow

### Getting Started
1. **Initialize project**: `go mod init github.com/yourcompany/dvarpala`
2. **Install dependencies**: `go mod tidy`
3. **Set up environment**: `cp .env.example .env`
4. **Run database**: `docker-compose up postgres redis`
5. **Run migrations**: `make migrate-up`
6. **Start server**: `make run`

### Common Commands
```bash
# Development
make run                    # Start development server
make test                   # Run all tests
make lint                   # Run linter
go run cmd/dvarpala-cli/main.go user create --email test@company.com

# Database
make migrate-up             # Run migrations
make migrate-down           # Rollback migrations
make seed                   # Seed test data

# Build and Deploy
make build                  # Build all binaries
make docker-build           # Build Docker image
make deploy                 # Deploy to staging/production
```

## Key Go Idioms and Patterns

### Interface Usage
- Define interfaces at the point of use (consumer side)
- Keep interfaces small and focused
- Use composition over inheritance

### Error Handling
- Always handle errors explicitly
- Use custom error types for domain-specific errors
- Wrap errors with context using `fmt.Errorf("operation failed: %w", err)`

### Concurrency
- Use goroutines for async operations
- Implement graceful shutdown with context cancellation
- Use channels for communication between goroutines

### Code Organization
- Keep business logic in service layer
- Use dependency injection for testability
- Follow Go project layout standards

This context provides everything needed to develop the Dvarpala VPN system using Go best practices and idioms. The codebase emphasizes clean architecture, testability, and maintainability while leveraging Go's strengths in performance and deployment simplicity.