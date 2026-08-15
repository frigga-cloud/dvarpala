// Package auth provides authentication and session management.
package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"dvarpala/internal/redis"

	goredis "github.com/go-redis/redis/v8"
)

// ErrNoSession means no valid session exists (absent or expired).
var ErrNoSession = errors.New("no active session")

// Session is what a successful authentication produces.
//
// It is the handover between the web login and the VPN: the portal writes it,
// and the VPN's connect path reads it to decide what access to grant.
type Session struct {
	Token    string    `json:"token"`
	UserID   uint      `json:"user_id"`
	Email    string    `json:"email"`
	Provider string    `json:"provider"`
	Groups   []string  `json:"groups"`
	ClientIP string    `json:"client_ip"`
	IssuedAt time.Time `json:"issued_at"`
	Expires  time.Time `json:"expires_at"`
}

// SessionService stores sessions in Redis, which expires them automatically.
type SessionService struct {
	rdb *redis.Client
	ttl time.Duration
}

// NewSessionService creates a session store. The TTL comes from
// auth.session_duration in configuration (default 8 hours).
func NewSessionService(rdb *redis.Client, ttl time.Duration) *SessionService {
	if ttl <= 0 {
		ttl = 8 * time.Hour
	}
	return &SessionService{rdb: rdb, ttl: ttl}
}

// Key layouts.
//
// A session is written under two keys so it can be found two ways:
//
//	session:<token>  - by the browser's cookie
//	auth:<client-ip> - by the VPN, which only knows the client's tunnel IP
//
// The auth:<ip> form is the one the OpenVPN hooks look up, and is the layout
// the original design specifies.
func sessionKey(token string) string { return "session:" + token }
func authKey(clientIP string) string { return "auth:" + clientIP }

// Create issues a session and stores it under both keys.
func (s *SessionService) Create(ctx context.Context, sess Session) (*Session, error) {
	token, err := newToken()
	if err != nil {
		return nil, fmt.Errorf("generating session token: %w", err)
	}

	sess.Token = token
	sess.IssuedAt = time.Now()
	sess.Expires = sess.IssuedAt.Add(s.ttl)

	payload, err := json.Marshal(sess)
	if err != nil {
		return nil, fmt.Errorf("encoding session: %w", err)
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, sessionKey(token), payload, s.ttl)
	if sess.ClientIP != "" {
		pipe.Set(ctx, authKey(sess.ClientIP), payload, s.ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, fmt.Errorf("storing session: %w", err)
	}

	return &sess, nil
}

// Get returns a session by token.
func (s *SessionService) Get(ctx context.Context, token string) (*Session, error) {
	return s.fetch(ctx, sessionKey(token))
}

// GetByClientIP returns the session for a VPN client address.
//
// This is what the OpenVPN connect path calls: it knows the tunnel IP but not
// the browser's cookie.
func (s *SessionService) GetByClientIP(ctx context.Context, clientIP string) (*Session, error) {
	return s.fetch(ctx, authKey(clientIP))
}

func (s *SessionService) fetch(ctx context.Context, key string) (*Session, error) {
	payload, err := s.rdb.Get(ctx, key).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrNoSession
	}
	if err != nil {
		return nil, fmt.Errorf("reading session: %w", err)
	}

	var sess Session
	if err := json.Unmarshal(payload, &sess); err != nil {
		return nil, fmt.Errorf("decoding session: %w", err)
	}

	// Redis expiry is authoritative, but check anyway: a clock skew or a key
	// written with the wrong TTL should not grant access.
	if time.Now().After(sess.Expires) {
		return nil, ErrNoSession
	}

	return &sess, nil
}

// Revoke deletes a session by token, and its client-IP alias.
//
// Called on logout, on VPN disconnect, and when an administrator forces a
// user off.
func (s *SessionService) Revoke(ctx context.Context, token string) error {
	sess, err := s.Get(ctx, token)
	if err != nil && !errors.Is(err, ErrNoSession) {
		return err
	}

	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, sessionKey(token))
	if sess != nil && sess.ClientIP != "" {
		pipe.Del(ctx, authKey(sess.ClientIP))
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}

// RevokeByClientIP removes the session for a VPN client address. This is what
// the client-disconnect hook calls, so that reconnecting requires signing in
// again.
func (s *SessionService) RevokeByClientIP(ctx context.Context, clientIP string) error {
	sess, err := s.GetByClientIP(ctx, clientIP)
	if errors.Is(err, ErrNoSession) {
		return nil
	}
	if err != nil {
		return err
	}
	return s.Revoke(ctx, sess.Token)
}

// TTL reports how long sessions last.
func (s *SessionService) TTL() time.Duration { return s.ttl }

// newToken returns 256 bits of randomness, hex encoded.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
