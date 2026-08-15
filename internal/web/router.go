package web

import (
	"dvarpala/internal/auth"
	"dvarpala/internal/config"

	"github.com/gin-gonic/gin"
)

// SetupRoutes mounts the captive portal: the sign-in page a user sees inside
// the walled garden, and the login flow behind it.
func SetupRoutes(r *gin.RouterGroup, authSvc *auth.Service, cfg *config.Config) {
	// Static assets used by the portal page (captive-portal.js, css).
	r.Static("/static", "./web/static")

	NewAuthHandler(authSvc, secureCookies(cfg)).Register(r)
}

// secureCookies reports whether session cookies should be marked Secure.
//
// In debug mode the portal is served over plain HTTP on localhost, where a
// Secure cookie would never be sent back.
func secureCookies(cfg *config.Config) bool {
	return cfg.Server.Mode != "debug"
}
