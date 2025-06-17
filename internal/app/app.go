package app

import (
	"fmt"

	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/api/v1"
	"dvarpala/internal/redis"
	"dvarpala/internal/web"

	"github.com/gin-gonic/gin"
)

type Dvarpala struct {
	config *config.Config
	db     *database.DB
	redis  *redis.Client
	router *gin.Engine
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

	// Initialize router
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	app := &Dvarpala{
		config: cfg,
		db:     db,
		redis:  redisClient,
		router: router,
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
	v1.SetupRoutes(apiGroup, d.db, d.redis, d.config)

	// Web routes
	webGroup := d.router.Group("")
	web.SetupRoutes(webGroup, d.db, d.redis, d.config)
}
