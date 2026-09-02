// Package vpnapi serves the endpoints the VPN server itself calls.
//
// These are not for browsers or end users: they are called by the OpenVPN
// hook scripts running on the same machine, which know a client's tunnel IP
// but nothing else. They must never be exposed beyond localhost.
package vpnapi

import (
	"errors"
	"fmt"
	"log"
	"net"
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
	audit *services.AuditService
}

// NewHandler creates the internal API handler.
func NewHandler(a *auth.Service, svc *services.Services) *Handler {
	return &Handler{auth: a, perms: svc.Permissions, users: svc.Users, audit: svc.Audit}
}

// Register attaches the internal routes, behind the loopback check.
func (h *Handler) Register(r *gin.RouterGroup) {
	g := r.Group("/api/internal/vpn", localOnly)
	g.GET("/access/:clientip", h.Access)
	g.DELETE("/session/:clientip", h.Disconnect)
}

// localOnly refuses any caller that is not the server itself.
//
// These endpoints answer by tunnel address rather than by any credential the
// caller presents, so without this anyone who can reach the portal - which is
// every VPN client, authenticated or not - could read any user's access map
// and end any user's session.
//
// RemoteIP, not ClientIP: the peer address of the connection, never a header a
// caller can set. The OpenVPN hooks reach us over loopback (DVARPALA_API
// defaults to http://127.0.0.1:8080), so nothing legitimate is affected. A
// deployment that ever moved the hooks off this machine would need a real
// credential here rather than a wider address range.
func localOnly(c *gin.Context) {
	ip := net.ParseIP(c.RemoteIP())
	if ip == nil || !ip.IsLoopback() {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
			"error": "this endpoint is served only to the machine itself",
		})
		return
	}
	c.Next()
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
	Authenticated bool     `json:"authenticated"`
	Email         string   `json:"email,omitempty"`
	Groups        []string `json:"groups,omitempty"`
	Routes        []Route  `json:"routes"`

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
	commonName := c.Query("cn")

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

	// A session is stored against a tunnel address, and tunnel addresses are
	// handed out again as clients come and go. Whoever receives one next would
	// otherwise inherit whatever the previous holder had earned, for as long
	// as the disconnect grace period lasts.
	//
	// The certificate settles it: the session records who signed in, and
	// OpenVPN reports whose certificate this connection presented. Until now
	// those two facts never met.
	if !sameIdentity(commonName, sess.Email) {
		h.audit.Log(c.Request.Context(), services.Entry{
			UserID:       &sess.UserID,
			Action:       "vpn_access_denied",
			ResourceType: "vpn_session",
			IPAddress:    clientIP,
			Details: map[string]interface{}{
				"reason":       "certificate does not match the session at this address",
				"session_for":  sess.Email,
				"connected_as": commonName,
			},
		})

		c.JSON(http.StatusOK, AccessResponse{
			Authenticated: false, Routes: []Route{},
			Reason: "the session at this address belongs to somebody else",
		})
		return
	}

	// A session existing is not enough: the user may have been deactivated
	// since signing in, so re-check authorisation on every connect.
	if _, err := h.users.IsAuthorised(c.Request.Context(), sess.Email); err != nil {
		// Worth recording: someone holding a valid session was refused because
		// their account changed underneath them. That is the kill switch working,
		// and an administrator should be able to see it happen.
		h.audit.Log(c.Request.Context(), services.Entry{
			UserID:       &sess.UserID,
			Action:       "vpn_access_denied",
			ResourceType: "vpn_session",
			IPAddress:    clientIP,
			Details: map[string]interface{}{
				"email": sess.Email, "reason": err.Error(),
			},
		})

		c.JSON(http.StatusOK, AccessResponse{
			Authenticated: false, Routes: []Route{},
			Reason: fmt.Sprintf("session exists but user is no longer authorised: %v", err),
		})
		return
	}

	// The client is back and still authorised, so restore the full session
	// lifetime that the preceding disconnect shortened.
	_ = h.auth.ClientReconnected(c.Request.Context(), clientIP)

	grants, err := h.perms.ResourcesForUser(c.Request.Context(), sess.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, AccessResponse{
			Authenticated: false, Routes: []Route{}, Reason: err.Error(),
		})
		return
	}

	routes := toRoutes(grants)

	// The record that matters most: this person, at this address, was let on to
	// the network, and these are the resources it opened for them.
	h.audit.Log(c.Request.Context(), services.Entry{
		UserID:       &sess.UserID,
		Action:       "vpn_access_granted",
		ResourceType: "vpn_session",
		IPAddress:    clientIP,
		Details: map[string]interface{}{
			"email":     sess.Email,
			"groups":    sess.Groups,
			"routes":    len(routes),
			"resources": grantedResources(grants),
		},
	})

	c.JSON(http.StatusOK, AccessResponse{
		Authenticated: true,
		Email:         sess.Email,
		Groups:        sess.Groups,
		Routes:        routes,
	})
}

// Disconnect ends network access for a client address.
//
// Called by the client-disconnect hook. The session is shortened to a brief
// grace window rather than deleted, because a reconnect - which is how a
// newly-authenticated client receives its routes - necessarily begins with a
// disconnect. Deleting here would make full access unreachable.
//
// Access itself stops immediately regardless: the tunnel is gone, so the
// pushed routes are gone with it.
func (h *Handler) Disconnect(c *gin.Context) {
	clientIP := c.Param("clientip")

	// Read the session before shortening it, so the record can name who left
	// rather than only which address went quiet.
	sess, _ := h.auth.SessionForClientIP(c.Request.Context(), clientIP)

	if err := h.auth.ClientDisconnected(c.Request.Context(), clientIP); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	entry := services.Entry{
		Action:       "vpn_disconnected",
		ResourceType: "vpn_session",
		IPAddress:    clientIP,
	}
	if sess != nil {
		entry.UserID = &sess.UserID
		entry.Details = map[string]interface{}{"email": sess.Email}
	}
	h.audit.Log(c.Request.Context(), entry)

	c.JSON(http.StatusOK, gin.H{"grace_period": true, "client_ip": clientIP})
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

		network, netmask, ok := services.SplitCIDR(ip)
		if !ok {
			// Registering a resource refuses an address like this, so reaching
			// here means a row that predates that check. Say so: the symptom
			// otherwise is a grant that looks right everywhere and routes
			// nothing.
			log.Printf("vpn: resource %q has an address that cannot be routed (%q) - skipping",
				g.Resource.Name, ip)
			continue
		}

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

// grantedResources names the resources a connection opened, for the audit
// record. Names rather than IDs, because the reader of an audit trail is a
// person asking "what could they reach?".
func grantedResources(grants []services.AccessGrant) []string {
	names := make([]string, 0, len(grants))
	seen := make(map[string]bool)

	for _, g := range grants {
		if seen[g.Resource.Name] {
			continue // the same resource granted via two groups is one resource
		}
		seen[g.Resource.Name] = true
		names = append(names, g.Resource.Name)
	}
	return names
}

// sameIdentity reports whether a connecting certificate belongs to the person
// whose session is stored at this address.
//
// An absent name is accepted. The hook has only sent one since this check
// existed, and refusing without it would lock every client out of a server
// whose hooks had not been updated alongside it - an upgrade that half
// succeeds should not deny everybody access. The endpoint is served only to
// this machine, so the caller is the hook rather than anyone who could choose
// to leave the name out.
func sameIdentity(commonName, email string) bool {
	name := strings.TrimSpace(commonName)
	if name == "" {
		return true
	}
	return strings.EqualFold(name, strings.TrimSpace(email))
}
