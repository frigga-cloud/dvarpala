package web

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
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
	g.GET("/activity", h.Activity)
	g.GET("/audit", h.Audit)

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

	if isAdmin(sess) {
		c.Set("session", sess)
		c.Next()
		return
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

	// Activity page.
	Live    []liveRow
	SignIns []signInRow

	// Audit page.
	Trail       []auditRow
	Kinds       []kindRow
	FilterEmail string
	FilterKind  string
}

// liveRow is one person signed in at this moment.
//
// Times arrive pre-formatted because the console template has no function map
// and adding one would mean every page paying for a facility two tables need.
type liveRow struct {
	Email     string
	Provider  string
	ClientIP  string
	Groups    string
	Since     string
	Remaining string
	OnTunnel  bool
}

// signInRow is one person's most recent sign-in.
type signInRow struct {
	Email  string
	Status string
	When   string
	Ago    string
}

// auditRow is one entry of the trail.
type auditRow struct {
	ID      uint
	When    string
	Email   string
	Action  string
	IP      string
	Details string
}

// kindRow is one kind of event and how often it appears, which is how an
// administrator discovers what can be filtered on without reading the source.
type kindRow struct {
	Action string
	Count  int64
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

// Activity answers two questions that get asked in the same breath during an
// incident: who is on the network right now, and who was here recently.
//
// They come from different places and mean different things. The live list is
// Redis, and it is the truth about access - a session there is what the VPN
// consults when it decides whether to open the firewall. Last sign-in is the
// database, and it is only a memory of the last time someone succeeded. A
// person can appear in the second and not the first, which is the ordinary
// case of somebody who has gone home.
func (h *AdminHandler) Activity(c *gin.Context) {
	ctx := c.Request.Context()

	active, err := h.auth.ActiveSessions(ctx)
	view := adminView{Page: "activity"}

	for _, a := range active {
		view.Live = append(view.Live, liveRow{
			Email:     a.Email,
			Provider:  a.Provider,
			ClientIP:  a.ClientIP,
			Groups:    strings.Join(a.Groups, ", "),
			Since:     a.IssuedAt.Format("2006-01-02 15:04"),
			Remaining: humanDuration(a.Remaining),
			OnTunnel:  a.OnTunnel,
		})
	}

	// Last sign-in is a column of the users table, so this is the same read
	// the People page performs.
	users, uErr := h.svc.Users.ListUsers(ctx)
	if err == nil {
		err = uErr
	}

	seen := make([]models.User, 0, len(users))
	for _, u := range users {
		if u.LastLogin != nil {
			seen = append(seen, u)
		}
	}
	sort.Slice(seen, func(i, j int) bool {
		return seen[i].LastLogin.After(*seen[j].LastLogin)
	})
	if len(seen) > 25 {
		seen = seen[:25]
	}

	for _, u := range seen {
		view.SignIns = append(view.SignIns, signInRow{
			Email:  u.Email,
			Status: string(u.Status),
			When:   u.LastLogin.Format("2006-01-02 15:04"),
			Ago:    humanDuration(time.Since(*u.LastLogin)) + " ago",
		})
	}

	h.render(c, view, err)
}

// Audit shows the trail, newest first.
//
// Read-only, and deliberately so: the service offers no way to alter or remove
// a record, and a console that could edit the trail would make it worthless as
// evidence of anything.
func (h *AdminHandler) Audit(c *gin.Context) {
	ctx := c.Request.Context()

	email := strings.TrimSpace(c.Query("email"))
	kind := strings.TrimSpace(c.Query("action"))

	records, err := h.svc.Audit.List(ctx, services.AuditQuery{
		Email: email,
		// A trail is only useful if you can see far enough back to find the
		// thing you came looking for.
		Limit:  200,
		Action: kind,
	})

	view := adminView{Page: "audit", FilterEmail: email, FilterKind: kind}
	for _, r := range records {
		who := r.Email
		if who == "" {
			// Written before anyone was identified - a refused login, or an
			// installation step. Saying so beats an empty cell.
			who = ""
		}
		view.Trail = append(view.Trail, auditRow{
			ID:      r.ID,
			When:    r.CreatedAt.Format("2006-01-02 15:04:05"),
			Email:   who,
			Action:  r.Action,
			IP:      r.IPAddress,
			Details: truncate(r.Details, 90),
		})
	}

	if counts, cErr := h.svc.Audit.Actions(ctx); cErr == nil {
		for a, n := range counts {
			view.Kinds = append(view.Kinds, kindRow{Action: a, Count: n})
		}
		sort.Slice(view.Kinds, func(i, j int) bool {
			if view.Kinds[i].Count != view.Kinds[j].Count {
				return view.Kinds[i].Count > view.Kinds[j].Count
			}
			return view.Kinds[i].Action < view.Kinds[j].Action
		})
	}

	h.render(c, view, err)
}

// humanDuration renders a span the way somebody reading a console wants it:
// coarse, and never more than two units.
func humanDuration(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	if d < time.Minute {
		return "under a minute"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		h := int(d.Hours())
		if m := int(d.Minutes()) % 60; m > 0 {
			return fmt.Sprintf("%dh %dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	}
	days := int(d.Hours()) / 24
	if hrs := int(d.Hours()) % 24; hrs > 0 {
		return fmt.Sprintf("%dd %dh", days, hrs)
	}
	return fmt.Sprintf("%dd", days)
}

// truncate shortens a value to fit a table cell without hiding that it was cut.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
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
