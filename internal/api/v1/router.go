package v1

import (
	"dvarpala/internal/api/v1/users"
	"dvarpala/internal/config"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"

	"github.com/gin-gonic/gin"
)

// SetupRoutes mounts the v1 API.
//
// Handlers live in their own packages under api/v1/; this function only wires
// them up. Endpoints still returning a placeholder string are marked TODO and
// are implemented in later phases.
func SetupRoutes(r *gin.RouterGroup, svc *services.Services, redis *redis.Client, cfg *config.Config) {
	// Users - implemented
	users.NewHandler(svc.Users).Register(r)

	// TODO(phase-3): OAuth validation for external tools
	auth := r.Group("/auth")
	{
		auth.POST("/validate", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "auth validation endpoint"})
		})
	}

	// TODO(phase-2): group management
	groups := r.Group("/groups")
	{
		groups.GET("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "list groups endpoint"})
		})
		groups.POST("", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "create group endpoint"})
		})
	}

	// TODO(phase-4): live VPN session status
	vpn := r.Group("/vpn")
	{
		vpn.GET("/status", func(c *gin.Context) {
			c.JSON(200, gin.H{"message": "vpn status endpoint"})
		})
	}
}
