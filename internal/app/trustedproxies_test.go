package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// Sessions are keyed on the client's tunnel address, so a client that can
// forge that address can bind a session to a tunnel it does not own - granting
// network access to somebody else. Gin trusts X-Forwarded-For from every peer
// by default, which made exactly that possible.
//
// This pins the behaviour: with no trusted proxies configured, the forwarded
// header must be ignored.
func TestForwardedHeaderIsIgnoredByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	if err := r.SetTrustedProxies(nil); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}

	var seen string
	r.GET("/whoami", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.RemoteAddr = "10.9.9.9:1234"
	req.Header.Set("X-Forwarded-For", "172.30.100.77")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if seen == "172.30.100.77" {
		t.Fatal("X-Forwarded-For was believed; a client could claim another client's address")
	}
	if seen != "10.9.9.9" {
		t.Errorf("ClientIP() = %q, want the real peer address 10.9.9.9", seen)
	}
}

// A deployment behind a real reverse proxy must still be able to see the
// original client, so the trust list has to work when it is set.
func TestForwardedHeaderHonouredFromAConfiguredProxy(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	if err := r.SetTrustedProxies([]string{"10.9.9.9"}); err != nil {
		t.Fatalf("SetTrustedProxies: %v", err)
	}

	var seen string
	r.GET("/whoami", func(c *gin.Context) {
		seen = c.ClientIP()
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/whoami", nil)
	req.RemoteAddr = "10.9.9.9:1234"
	req.Header.Set("X-Forwarded-For", "172.30.100.77")
	r.ServeHTTP(httptest.NewRecorder(), req)

	if seen != "172.30.100.77" {
		t.Errorf("ClientIP() = %q, want 172.30.100.77 from the trusted proxy", seen)
	}
}
