package web

import (
	"net/http"
	"strings"
	"time"

	"dvarpala/internal/auth"

	"github.com/gin-gonic/gin"
)

// NotFound answers a request for a path this server does not have.
//
// Inside the walled garden a signed-out client's web requests are rewritten to
// arrive here regardless of what was asked for, so the paths that turn up are
// whatever somebody was opening at the time - a news site, a mail client, or
// the connectivity check their operating system makes by itself. None of them
// exist here, and all of them should end at the sign-in page.
//
// That last case is what makes the portal appear on its own. Every operating
// system tests a new network by fetching a known URL over plain HTTP and
// checking the answer is exactly what it expects. A redirect is not, and that
// is precisely the signal that makes it open its "sign in to network" panel.
// Serving a 404 would leave the person with a broken-looking browser and no
// indication that signing in is what they need to do.
func NotFound(a *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Programmatic callers are told the truth. A redirect to an HTML page
		// in answer to a missing endpoint turns a clear 404 into a confusing
		// parse error somewhere else.
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "no such endpoint", "path": c.Request.URL.Path,
			})
			return
		}

		// Someone already signed in has asked for a page that genuinely does
		// not exist. Telling them to sign in would be a lie.
		//
		// With no way to ask - which is only possible in a test - treat the
		// caller as signed out. That is the safe direction: the portal sends
		// anyone who turns out to have a session on to the success page, so
		// nobody is stranded either way.
		if a != nil && sessionFor(c, a) != nil {
			c.HTML(http.StatusNotFound, "auth-error.html", gin.H{
				"reason":     "That page does not exist.",
				"error_code": "NOT_FOUND",
				"provider":   "",
				"timestamp":  time.Now().Format("2006-01-02 15:04:05"),
			})
			return
		}

		c.Redirect(http.StatusFound, "/")
	}
}
