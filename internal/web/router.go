package web

import (
	"github.com/gin-gonic/gin"
	"github.com/yourcompany/dvarpala/internal/config"
	"github.com/yourcompany/dvarpala/internal/database"
	"github.com/yourcompany/dvarpala/internal/redis"
)

func SetupRoutes(r *gin.RouterGroup, db *database.DB, redis *redis.Client, cfg *config.Config) {
	// Static files
	r.Static("/static", "./web/static")
	
	// Web routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "Dvarpala VPN",
		})
	})

	r.GET("/login", func(c *gin.Context) {
		c.HTML(200, "login.html", gin.H{
			"title": "Login - Dvarpala VPN",
		})
	})

	r.GET("/dashboard", func(c *gin.Context) {
		c.HTML(200, "dashboard.html", gin.H{
			"title": "Dashboard - Dvarpala VPN",
		})
	})
}