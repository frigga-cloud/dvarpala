package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"dvarpala/internal/redis"
	"dvarpala/internal/services"

	goredis "github.com/go-redis/redis/v8"
)

// Errors returned by Service.
var (
	ErrBadState      = errors.New("invalid or expired login attempt")
	ErrDomainBlocked = errors.New("email domain not allowed")
)

// stateTTL is how long a login attempt may sit unfinished.
const stateTTL = 10 * time.Minute

// Service runs the authentication flow.
//
// The two-step check of the product design happens in Complete():
//
//	step 1  the provider proves the person controls that email
//	step 2  Dvarpala's own database decides whether that email may have access
//
// Passing step 1 alone grants nothing.
type Service struct {
	providers *Registry
	sessions  *SessionService
	users     *services.UserService
	perms     *services.PermissionService
	audit     *services.AuditService
	rdb       *redis.Client

	// allowedDomains restricts which email domains may authenticate at all.
	// Empty means any domain is acceptable.
	allowedDomains []string
}

// NewService creates the authentication service.
func NewService(providers *Registry, sessions *SessionService, svc *services.Services,
	rdb *redis.Client, allowedDomains []string) *Service {
	return &Service{
		providers:      providers,
		sessions:       sessions,
		users:          svc.Users,
		perms:          svc.Permissions,
		audit:          svc.Audit,
		rdb:            rdb,
		allowedDomains: normaliseDomains(allowedDomains),
	}
}

// Providers exposes the enabled providers, for rendering the sign-in page.
func (s *Service) Providers() []Provider { return s.providers.Enabled() }

// Begin starts a login and returns the URL to send the browser to.
//
// The state token defends against cross-site request forgery: it is random,
// stored server-side, and must come back unchanged. It is single-use.
func (s *Service) Begin(ctx context.Context, providerName, clientIP string) (string, error) {
	provider, err := s.providers.Get(providerName)
	if err != nil {
		return "", err
	}

	state, err := newToken()
	if err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}

	// Bind the state to the provider and client so a token issued for one
	// login cannot complete another.
	value := providerName + "|" + clientIP
	if err := s.rdb.Set(ctx, stateKey(state), value, stateTTL).Err(); err != nil {
		return "", fmt.Errorf("storing state: %w", err)
	}

	return provider.AuthURL(state), nil
}

// Complete finishes a login: it validates the state, resolves the identity,
// applies the domain allow-list, checks the database, and issues a session.
func (s *Service) Complete(ctx context.Context, providerName, code, state, clientIP string) (*Session, error) {
	provider, err := s.providers.Get(providerName)
	if err != nil {
		return nil, err
	}

	if err := s.consumeState(ctx, state, providerName); err != nil {
		return nil, err
	}

	// Step 1: who is this?
	info, err := provider.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("verifying identity: %w", err)
	}

	if !s.domainAllowed(info.Email) {
		s.logDenied(ctx, info.Email, clientIP, "domain_not_allowed")
		return nil, fmt.Errorf("%w: %s", ErrDomainBlocked, info.Email)
	}

	// Step 2: may they have access?
	user, err := s.users.IsAuthorised(ctx, info.Email)
	if err != nil {
		s.logDenied(ctx, info.Email, clientIP, "not_authorised")
		return nil, err
	}

	// Which groups they are in travels with the session, so the VPN does not
	// have to query the database on every connect.
	groups := make([]string, 0, len(user.Groups))
	for _, g := range user.Groups {
		groups = append(groups, g.Name)
	}

	sess, err := s.sessions.Create(ctx, Session{
		UserID:   user.ID,
		Email:    user.Email,
		Provider: info.Provider,
		Groups:   groups,
		ClientIP: clientIP,
	})
	if err != nil {
		return nil, err
	}

	_ = s.users.RecordLogin(ctx, user.ID)

	s.audit.Log(ctx, services.Entry{
		UserID:       &user.ID,
		Action:       "authentication_success",
		ResourceType: "session",
		IPAddress:    clientIP,
		Details: map[string]interface{}{
			"email": user.Email, "provider": info.Provider, "groups": groups,
		},
	})

	return sess, nil
}

// Logout revokes a session.
func (s *Service) Logout(ctx context.Context, token string) error {
	sess, err := s.sessions.Get(ctx, token)
	if err != nil && !errors.Is(err, ErrNoSession) {
		return err
	}
	if sess != nil {
		s.audit.Log(ctx, services.Entry{
			UserID:       &sess.UserID,
			Action:       "logout",
			ResourceType: "session",
			IPAddress:    sess.ClientIP,
			Details:      map[string]interface{}{"email": sess.Email},
		})
	}
	return s.sessions.Revoke(ctx, token)
}

// consumeState validates a state token and deletes it, so it cannot be reused.
func (s *Service) consumeState(ctx context.Context, state, providerName string) error {
	if state == "" {
		return ErrBadState
	}

	value, err := s.rdb.GetDel(ctx, stateKey(state)).Result()
	if errors.Is(err, goredis.Nil) {
		return ErrBadState
	}
	if err != nil {
		return fmt.Errorf("checking state: %w", err)
	}

	if !strings.HasPrefix(value, providerName+"|") {
		return fmt.Errorf("%w: issued for a different provider", ErrBadState)
	}
	return nil
}

// domainAllowed applies the configured email domain allow-list.
func (s *Service) domainAllowed(email string) bool {
	if len(s.allowedDomains) == 0 {
		return true
	}

	at := strings.LastIndex(email, "@")
	if at < 0 {
		return false
	}
	domain := strings.ToLower(email[at+1:])

	for _, allowed := range s.allowedDomains {
		if domain == allowed {
			return true
		}
	}
	return false
}

func (s *Service) logDenied(ctx context.Context, email, clientIP, reason string) {
	s.audit.Log(ctx, services.Entry{
		Action:       "authentication_denied",
		ResourceType: "session",
		IPAddress:    clientIP,
		Details:      map[string]interface{}{"email": email, "reason": reason},
	})
}

func stateKey(state string) string { return "oauth_state:" + state }

func normaliseDomains(in []string) []string {
	out := make([]string, 0, len(in))
	for _, d := range in {
		d = strings.ToLower(strings.TrimSpace(d))
		if d != "" {
			out = append(out, d)
		}
	}
	return out
}

// Session returns the session for a token, if it is still valid.
func (s *Service) Session(ctx context.Context, token string) (*Session, error) {
	return s.sessions.Get(ctx, token)
}

// SessionForClientIP returns the session for a VPN client address. This is the
// lookup the OpenVPN connect path performs.
func (s *Service) SessionForClientIP(ctx context.Context, clientIP string) (*Session, error) {
	return s.sessions.GetByClientIP(ctx, clientIP)
}

// LogoutClientIP revokes whatever session is bound to a client address.
func (s *Service) LogoutClientIP(ctx context.Context, clientIP string) error {
	return s.sessions.RevokeByClientIP(ctx, clientIP)
}

// ClientDisconnected shortens the session for a client address rather than
// deleting it, so that a reconnect can still apply the routes the user just
// authenticated for. See disconnectGrace.
func (s *Service) ClientDisconnected(ctx context.Context, clientIP string) error {
	err := s.sessions.Disconnected(ctx, clientIP)
	if errors.Is(err, ErrNoSession) {
		return nil
	}
	return err
}

// ClientReconnected restores a session's full lifetime.
func (s *Service) ClientReconnected(ctx context.Context, clientIP string) error {
	return s.sessions.Reconnected(ctx, clientIP)
}
