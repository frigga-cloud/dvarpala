package v1

import (
	"dvarpala/internal/config"
	"dvarpala/internal/database"
	"dvarpala/internal/redis"

	"github.com/gin-gonic/gin"
)

func SetupRoutes(r *gin.RouterGroup, db *database.DB, redis *redis.Client, cfg *config.Config) {
	// Auth routes
	auth := r.Group("/auth")
	{
		auth.POST("/validate", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "auth validation endpoint"})
		})
	}

	// User routes
	users := r.Group("/users")
	{
		users.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "list users endpoint"})
		})
		users.POST("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "create user endpoint"})
		})
	}

	// Group routes
	groups := r.Group("/groups")
	{
		groups.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "list groups endpoint"})
		})
		groups.POST("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "create group endpoint"})
		})
	}

	// VPN routes
	vpn := r.Group("/vpn")
	{
		vpn.GET("/status", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "vpn status endpoint"})
		})
	}
}
