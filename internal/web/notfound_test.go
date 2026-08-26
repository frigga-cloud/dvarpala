package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

// A failure page that states a fact nobody can act on is a dead end. The
// commonest of these is a spent sign-in: the reason says the attempt was
// invalid or expired, and never that pressing back is what did it.
func TestFailuresExplainWhatToDoAboutThem(t *testing.T) {
	actionable := []string{
		"INVALID_STATE",
		"DOMAIN_NOT_ALLOWED",
		"NOT_AUTHORISED",
		"PROVIDER_UNAVAILABLE",
		"NOT_REDEEMABLE",
	}

	for _, code := range actionable {
		if hintFor(code) == "" {
			t.Errorf("%s offers no guidance, so the page is a dead end", code)
		}
	}

	// The spent-token case is the one people actually hit, and the fix - stop
	// pressing back - has to be in the text.
	if got := hintFor("INVALID_STATE"); !strings.Contains(got, "back") {
		t.Errorf("INVALID_STATE hint does not mention going back: %q", got)
	}

	// An unrecognised code says nothing rather than guessing.
	if got := hintFor("SOMETHING_NEW"); got != "" {
		t.Errorf("unknown code invented guidance: %q", got)
	}
}
