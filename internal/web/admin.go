package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"dvarpala/internal/auth"
	"dvarpala/internal/database/models"
	"dvarpala/internal/services"

	"github.com/gin-gonic/gin"
)

// adminGroup is the group whose members may open the console. The installer
// creates it and puts the first administrator in it.
const adminGroup = "system_admins"

// AdminHandler serves the administration console.
//
// It reads and writes through the same services the CLI uses, so the console
// and dvarpala-cli can never disagree about who may reach what.
type AdminHandler struct {
	auth *auth.Service
	svc  *services.Services
}

// NewAdminHandler creates the console handler.
func NewAdminHandler(a *auth.Service, svc *services.Services) *AdminHandler {
	return &AdminHandler{auth: a, svc: svc}
}

// Register attaches the console routes behind the administrator check.
func (h *AdminHandler) Register(r *gin.RouterGroup) {
	g := r.Group("/admin", h.requireAdmin)

	g.GET("", func(c *gin.Context) { c.Redirect(http.StatusFound, "/admin/users") })
	g.GET("/users", h.Users)
	g.GET("/users/:email", h.UserDetail)
	g.GET("/groups", h.Groups)
	g.GET("/resources", h.Resources)

	// Anything that changes something is a POST, and every POST carries a
	// token tied to the caller's session. Without that, any page on the
	// internet could quietly make an administrator's browser grant access.
	w := g.Group("", h.requireCSRF)
	w.POST("/users/create", h.CreateUser)
	w.POST("/users/deactivate", h.DeactivateUser)
	w.POST("/groups/create", h.CreateGroup)
	w.POST("/groups/assign", h.AssignGroup)
	w.POST("/groups/remove", h.RemoveGroup)
	w.POST("/resources/create", h.CreateResource)
	w.POST("/permissions/grant", h.GrantPermission)
	w.POST("/permissions/revoke", h.RevokePermission)
	w.POST("/vpn/issue", h.IssueProfile)
}

// requireAdmin allows only signed-in members of the administrator group.
func (h *AdminHandler) requireAdmin(c *gin.Context) {
	sess := sessionFor(c, h.auth)
	if sess == nil {
		c.Redirect(http.StatusFound, "/")
		c.Abort()
		return
	}

	for _, g := range sess.Groups {
		if g == adminGroup {
			c.Set("session", sess)
			c.Next()
			return
		}
	}

	c.HTML(http.StatusForbidden, "admin.html", adminView{Page: "denied", Session: sess})
	c.Abort()
}

// csrfToken derives a per-session token from the session token.
//
// The session token lives in an HttpOnly cookie, so a page on another site can
// cause the browser to send the cookie but cannot read it, and therefore
// cannot produce this value.
func csrfToken(sessionToken string) string {
	sum := sha256.Sum256([]byte("dvarpala-csrf|" + sessionToken))
	return hex.EncodeToString(sum[:])
}

// requireCSRF rejects a form submission that did not come from our own page.
func (h *AdminHandler) requireCSRF(c *gin.Context) {
	sess := h.session(c)
	if sess == nil {
		c.Redirect(http.StatusFound, "/")
		c.Abort()
		return
	}

	want := csrfToken(sess.Token)
	got := c.PostForm("csrf")
	if subtle.ConstantTimeCompare([]byte(want), []byte(got)) != 1 {
		c.String(http.StatusForbidden, "This form has expired. Reload the page and try again.")
		c.Abort()
		return
	}
	c.Next()
}

// adminView is everything a console page may render. Each handler fills only
// the fields its own page needs.
type adminView struct {
	Page      string
	Session   *auth.Session
	CSRF      string
	Notice    string
	Error     string
	Users     []models.User
	Groups    []models.Group
	Resources []models.Resource
	User      *models.User
	Grants    []services.AccessGrant
	Allowed   bool
	DenyMsg   string
}

func (h *AdminHandler) session(c *gin.Context) *auth.Session {
	if v, ok := c.Get("session"); ok {
		if sess, ok := v.(*auth.Session); ok {
			return sess
		}
	}
	return nil
}

// render sends a page, carrying any notice or error from the previous action.
func (h *AdminHandler) render(c *gin.Context, v adminView, err error) {
	sess := h.session(c)
	v.Session = sess
	if sess != nil {
		v.CSRF = csrfToken(sess.Token)
	}
	v.Notice = c.Query("ok")
	if v.Error == "" {
		v.Error = c.Query("err")
	}
	if err != nil {
		v.Error = err.Error()
	}
	c.HTML(http.StatusOK, "admin.html", v)
}

// back returns to a page carrying the outcome of an action, so a reload does
// not repeat it.
func (h *AdminHandler) back(c *gin.Context, path string, err error, okMsg string) {
	if err != nil {
		c.Redirect(http.StatusFound, path+"?err="+url.QueryEscape(err.Error()))
		return
	}
	c.Redirect(http.StatusFound, path+"?ok="+url.QueryEscape(okMsg))
}

// ── pages ───────────────────────────────────────────────────────────────────

// Users lists everyone, with the groups they belong to.
func (h *AdminHandler) Users(c *gin.Context) {
	list, err := h.svc.Users.ListUsers(c.Request.Context())
	groups, _ := h.svc.Groups.ListGroups(c.Request.Context())
	h.render(c, adminView{Page: "users", Users: list, Groups: groups}, err)
}

// UserDetail answers the question the VPN asks: what may this person reach?
func (h *AdminHandler) UserDetail(c *gin.Context) {
	email := c.Param("email")

	user, err := h.svc.Users.GetUserByEmail(c.Request.Context(), email)
	if err != nil {
		h.render(c, adminView{Page: "user"}, err)
		return
	}

	view := adminView{Page: "user", User: user}
	if groups, err := h.svc.Groups.ListGroups(c.Request.Context()); err == nil {
		view.Groups = groups
	}

	// The same check the VPN authentication path makes.
	if _, err := h.svc.Users.IsAuthorised(c.Request.Context(), email); err != nil {
		view.DenyMsg = err.Error()
	} else {
		view.Allowed = true
	}

	grants, err := h.svc.Permissions.ResourcesForUser(c.Request.Context(), email)
	view.Grants = grants
	h.render(c, view, err)
}

// Groups lists every group, with what each may reach.
func (h *AdminHandler) Groups(c *gin.Context) {
	list, err := h.svc.Groups.ListGroups(c.Request.Context())
	resources, _ := h.svc.Resources.ListResources(c.Request.Context())
	h.render(c, adminView{Page: "groups", Groups: list, Resources: resources}, err)
}

// Resources lists every protected resource.
func (h *AdminHandler) Resources(c *gin.Context) {
	list, err := h.svc.Resources.ListResources(c.Request.Context())
	h.render(c, adminView{Page: "resources", Resources: list}, err)
}

// ── actions ─────────────────────────────────────────────────────────────────

// CreateUser adds someone. They reach nothing until put in a group.
func (h *AdminHandler) CreateUser(c *gin.Context) {
	email := c.PostForm("email")
	_, err := h.svc.Users.CreateUser(c.Request.Context(), services.CreateUserRequest{
		Email:      email,
		FullName:   c.PostForm("name"),
		Department: c.PostForm("department"),
	})
	h.back(c, "/admin/users", err, "Added "+email+". They reach nothing until you put them in a group.")
}

// DeactivateUser stops someone authenticating again.
func (h *AdminHandler) DeactivateUser(c *gin.Context) {
	email := c.PostForm("email")
	err := h.svc.Users.DeactivateUser(c.Request.Context(), email)
	h.back(c, "/admin/users", err,
		"Deactivated "+email+". An existing VPN session is not cut off until they reconnect.")
}

// CreateGroup adds a group.
func (h *AdminHandler) CreateGroup(c *gin.Context) {
	name := c.PostForm("name")
	_, err := h.svc.Groups.CreateGroup(c.Request.Context(), services.CreateGroupRequest{
		Name:        name,
		Description: c.PostForm("description"),
		Parent:      c.PostForm("parent"),
	})
	h.back(c, "/admin/groups", err, "Created group "+name)
}

// AssignGroup puts someone in a group, which is how access is granted.
func (h *AdminHandler) AssignGroup(c *gin.Context) {
	email, group := c.PostForm("email"), c.PostForm("group")
	err := h.svc.Groups.AddUserToGroup(c.Request.Context(), email, group)
	h.back(c, "/admin/users/"+url.PathEscape(email), err,
		"Added to "+group+". Takes effect when they next reconnect.")
}

// RemoveGroup takes someone out of a group.
func (h *AdminHandler) RemoveGroup(c *gin.Context) {
	email, group := c.PostForm("email"), c.PostForm("group")
	err := h.svc.Groups.RemoveUserFromGroup(c.Request.Context(), email, group)
	h.back(c, "/admin/users/"+url.PathEscape(email), err, "Removed from "+group)
}

// CreateResource registers something access can be granted to.
func (h *AdminHandler) CreateResource(c *gin.Context) {
	port, _ := strconv.Atoi(c.PostForm("port"))
	name := c.PostForm("name")

	_, err := h.svc.Resources.CreateResource(c.Request.Context(), services.CreateResourceRequest{
		Name:        name,
		Type:        c.PostForm("type"),
		URL:         c.PostForm("url"),
		IPAddress:   c.PostForm("ip"),
		Port:        port,
		Description: c.PostForm("description"),
	})

	msg := "Registered " + name
	if err == nil && c.PostForm("ip") == "" {
		// Worth saying out loud: without an address there is nothing to route.
		msg += ". Warning: no IP address, so no route or firewall entry will be created for it."
	}
	h.back(c, "/admin/resources", err, msg)
}

// GrantPermission gives a group access to a resource.
func (h *AdminHandler) GrantPermission(c *gin.Context) {
	group, resource := c.PostForm("group"), c.PostForm("resource")
	err := h.svc.Permissions.Grant(c.Request.Context(), group, resource, c.PostForm("permission"))
	h.back(c, "/admin/groups", err, group+" may now reach "+resource)
}

// RevokePermission removes a group's access to a resource.
func (h *AdminHandler) RevokePermission(c *gin.Context) {
	group, resource := c.PostForm("group"), c.PostForm("resource")
	err := h.svc.Permissions.Revoke(c.Request.Context(), group, resource, c.PostForm("permission"))
	h.back(c, "/admin/groups", err, group+" can no longer reach "+resource)
}

// IssueProfile issues a VPN profile and sends it as a download.
//
// This is the step that previously required SSH and scp: the file contains the
// user's private key, so it is served straight to the administrator's browser
// rather than being written anywhere on the server's public paths.
func (h *AdminHandler) IssueProfile(c *gin.Context) {
	email := c.PostForm("email")

	cfg, err := h.svc.VPNConfigs.Issue(c.Request.Context(), email, "console", 365*24*time.Hour)
	if err != nil {
		h.back(c, "/admin/users/"+url.PathEscape(email), err, "")
		return
	}

	filename := email + ".ovpn"
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "application/x-openvpn-profile", []byte(cfg.ConfigData))
}
