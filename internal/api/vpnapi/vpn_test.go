package vpnapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// The internal endpoints decide who gets network access and identify callers
// only by tunnel address, so reaching them must require being the server
// itself. Every VPN client can reach the portal's port; none of them may
// reach these.
func TestInternalEndpointsAreLoopbackOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name       string
		remoteAddr string
		want       int
	}{
		{"the hooks, over IPv4 loopback", "127.0.0.1:54321", http.StatusOK},
		{"the hooks, over IPv6 loopback", "[::1]:54321", http.StatusOK},
		{"a VPN client on the tunnel", "172.30.100.2:54321", http.StatusForbidden},
		{"something on the local network", "192.168.1.50:54321", http.StatusForbidden},
		{"the public internet", "203.0.113.7:54321", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/api/internal/vpn/access/:clientip", localOnly, func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"reached": true})
			})

			req := httptest.NewRequest(http.MethodGet, "/api/internal/vpn/access/172.30.100.2", nil)
			req.RemoteAddr = tc.remoteAddr

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.want {
				t.Errorf("from %s: status = %d, want %d", tc.remoteAddr, w.Code, tc.want)
			}
		})
	}
}

// A caller must not be able to talk its way past the check with a header.
func TestForwardedHeadersCannotForgeLoopback(t *testing.T) {
	gin.SetMode(gin.TestMode)

	r := gin.New()
	r.GET("/api/internal/vpn/access/:clientip", localOnly, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"reached": true})
	})

	for _, header := range []string{"X-Forwarded-For", "X-Real-IP"} {
		req := httptest.NewRequest(http.MethodGet, "/api/internal/vpn/access/172.30.100.2", nil)
		req.RemoteAddr = "172.30.100.2:54321"
		req.Header.Set(header, "127.0.0.1")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want %d", header, w.Code, http.StatusForbidden)
		}
	}
}
