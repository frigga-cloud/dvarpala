package auth

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// DevProvider is a stand-in for a real identity provider, for local
// development and tests.
//
// Instead of redirecting to Google, it redirects to a local page where you
// type an email address. Everything after that - the state check, the domain
// allow-list, the database authorisation, session creation - runs exactly as
// it does in production. Only the "who are you" step is faked.
//
// It must never be enabled on a real deployment: anyone could claim any
// identity. See Guard() below, which the server calls at startup.
type DevProvider struct {
	// BaseURL is where this Dvarpala instance is reachable, e.g.
	// http://localhost:8080
	BaseURL string
}

// NewDevProvider creates the development provider.
func NewDevProvider(baseURL string) *DevProvider {
	return &DevProvider{BaseURL: strings.TrimRight(baseURL, "/")}
}

// Name implements Provider.
func (d *DevProvider) Name() string { return "dev" }

// DisplayName implements Provider.
func (d *DevProvider) DisplayName() string { return "Development login (insecure)" }

// AuthURL sends the browser to a local form rather than to a real provider.
func (d *DevProvider) AuthURL(state string) string {
	return fmt.Sprintf("%s/auth/dev/login?state=%s", d.BaseURL, url.QueryEscape(state))
}

// Exchange treats the code as the email address the form supplied.
//
// A real provider would call its token endpoint here and verify a signature.
func (d *DevProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	email := strings.ToLower(strings.TrimSpace(code))
	if email == "" || !strings.Contains(email, "@") {
		return nil, fmt.Errorf("dev provider: %q is not an email address", code)
	}

	name := email
	if at := strings.Index(email, "@"); at > 0 {
		name = email[:at]
	}

	return &UserInfo{Email: email, Name: name, Provider: d.Name()}, nil
}

// Guard reports whether it is safe to enable the dev provider.
//
// It is refused unless the server is in debug mode, so that a production
// deployment cannot accidentally ship a login that accepts any identity.
func (d *DevProvider) Guard(serverMode string) error {
	if strings.ToLower(strings.TrimSpace(serverMode)) != "debug" {
		return fmt.Errorf("dev provider refused: server.mode is %q, expected \"debug\"", serverMode)
	}
	return nil
}
