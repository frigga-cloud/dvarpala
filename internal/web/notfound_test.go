package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Inside the walled garden a signed-out client's web requests are rewritten to
// arrive here whatever they asked for. Every operating system tests a new
// network by fetching a known URL over plain HTTP; an answer that is not the
// expected one is what makes it show its "sign in to network" panel.
//
// A 404 here would leave somebody with a browser that looks broken and no
// indication that signing in is what they need to do.
func TestUnknownPagesSendPeopleToTheSignInPage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	paths := []struct {
		name string
		path string
	}{
		{"apple's check", "/hotspot-detect.html"},
		{"android's check", "/generate_204"},
		{"windows' check", "/connecttest.txt"},
		{"whatever they were opening", "/news/story/1"},
		{"the bare root of another site", "/index.html"},
	}

	for _, tc := range paths {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.NoRoute(NotFound(nil))

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))

			if w.Code != http.StatusFound {
				t.Fatalf("%s returned %d, want %d - the portal will not appear by itself",
					tc.path, w.Code, http.StatusFound)
			}
			if got := w.Header().Get("Location"); got != "/" {
				t.Errorf("sent to %q, want %q", got, "/")
			}
		})
	}
}

// A missing endpoint is a fact, and a redirect to an HTML page in answer to it
// turns a clear 404 into a parse error somewhere else.
func TestMissingAPIEndpointsAreNotRedirected(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.NoRoute(NotFound(nil))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/nope", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("returned %d, want %d", w.Code, http.StatusNotFound)
	}
	if ct := w.Header().Get("Content-Type"); ct[:16] != "application/json" {
		t.Errorf("content type is %q, want JSON", ct)
	}
}
