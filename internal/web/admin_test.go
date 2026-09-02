package web

import "testing"

// The admin console can grant network access, so a form submission must be
// provably from our own page rather than from any site the administrator
// happens to have open.
func TestCSRFTokenIsTiedToTheSession(t *testing.T) {
	a := csrfToken("session-token-a")
	b := csrfToken("session-token-b")

	if a == b {
		t.Fatal("two sessions produced the same token; one administrator could forge another's form")
	}
	if a != csrfToken("session-token-a") {
		t.Fatal("token is not stable for one session; every form would be rejected")
	}
	if a == "" || len(a) != 64 {
		t.Fatalf("token = %q, expected 64 hex characters", a)
	}
}

// The session token itself must not be recoverable from the form field, since
// that field is visible in the page source.
func TestCSRFTokenDoesNotLeakTheSessionToken(t *testing.T) {
	const secret = "a-real-session-token"
	if got := csrfToken(secret); got == secret {
		t.Fatal("the form field is the session token itself")
	}
}
