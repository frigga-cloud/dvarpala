package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GoogleProvider authenticates against Google, including Google Workspace.
//
// It implements the OAuth 2.0 authorisation-code flow directly rather than
// through a library, so the exchange is visible: redirect the browser, receive
// a code, swap the code for a token, then read the user's identity.
type GoogleProvider struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// HostedDomain, if set, asks Google to restrict the account chooser to
	// one Workspace domain. It is a convenience, not a security control -
	// the allow-list in Service is what actually enforces the domain.
	HostedDomain string

	client *http.Client
}

// Google's endpoints. These match models.GetDefaultProviderConfig, which the
// original developer had already filled in correctly.
const (
	googleAuthURL     = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL    = "https://oauth2.googleapis.com/token"
	googleUserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
)

// NewGoogleProvider creates the Google provider. It returns an error if the
// credentials are missing, so a misconfigured deployment fails at startup
// rather than at the first login attempt.
func NewGoogleProvider(clientID, clientSecret, redirectURL, hostedDomain string) (*GoogleProvider, error) {
	if strings.TrimSpace(clientID) == "" || strings.TrimSpace(clientSecret) == "" {
		return nil, fmt.Errorf("google: client_id and client_secret are required " +
			"(set OAUTH_GOOGLE_CLIENT_ID and OAUTH_GOOGLE_CLIENT_SECRET)")
	}
	if strings.TrimSpace(redirectURL) == "" {
		return nil, fmt.Errorf("google: redirect_url is required and must match " +
			"the value registered in the Google Cloud console exactly")
	}

	return &GoogleProvider{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		HostedDomain: hostedDomain,
		client:       &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// Name implements Provider.
func (g *GoogleProvider) Name() string { return "google" }

// DisplayName implements Provider.
func (g *GoogleProvider) DisplayName() string { return "Google Workspace" }

// AuthURL implements Provider.
func (g *GoogleProvider) AuthURL(state string) string {
	q := url.Values{
		"client_id":     {g.ClientID},
		"redirect_uri":  {g.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},

		// Always show the account chooser: on a shared machine, silently
		// reusing the previous session would authenticate the wrong person.
		"prompt": {"select_account"},
	}
	if g.HostedDomain != "" {
		q.Set("hd", g.HostedDomain)
	}

	return googleAuthURL + "?" + q.Encode()
}

// Exchange swaps the authorisation code for a token, then reads the identity.
func (g *GoogleProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("google: no authorisation code returned")
	}

	token, err := g.exchangeCode(ctx, code)
	if err != nil {
		return nil, err
	}
	return g.fetchUserInfo(ctx, token)
}

// exchangeCode performs the token request.
func (g *GoogleProvider) exchangeCode(ctx context.Context, code string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {g.ClientID},
		"client_secret": {g.ClientSecret},
		"redirect_uri":  {g.RedirectURL},
		"grant_type":    {"authorization_code"},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, googleTokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("google: building token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("google: token request failed: %w", err)
	}
	defer resp.Body.Close()

	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("google: decoding token response: %w", err)
	}

	if body.Error != "" {
		// redirect_uri_mismatch is by far the most common setup mistake, so
		// name the fix rather than only reporting the code.
		if body.Error == "redirect_uri_mismatch" {
			return "", fmt.Errorf("google: redirect_uri_mismatch - the console must "+
				"list exactly %q", g.RedirectURL)
		}
		return "", fmt.Errorf("google: %s: %s", body.Error, body.ErrorDesc)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("google: no access token in response (HTTP %d)", resp.StatusCode)
	}

	return body.AccessToken, nil
}

// fetchUserInfo reads the authenticated user's profile.
func (g *GoogleProvider) fetchUserInfo(ctx context.Context, token string) (*UserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, googleUserInfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("google: building userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("google: userinfo request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: userinfo returned HTTP %d", resp.StatusCode)
	}

	var body struct {
		Email         string `json:"email"`
		VerifiedEmail bool   `json:"verified_email"`
		Name          string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("google: decoding userinfo: %w", err)
	}

	if body.Email == "" {
		return nil, fmt.Errorf("google: no email returned; the email scope may be missing")
	}

	// An unverified address proves nothing about who controls it.
	if !body.VerifiedEmail {
		return nil, fmt.Errorf("google: %s is not a verified address", body.Email)
	}

	return &UserInfo{
		Email:    strings.ToLower(body.Email),
		Name:     body.Name,
		Provider: g.Name(),
	}, nil
}
