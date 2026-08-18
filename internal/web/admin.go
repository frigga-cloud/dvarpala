package web

import (
	"net/http"

	"dvarpala/internal/auth"
	"dvarpala/internal/database/models"
	"dvarpala/internal/services"

	"github.com/gin-gonic/gin"
)

// adminGroup is the group whose members may open the console. The installer
// creates it and puts the first administrator in it.
const adminGroup = "system_admins"

// AdminHandler serves the read-only administration console.
//
// It reads through the same services the CLI uses, so the console and
// dvarpala-cli can never disagree about who may reach what. Nothing here
// writes; creating and granting still go through the CLI.
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
}

// requireAdmin allows only signed-in members of the administrator group.
//
// Someone signed in but not an administrator is told so, rather than being
// bounced to the login page they have already completed.
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

	c.HTML(http.StatusForbidden, "admin.html", adminView{
		Page:    "denied",
		Session: sess,
	})
	c.Abort()
}

// adminView is everything a console page may render. Each handler fills only
// the fields its own page needs.
type adminView struct {
	Page      string
	Session   *auth.Session
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

// render sends a page, or the same page carrying an error message.
func (h *AdminHandler) render(c *gin.Context, v adminView, err error) {
	v.Session = h.session(c)
	if err != nil {
		v.Error = err.Error()
	}
	c.HTML(http.StatusOK, "admin.html", v)
}

// Users lists everyone, with the groups they belong to.
func (h *AdminHandler) Users(c *gin.Context) {
	list, err := h.svc.Users.ListUsers(c.Request.Context())
	h.render(c, adminView{Page: "users", Users: list}, err)
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

// Groups lists every group.
func (h *AdminHandler) Groups(c *gin.Context) {
	list, err := h.svc.Groups.ListGroups(c.Request.Context())
	h.render(c, adminView{Page: "groups", Groups: list}, err)
}

// Resources lists every protected resource.
func (h *AdminHandler) Resources(c *gin.Context) {
	list, err := h.svc.Resources.ListResources(c.Request.Context())
	h.render(c, adminView{Page: "resources", Resources: list}, err)
}
