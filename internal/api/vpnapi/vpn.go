// Package vpnapi serves the endpoints the VPN server itself calls.
//
// These are not for browsers or end users: they are called by the OpenVPN
// hook scripts running on the same machine, which know a client's tunnel IP
// but nothing else. They must never be exposed beyond localhost.
package vpnapi

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"dvarpala/internal/auth"
	"dvarpala/internal/services"

	"github.com/gin-gonic/gin"
)

// Handler serves the VPN-facing endpoints.
type Handler struct {
	auth  *auth.Service
	perms *services.PermissionService
	users *services.UserService
}

// NewHandler creates the internal API handler.
func NewHandler(a *auth.Service, svc *services.Services) *Handler {
	return &Handler{auth: a, perms: svc.Permissions, users: svc.Users}
}

// Register attaches the internal routes.
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/api/internal/vpn")
	g.GET("/access/:clientip", h.Access)
	g.DELETE("/session/:clientip", h.Disconnect)
}

// Route is one network route to push to a client.
type Route struct {
	// Network and Netmask are in the form OpenVPN expects:
	//   push "route 10.0.5.20 255.255.255.255"
	Network string `json:"network"`
	Netmask string `json:"netmask"`

	// Context, for logs and debugging. Not used for routing.
	Resource   string `json:"resource"`
	Permission string `json:"permission"`
	ViaGroup   string `json:"via_group"`
}

// AccessResponse is what client-connect.sh receives.
type AccessResponse struct {
	Authenticated bool    `json:"authenticated"`
	Email         string  `json:"email,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Routes        []Route `json:"routes"`

	// Reason explains a refusal, for the VPN's log.
	Reason string `json:"reason,omitempty"`
}

// Access answers: may this VPN client have network access, and to what?
//
// This is the question the OpenVPN client-connect hook asks on every
// connection. An unauthenticated client gets no routes, which leaves them in
// the captive portal.
func (h *Handler) Access(c *gin.Context) {
	clientIP := c.Param("clientip")

	sess, err := h.auth.SessionForClientIP(c.Request.Context(), clientIP)
	if errors.Is(err, auth.ErrNoSession) {
		c.JSON(http.StatusOK, AccessResponse{
			Authenticated: false,
			Routes:        []Route{},
			Reason:        "no active session; client remains in the captive portal",
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccessResponse{
			Authenticated: false, Routes: []Route{}, Reason: err.Error(),
		})
		return
	}

	// A session existing is not enough: the user may have been deactivated
	// since signing in, so re-check authorisation on every connect.
	if _, err := h.users.IsAuthorised(c.Request.Context(), sess.Email); err != nil {
		c.JSON(http.StatusOK, AccessResponse{
			Authenticated: false, Routes: []Route{},
			Reason: fmt.Sprintf("session exists but user is no longer authorised: %v", err),
		})
		return
	}

	grants, err := h.perms.ResourcesForUser(c.Request.Context(), sess.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccessResponse{
			Authenticated: false, Routes: []Route{}, Reason: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AccessResponse{
		Authenticated: true,
		Email:         sess.Email,
		Groups:        sess.Groups,
		Routes:        toRoutes(grants),
	})
}

// Disconnect revokes the session for a client address.
//
// Called by the client-disconnect hook, so that reconnecting requires signing
// in again - the "access is revoked on disconnect" half of the design.
func (h *Handler) Disconnect(c *gin.Context) {
	clientIP := c.Param("clientip")

	if err := h.auth.LogoutClientIP(c.Request.Context(), clientIP); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"revoked": true, "client_ip": clientIP})
}

// toRoutes turns access grants into pushable routes.
//
// Only resources with an IP address can be routed to. A dashboard identified
// only by URL is reachable once its host is routable, so it contributes no
// route of its own.
func toRoutes(grants []services.AccessGrant) []Route {
	routes := make([]Route, 0, len(grants))
	seen := make(map[string]bool)

	for _, g := range grants {
		ip := strings.TrimSpace(g.Resource.IPAddress)
		if ip == "" {
			continue
		}

		network, netmask := splitCIDR(ip)
		key := network + "/" + netmask
		if seen[key] {
			continue // the same host granted via two groups needs one route
		}
		seen[key] = true

		routes = append(routes, Route{
			Network:    network,
			Netmask:    netmask,
			Resource:   g.Resource.Name,
			Permission: string(g.Permission),
			ViaGroup:   g.ViaGroup,
		})
	}
	return routes
}

// splitCIDR converts an address into the network/netmask pair OpenVPN wants.
// A bare address becomes a single-host route.
func splitCIDR(addr string) (network, netmask string) {
	if !strings.Contains(addr, "/") {
		return addr, "255.255.255.255"
	}

	parts := strings.SplitN(addr, "/", 2)
	masks := map[string]string{
		"8": "255.0.0.0", "16": "255.255.0.0", "21": "255.255.248.0",
		"24": "255.255.255.0", "26": "255.255.255.192", "32": "255.255.255.255",
	}
	if m, ok := masks[parts[1]]; ok {
		return parts[0], m
	}
	return parts[0], "255.255.255.255"
}
