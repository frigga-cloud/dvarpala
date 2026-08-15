package app

import (
	"fmt"
	"log"
	"time"

	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/api/vpnapi"
	"dvarpala/internal/api/v1"
	"dvarpala/internal/auth"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"
	"dvarpala/internal/web"

	"github.com/gin-gonic/gin"
)

type Dvarpala struct {
	config   *config.Config
	db       *database.DB
	redis    *redis.Client
	services *services.Services
	auth     *auth.Service
	router   *gin.Engine
}

func NewDvarpala(cfg *config.Config) (*Dvarpala, error) {
	// Initialize database
	db, err := database.NewConnection(cfg.Database)
	if err != nil {
		return nil, err
	}

	// Initialize database tables and essential data
	if err := database.InitializeDatabase(db.DB); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize Redis
	redisClient, err := redis.NewClient(cfg.Redis)
	if err != nil {
		return nil, err
	}

	// Authentication: providers, session store, and the login flow.
	svc := services.New(db.DB)
	sessions := auth.NewSessionService(redisClient,
		time.Duration(cfg.Auth.SessionDuration)*time.Second)

	providers := auth.NewRegistry()

	// Google, when credentials are configured. Missing credentials are not
	// fatal: the provider is simply not offered.
	if cfg.OAuth.Google.ClientID != "" {
		google, err := auth.NewGoogleProvider(
			cfg.OAuth.Google.ClientID,
			cfg.OAuth.Google.ClientSecret,
			cfg.OAuth.Google.RedirectURL,
			"", // hosted domain: leave to the allow-list
		)
		if err != nil {
			return nil, fmt.Errorf("configuring google provider: %w", err)
		}
		providers.Add(google)
		log.Println("Google login enabled")
	}

	// Development stand-in, refused outside debug mode.
	if dev := auth.NewDevProvider(fmt.Sprintf("http://localhost:%d", cfg.Server.Port)); dev.Guard(cfg.Server.Mode) == nil {
		providers.Add(dev)
		log.Println("WARNING: development login provider is enabled (server.mode=debug)")
	}

	if providers.Len() == 0 {
		log.Println("WARNING: no login providers are enabled - nobody can authenticate")
	}

	authSvc := auth.NewService(providers, sessions, svc, redisClient, cfg.Auth.AllowedDomains)

	// Initialize router
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.LoadHTMLGlob("web/templates/*.html")

	app := &Dvarpala{
		config:   cfg,
		db:       db,
		redis:    redisClient,
		services: svc,
		auth:     authSvc,
		router:   router,
	}

	// Setup routes
	app.setupRoutes()

	return app, nil
}

func (d *Dvarpala) Router() *gin.Engine {
	return d.router
}

func (d *Dvarpala) setupRoutes() {
	// API routes
	apiGroup := d.router.Group("/api/v1")
	v1.SetupRoutes(apiGroup, d.services, d.redis, d.config)

	// Endpoints the OpenVPN hooks call. Localhost only in a real deployment.
	vpnapi.NewHandler(d.auth, d.services).Register(d.router.Group(""))

	// Web routes
	webGroup := d.router.Group("")
	web.SetupRoutes(webGroup, d.auth, d.config)
}
