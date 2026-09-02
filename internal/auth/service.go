package auth

import (
	"context"
	"errors"
	"fmt"
	"log"
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

// reconnectDelay is how long to leave a newly signed-in client's tunnel alone
// before restarting it.
//
// Long enough for the browser to receive the cookie and load the page that
// says it worked - two round trips over the tunnel that is about to be cut.
// Short enough that nobody wonders whether their access is coming.
const reconnectDelay = 3 * time.Second

// consumeStateScript is GETDEL, written out. GETDEL needs Redis 6.2, and
// Ubuntu 22.04 ships 6.0, so every apt-installed server would fail to log
// anyone in. A script is atomic on any Redis that supports EVAL.
var consumeStateScript = goredis.NewScript(`
local v = redis.call('GET', KEYS[1])
if v then redis.call('DEL', KEYS[1]) end
return v
`)

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

	// otp is set only when code sign-in is enabled for this deployment.
	otp *OTPStore

	// breakGlass redeems the emergency links the CLI issues.
	breakGlass *BreakGlass

	// reconnect makes a client re-establish its tunnel after signing in, when
	// the VPN offers a way to. Optional: without it the person has to
	// reconnect by hand, which is what they had to do before.
	reconnect Reconnector

	// domains is the allow-list held in the database, which an administrator
	// can change from the console. When present it is authoritative.
	domains *services.DomainService

	// allowedDomains is the fallback for a service built without a database -
	// tests, and tools that only need the login rules. Empty means any domain
	// is acceptable.
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
		domains:        svc.Domains,
		allowedDomains: normaliseDomains(allowedDomains),
	}
}

// Providers exposes the enabled providers, for rendering the sign-in page.
func (s *Service) Providers() []Provider { return s.providers.Enabled() }

// ActiveSessions lists everyone signed in right now.
//
// The session store is deliberately not exported, because a caller holding it
// could mint or revoke a session without the checks this service performs.
// Reading who is present carries no such risk, and the console needs it to
// answer the question an administrator actually asks during an incident: who
// is on the network at this moment.
func (s *Service) ActiveSessions(ctx context.Context) ([]ActiveSession, error) {
	return s.sessions.Active(ctx)
}

// SessionTTL reports how long a session lasts in this deployment.
func (s *Service) SessionTTL() time.Duration { return s.sessions.TTL() }

// Begin starts a login and returns the URL to send the browser to.
//
// The state token defends against cross-site request forgery: it is random,
// stored server-side, and must come back unchanged. It is single-use.
func (s *Service) Begin(ctx context.Context, providerName, clientIP string) (string, error) {
	provider, err := s.providers.Get(providerName)
	if err != nil {
		s.logFailed(ctx, providerName, clientIP, "unknown_provider", err)
		return "", err
	}

	state, err := s.NewLoginAttempt(ctx, providerName, clientIP)
	if err != nil {
		return "", err
	}

	return provider.AuthURL(state), nil
}

// NewLoginAttempt records a login attempt and returns its state token.
//
// Begin uses this and then hands the token to the provider. The portal calls
// it directly, because asking for an email address and sending the code are
// one action there rather than a redirect to somewhere else - but the attempt
// still has to exist first, or Complete would have nothing to validate
// against and the sign-in could not finish.
func (s *Service) NewLoginAttempt(ctx context.Context, providerName, clientIP string) (string, error) {
	state, err := newToken()
	if err != nil {
		return "", fmt.Errorf("generating state: %w", err)
	}

	// Bind the state to the provider and client so a token issued for one
	// login cannot complete another.
	value := providerName + "|" + clientIP
	if err := s.rdb.Set(ctx, stateKey(state), value, stateTTL).Err(); err != nil {
		s.logFailed(ctx, providerName, clientIP, "state_not_stored", err)
		return "", fmt.Errorf("storing state: %w", err)
	}

	return state, nil
}

// Complete finishes a login: it validates the state, resolves the identity,
// applies the domain allow-list, checks the database, and issues a session.
func (s *Service) Complete(ctx context.Context, providerName, code, state, clientIP string) (*Session, error) {
	provider, err := s.providers.Get(providerName)
	if err != nil {
		s.logFailed(ctx, providerName, clientIP, "unknown_provider", err)
		return nil, err
	}

	if err := s.consumeState(ctx, state, providerName); err != nil {
		s.logFailed(ctx, providerName, clientIP, "bad_state", err)
		return nil, err
	}

	// Step 1: who is this?
	info, err := provider.Exchange(ctx, code)
	if err != nil {
		s.logFailed(ctx, providerName, clientIP, "identity_not_verified", err)
		return nil, fmt.Errorf("verifying identity: %w", err)
	}

	if !s.domainAllowed(ctx, info.Email) {
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
		s.logFailed(ctx, providerName, clientIP, "session_not_stored", err)
		return nil, err
	}

	_ = s.users.RecordLogin(ctx, user.ID)

	// Close the walled-garden tunnel they signed in over, so their client
	// reconnects and picks up the routes this login has just earned.
	//
	// Shortly, and not now. The browser is still being answered over that
	// tunnel: it has yet to receive the cookie or follow the redirect to the
	// page saying it worked. Killing the tunnel in the middle of that lost
	// both, and the person was shown a failure until they refreshed - having
	// in fact signed in successfully.
	//
	// A background context, because the request's own is cancelled the moment
	// the response is done, which is before this runs.
	if s.reconnect != nil {
		email := user.Email
		go func() {
			time.Sleep(reconnectDelay)
			if _, err := s.reconnect.Reconnect(context.Background(), email); err != nil {
				log.Printf("vpn: could not prompt %s to reconnect: %v", email, err)
			}
		}()
	}

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

	value, err := consumeStateScript.Run(ctx, s.rdb, []string{stateKey(state)}).Text()
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

// domainAllowed applies the email domain allow-list.
//
// The database is asked when there is one, so a domain added in the console
// takes effect on the next sign-in rather than the next restart. A database
// that cannot answer refuses the login: an allow-list that fails open is not
// an allow-list, and the alternative is that a database blip briefly admits
// every domain on the internet.
func (s *Service) domainAllowed(ctx context.Context, email string) bool {
	if s.domains != nil {
		ok, err := s.domains.Allowed(ctx, email)
		if err != nil {
			log.Printf("auth: could not read the domain allow-list, refusing %s: %v", email, err)
			return false
		}
		return ok
	}

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

// logFailed records a login that broke down before anyone was identified.
//
// Distinct from logDenied, which means "we know who you are and you may not
// have access". This means the machinery itself did not get that far: an
// unknown provider, a stale state token, a provider that would not answer. The
// person sees only a redirect to the error page, so without this an
// administrator has nothing at all to look at - which is exactly how a
// disabled provider once cost an afternoon.
func (s *Service) logFailed(ctx context.Context, provider, clientIP, stage string, cause error) {
	details := map[string]interface{}{"provider": provider, "stage": stage}
	if cause != nil {
		details["error"] = cause.Error()
	}

	s.audit.Log(ctx, services.Entry{
		Action:       "authentication_failed",
		ResourceType: "session",
		IPAddress:    clientIP,
		Details:      details,
	})
}

func (s *Service) logDenied(ctx context.Context, email, clientIP, reason string) {
	s.audit.Log(ctx, services.Entry{
		Action:       "authentication_denied",
		ResourceType: "session",
		IPAddress:    clientIP,
		Details:      map[string]interface{}{"email": email, "reason": reason},
	})
}

// EnableOTP turns on signing in with an emailed code.
func (s *Service) EnableOTP(store *OTPStore) { s.otp = store }

// EnableBreakGlass turns on redeeming the emergency links the CLI issues.
//
// Always on: the situation it exists for is one where nothing else works, so
// a deployment that had to remember to enable it would not have it when it
// mattered. Issuing still requires shell access to the server.
func (s *Service) EnableBreakGlass(b *BreakGlass) { s.breakGlass = b }

// EnableReconnect lets a completed login apply itself without the person
// disconnecting and reconnecting by hand.
//
// Routes are chosen when a tunnel is established and cannot be changed after,
// so somebody who signs in while connected is still holding the walled-garden
// tunnel they arrived on. Closing it makes their client reconnect by itself -
// every VPN client does - and the second connection is the one that gets
// their routes. The person sees access appear.
func (s *Service) EnableReconnect(r Reconnector) { s.reconnect = r }

// Reconnector asks a client to re-establish its tunnel.
type Reconnector interface {
	Reconnect(ctx context.Context, commonName string) (int, error)
}

// RedeemBreakGlass exchanges an emergency link for its session.
func (s *Service) RedeemBreakGlass(ctx context.Context, code, clientIP string) (*Session, error) {
	if s.breakGlass == nil {
		return nil, ErrNoBreakGlass
	}

	sess, err := s.breakGlass.Redeem(ctx, code)
	if err != nil {
		s.logFailed(ctx, "break-glass", clientIP, "not_redeemable", err)
		return nil, err
	}
	return sess, nil
}

// RequestCode sends a sign-in code, if the address is one that could sign in.
//
// The caller learns nothing about whether it was. A portal that answers "no
// such user" is a way to enumerate an organisation's staff, and this one is
// reachable by anyone holding a VPN certificate. Refusals go to the audit
// trail instead, where an administrator can see them and an attacker cannot.
func (s *Service) RequestCode(ctx context.Context, email, clientIP string) error {
	if s.otp == nil {
		return errors.New("code sign-in is not enabled")
	}
	email = strings.ToLower(strings.TrimSpace(email))

	if !s.domainAllowed(ctx, email) {
		s.logDenied(ctx, email, clientIP, "domain_not_allowed")
		return nil
	}
	if _, err := s.users.IsAuthorised(ctx, email); err != nil {
		s.logDenied(ctx, email, clientIP, "not_authorised")
		return nil
	}

	// Rate limits and delivery failures are returned: the first is something
	// the person can act on, and the second is something they must not be
	// left waiting on in silence.
	err := s.otp.Request(ctx, email)

	// Record the request either way. Without this, "I never got a code" has
	// no answer: nothing distinguishes a code the mail server refused from
	// one that was delivered and ignored, or from a person who never asked.
	details := map[string]interface{}{"email": email}
	action := "signin_code_sent"
	if err != nil {
		action = "signin_code_failed"
		details["error"] = err.Error()
	}
	s.audit.Log(ctx, services.Entry{
		Action:       action,
		ResourceType: "session",
		IPAddress:    clientIP,
		Details:      details,
	})

	return err
}

// VerifyCode checks a code against a login attempt and, on success, marks the
// attempt verified so Complete can exchange it for a session.
func (s *Service) VerifyCode(ctx context.Context, email, code, state string) error {
	if s.otp == nil {
		return errors.New("code sign-in is not enabled")
	}
	return s.otp.Verify(ctx, email, code, state)
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

// SignedIn reports whether a tunnel address belongs to somebody who has
// identified themselves. This is the question the idle sweep asks of every
// open tunnel.
func (s *Service) SignedIn(ctx context.Context, clientIP string) bool {
	_, err := s.sessions.GetByClientIP(ctx, clientIP)
	return err == nil
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
