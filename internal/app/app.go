package app

import (
	"github.com/yourcompany/dvarpala/internal/config"
	"github.com/yourcompany/dvarpala/internal/database"
	"github.com/yourcompany/dvarpala/internal/api/v1"
	"github.com/yourcompany/dvarpala/internal/redis"
	"github.com/yourcompany/dvarpala/internal/web"

	"github.com/gin-gonic/gin"
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
	v1.SetupRoutes(apiGroup, a.db, a.redis, a.config)

	// Web routes
	webGroup := a.router.Group("")
	web.SetupRoutes(webGroup, a.db, a.redis, a.config)
}
