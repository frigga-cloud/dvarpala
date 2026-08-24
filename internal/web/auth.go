package web

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"dvarpala/internal/auth"

	"github.com/gin-gonic/gin"
)

// sessionCookie is the browser's handle on its session.
const sessionCookie = "dvarpala_session"

// AuthHandler serves the captive portal and the login flow.
type AuthHandler struct {
	auth *auth.Service

	// secureCookies should be true wherever the portal is served over HTTPS.
	secureCookies bool
}

// NewAuthHandler creates the portal handler.
func NewAuthHandler(a *auth.Service, secureCookies bool) *AuthHandler {
	return &AuthHandler{auth: a, secureCookies: secureCookies}
}

// Register attaches the portal routes.
func (h *AuthHandler) Register(r *gin.RouterGroup) {
	r.GET("/", h.Portal)
	r.GET("/auth/success", h.Success)
	r.GET("/auth/error", h.Failure)
	r.GET("/auth/logout", h.Logout)

	// Development provider form. Kept outside /auth/ so it cannot collide
	// with the /auth/:provider wildcard.
	r.GET("/dev/login", h.DevLoginForm)
	r.POST("/dev/login", h.DevLoginSubmit)

	// Polled by captive-portal.js so the page can notice a completed login.
	r.GET("/api/internal/auth-status", h.Status)

	// Generic provider routes.
	r.GET("/auth/:provider", h.Begin)
	r.GET("/auth/:provider/callback", h.Callback)
}

// Portal serves the sign-in page a user sees inside the walled garden.
//
// The page is built from the providers this deployment actually enabled. It
// used to offer four sign-in buttons regardless, three of which led nowhere:
// a person inside the walled garden has no other page to try, so a button
// that cannot work is worse there than anywhere else.
func (h *AuthHandler) Portal(c *gin.Context) {
	// Already signed in? Say so rather than asking again.
	if sess := h.currentSession(c); sess != nil {
		c.Redirect(http.StatusFound, "/auth/success")
		return
	}

	c.HTML(http.StatusOK, "captive-portal.html", portalView(h.auth, ""))
}

// portalView describes the sign-in page: whether to show the code form, and
// which other providers to offer alongside it.
//
// A package function rather than a method, because the code sign-in handler
// also has to render the portal - when creating a login attempt fails, the
// only honest place to send someone is back to where they started.
func portalView(a *auth.Service, errMessage string) gin.H {
	var (
		otp    bool
		others []gin.H
	)

	for _, p := range a.Providers() {
		if p.Name() == "otp" {
			otp = true
			continue // the code form takes its place, rather than a button
		}
		others = append(others, gin.H{"name": p.Name(), "label": p.DisplayName()})
	}

	return gin.H{
		"otp":       otp,
		"providers": others,
		"none":      !otp && len(others) == 0,
		"error":     errMessage,
	}
}

// Begin starts a login with the named provider.
func (h *AuthHandler) Begin(c *gin.Context) {
	provider := c.Param("provider")

	redirectTo, err := h.auth.Begin(c.Request.Context(), provider, clientIP(c))
	if err != nil {
		h.fail(c, provider, "PROVIDER_UNAVAILABLE", err.Error())
		return
	}

	c.Redirect(http.StatusFound, redirectTo)
}

// Callback completes a login and issues a session cookie.
func (h *AuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	sess, err := h.auth.Complete(c.Request.Context(), provider, code, state, clientIP(c))
	if err != nil {
		h.fail(c, provider, errorCode(err), err.Error())
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(sessionCookie, sess.Token,
		int(time.Until(sess.Expires).Seconds()), "/", "", h.secureCookies, true)

	c.Redirect(http.StatusFound, "/auth/success")
}

// Success shows the "you're in" page.
func (h *AuthHandler) Success(c *gin.Context) {
	sess := h.currentSession(c)
	if sess == nil {
		c.Redirect(http.StatusFound, "/")
		return
	}

	c.HTML(http.StatusOK, "auth-success.html", gin.H{
		"username":  sess.Email,
		"provider":  sess.Provider,
		"timestamp": sess.IssuedAt.Format("2006-01-02 15:04:05"),
	})
}

// Failure shows the error page.
func (h *AuthHandler) Failure(c *gin.Context) {
	c.HTML(http.StatusOK, "auth-error.html", gin.H{
		"reason":     defaultTo(c.Query("reason"), "Authentication failed. Please try again."),
		"error_code": defaultTo(c.Query("code"), "AUTH_FAILED"),
		"provider":   c.Query("provider"),
		"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
	})
}

// Status reports whether the caller has a session.
//
// The captive portal polls this so it can move on once authentication
// completes in another tab or window.
func (h *AuthHandler) Status(c *gin.Context) {
	sess := h.currentSession(c)
	if sess == nil {
		c.JSON(http.StatusOK, gin.H{"authenticated": false, "user": "", "expires_at": ""})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"authenticated": true,
		"user":          sess.Email,
		"provider":      sess.Provider,
		"groups":        sess.Groups,
		"expires_at":    sess.Expires.Format(time.RFC3339),
	})
}

// Logout revokes the session and clears the cookie.
func (h *AuthHandler) Logout(c *gin.Context) {
	if token, err := c.Cookie(sessionCookie); err == nil && token != "" {
		_ = h.auth.Logout(c.Request.Context(), token)
	}
	// Also clear any session bound to this client address, so a stale VPN
	// session cannot immediately re-authenticate the browser.
	_ = h.auth.LogoutClientIP(c.Request.Context(), clientIP(c))
	c.SetCookie(sessionCookie, "", -1, "/", "", h.secureCookies, true)
	c.Redirect(http.StatusFound, "/")
}

// devLoginPage is the stand-in for a provider's own login screen. It exists
// only when the dev provider is enabled, which requires debug mode.
var devLoginPage = template.Must(template.New("dev").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Dvarpala - development login</title>
<style>
 body{font-family:system-ui,sans-serif;background:#1e293b;color:#f8fafc;
      display:flex;min-height:100vh;align-items:center;justify-content:center;margin:0}
 .card{background:#0f172a;padding:2.5rem;border-radius:12px;width:min(420px,90vw);
       box-shadow:0 10px 40px rgba(0,0,0,.4)}
 h1{margin:0 0 .25rem;font-size:1.25rem} p{color:#94a3b8;font-size:.9rem;margin:0 0 1.5rem}
 label{display:block;font-size:.8rem;color:#94a3b8;margin-bottom:.4rem}
 input{width:100%;padding:.7rem;border-radius:8px;border:1px solid #334155;
       background:#1e293b;color:#f8fafc;font-size:1rem;box-sizing:border-box}
 button{width:100%;margin-top:1rem;padding:.75rem;border:0;border-radius:8px;
        background:#6366f1;color:#fff;font-size:1rem;cursor:pointer}
 .warn{margin-top:1.5rem;padding:.7rem;border-radius:8px;background:#422006;
       color:#fbbf24;font-size:.78rem}
</style></head><body>
<form class="card" method="POST" action="/dev/login">
  <h1>Development login</h1>
  <p>Stands in for a real identity provider.</p>
  <input type="hidden" name="state" value="{{.State}}">
  <label for="email">Email address</label>
  <input id="email" name="email" type="email" placeholder="sam@acme.com" required autofocus>
  <button type="submit">Sign in</button>
  <div class="warn">This accepts any email without a password. It is refused
  unless server.mode is "debug".</div>
</form></body></html>`))

// DevLoginForm renders the stand-in provider's login screen.
func (h *AuthHandler) DevLoginForm(c *gin.Context) {
	c.Status(http.StatusOK)
	c.Header("Content-Type", "text/html; charset=utf-8")
	_ = devLoginPage.Execute(c.Writer, gin.H{"State": c.Query("state")})
}

// DevLoginSubmit sends the browser back to the callback, exactly as a real
// provider would - carrying a code (here, the email) and the state.
func (h *AuthHandler) DevLoginSubmit(c *gin.Context) {
	email := c.PostForm("email")
	state := c.PostForm("state")

	c.Redirect(http.StatusFound, fmt.Sprintf("/auth/dev/callback?code=%s&state=%s",
		template.URLQueryEscaper(email), template.URLQueryEscaper(state)))
}

// currentSession returns the caller's session, or nil.
//
// It looks in two places, and every handler uses this one function so they
// cannot disagree about whether someone is signed in:
//
//  1. the browser cookie
//  2. the VPN client-IP key, for a tunnel that is authenticated but whose
//     browser has no cookie (a different browser, or cookies cleared)
func (h *AuthHandler) currentSession(c *gin.Context) *auth.Session {
	return sessionFor(c, h.auth)
}

// sessionFor is the one place a request is turned into a session, shared by
// the portal and the admin console so they cannot disagree about whether
// someone is signed in.
func sessionFor(c *gin.Context, a *auth.Service) *auth.Session {
	if token, err := c.Cookie(sessionCookie); err == nil && token != "" {
		if sess, err := a.Session(c.Request.Context(), token); err == nil {
			return sess
		}
	}

	if sess, err := a.SessionForClientIP(c.Request.Context(), clientIP(c)); err == nil {
		return sess
	}
	return nil
}

func (h *AuthHandler) fail(c *gin.Context, provider, code, reason string) {
	c.Redirect(http.StatusFound, fmt.Sprintf("/auth/error?code=%s&reason=%s&provider=%s",
		template.URLQueryEscaper(code), template.URLQueryEscaper(reason),
		template.URLQueryEscaper(provider)))
}

// errorCode maps an error to a short code for the error page.
func errorCode(err error) string {
	switch {
	case errors.Is(err, auth.ErrBadState):
		return "INVALID_STATE"
	case errors.Is(err, auth.ErrDomainBlocked):
		return "DOMAIN_NOT_ALLOWED"
	default:
		return "NOT_AUTHORISED"
	}
}

// clientIP is the address the session is keyed on. Behind the VPN this is the
// client's tunnel address, which is what the OpenVPN hooks look up.
func clientIP(c *gin.Context) string { return c.ClientIP() }

func defaultTo(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}
