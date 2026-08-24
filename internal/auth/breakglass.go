package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"dvarpala/internal/database/models"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"

	goredis "github.com/go-redis/redis/v8"
)

// ErrNoBreakGlass means a redemption link was never issued, has already been
// used, or has expired.
var ErrNoBreakGlass = errors.New("this emergency access link is not valid")

// Break-glass policy.
const (
	// breakGlassDefault is how long an emergency session lasts unless asked
	// otherwise. Long enough to fix what is broken, short enough that leaving
	// one open by accident is not a standing back door.
	breakGlassDefault = 15 * time.Minute

	// breakGlassMax bounds --minutes. Anyone who genuinely needs longer needs
	// to fix the mail server instead.
	breakGlassMax = 2 * time.Hour
)

func breakGlassKey(code string) string { return "breakglass:" + code }

// BreakGlass issues a session without an identity provider.
//
// Sign-in codes are the only way on to this network, which means a broken
// mail server locks out the very people who would repair it - including
// whoever would repair the mail server. This is the way back in.
//
// It is not a privilege escalation. Anyone who can run it already holds the
// database credentials and could simply create themselves an administrator;
// the difference is that this leaves an audit record saying exactly what
// happened, rather than an ordinary-looking account nobody questions.
//
// What it skips is one step and one step only: proving the person controls
// that mailbox. Everything else still applies - the account must exist, it
// must be active, and its groups decide what opens.
type BreakGlass struct {
	rdb      *redis.Client
	sessions *SessionService
	users    userAuthoriser
	audit    auditLogger
}

// userAuthoriser answers whether an account may sign in at all. Narrowed to
// the one question this asks, so the emergency path can be exercised without
// a database standing by - which is not a luxury when the point of the
// exercise is what happens once things are broken.
type userAuthoriser interface {
	IsAuthorised(ctx context.Context, email string) (*models.User, error)
}

// auditLogger records what happened.
type auditLogger interface {
	Log(ctx context.Context, e services.Entry)
}

// NewBreakGlass creates the issuer.
func NewBreakGlass(rdb *redis.Client, sessions *SessionService,
	users *services.UserService, audit *services.AuditService) *BreakGlass {
	return &BreakGlass{rdb: rdb, sessions: sessions, users: users, audit: audit}
}

// Grant is what an issued emergency access consists of.
type Grant struct {
	// Code is the single-use value that redeems the session in a browser.
	// The session token itself is deliberately not the thing being handed
	// around: this goes into a URL, and URLs are written to access logs.
	Code string

	Email   string
	Groups  []string
	Expires time.Time
}

// Issue creates an emergency session and the link that redeems it.
//
// reason is required by the command that calls this. An emergency access with
// no stated cause is indistinguishable, months later, from an intrusion.
func (b *BreakGlass) Issue(ctx context.Context, email, clientIP, reason string,
	ttl time.Duration) (*Grant, error) {
	if reason == "" {
		return nil, errors.New("a reason is required")
	}

	switch {
	case ttl <= 0:
		ttl = breakGlassDefault
	case ttl > breakGlassMax:
		return nil, fmt.Errorf("%s is longer than the %s maximum",
			ttl.Round(time.Minute), breakGlassMax)
	}

	// The same authorisation check an ordinary login performs. A deactivated
	// account stays deactivated: this is a way past a broken mail server, not
	// past a decision somebody made about an account.
	user, err := b.users.IsAuthorised(ctx, email)
	if err != nil {
		b.record(ctx, nil, "breakglass_refused", clientIP, map[string]interface{}{
			"email": email, "reason": reason, "refused_because": err.Error(),
		})
		return nil, err
	}

	sess, err := b.session(ctx, user, clientIP, ttl)
	if err != nil {
		return nil, err
	}

	code, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("generating redemption code: %w", err)
	}
	if err := b.rdb.Set(ctx, breakGlassKey(code), sess.Token, ttl).Err(); err != nil {
		return nil, fmt.Errorf("storing redemption code: %w", err)
	}

	b.record(ctx, &user.ID, "breakglass_issued", clientIP, map[string]interface{}{
		"email":   user.Email,
		"groups":  sess.Groups,
		"reason":  reason,
		"minutes": int(ttl.Minutes()),
		// Whether this bound the tunnel as well as the browser. The difference
		// is whether network access was granted or only the admin console.
		"binds_tunnel": clientIP != "",
	})

	return &Grant{
		Code: code, Email: user.Email, Groups: sess.Groups, Expires: sess.Expires,
	}, nil
}

// session builds and stores the session itself, at the requested lifetime
// rather than the deployment's ordinary one.
func (b *BreakGlass) session(ctx context.Context, user *models.User,
	clientIP string, ttl time.Duration) (*Session, error) {
	groups := make([]string, 0, len(user.Groups))
	for _, g := range user.Groups {
		groups = append(groups, g.Name)
	}

	// A store of its own, so the short lifetime applies to this session and
	// nothing else.
	short := NewSessionService(b.rdb, ttl)

	sess, err := short.Create(ctx, Session{
		UserID: user.ID,
		Email:  user.Email,
		// Named in the session, so every later record of what this session
		// did says how it was obtained.
		Provider: "break-glass",
		Groups:   groups,
		ClientIP: clientIP,
	})
	if err != nil {
		return nil, fmt.Errorf("creating emergency session: %w", err)
	}
	return sess, nil
}

// Redeem exchanges a code for its session, once.
func (b *BreakGlass) Redeem(ctx context.Context, code string) (*Session, error) {
	if code == "" {
		return nil, ErrNoBreakGlass
	}

	// Atomically, so a link that leaked into a log or a shoulder-surfed
	// terminal cannot be used behind the person it was issued to.
	token, err := consumeStateScript.Run(ctx, b.rdb, []string{breakGlassKey(code)}).Text()
	if errors.Is(err, goredis.Nil) || token == "" {
		return nil, ErrNoBreakGlass
	}
	if err != nil {
		return nil, fmt.Errorf("reading redemption code: %w", err)
	}

	sess, err := b.sessions.Get(ctx, token)
	if errors.Is(err, ErrNoSession) {
		return nil, ErrNoBreakGlass
	}
	if err != nil {
		return nil, err
	}

	b.record(ctx, &sess.UserID, "breakglass_redeemed", sess.ClientIP,
		map[string]interface{}{"email": sess.Email, "groups": sess.Groups})

	return sess, nil
}

func (b *BreakGlass) record(ctx context.Context, userID *uint, action, clientIP string,
	details map[string]interface{}) {
	b.audit.Log(ctx, services.Entry{
		UserID:       userID,
		Action:       action,
		ResourceType: "session",
		IPAddress:    clientIP,
		Details:      details,
	})
}
