package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"dvarpala/internal/config"
	"dvarpala/internal/redis"
)

// newTestSessions connects to a local Redis, skipping the test if none is
// running. Sessions are stored in DB 15 to keep test data away from anything
// real.
func newTestSessions(t *testing.T) *SessionService {
	t.Helper()

	rdb, err := redis.NewClient(config.RedisConfig{Addr: "localhost:6379", DB: 15})
	if err != nil {
		t.Skipf("redis not available on localhost:6379: %v", err)
	}

	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
	})

	return NewSessionService(rdb, time.Hour)
}

func TestCreateAndGetByToken(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)

	created, err := svc.Create(ctx, Session{
		UserID: 1, Email: "sam@acme.com", Provider: "google",
		Groups: []string{"engineering"}, ClientIP: "172.30.100.5",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Token == "" {
		t.Fatal("expected a token")
	}
	if len(created.Token) != 64 {
		t.Errorf("token length = %d, want 64 hex chars", len(created.Token))
	}

	got, err := svc.Get(ctx, created.Token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Email != "sam@acme.com" {
		t.Errorf("Email = %q, want sam@acme.com", got.Email)
	}
	if len(got.Groups) != 1 || got.Groups[0] != "engineering" {
		t.Errorf("Groups = %v, want [engineering]", got.Groups)
	}
}

// The VPN only knows a client's tunnel IP, so lookup by IP must work.
func TestGetByClientIP(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)

	if _, err := svc.Create(ctx, Session{
		UserID: 1, Email: "sam@acme.com", ClientIP: "172.30.100.5",
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := svc.GetByClientIP(ctx, "172.30.100.5")
	if err != nil {
		t.Fatalf("GetByClientIP: %v", err)
	}
	if got.Email != "sam@acme.com" {
		t.Errorf("Email = %q, want sam@acme.com", got.Email)
	}
}

func TestUnknownSession(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)

	if _, err := svc.Get(ctx, "nonexistent"); !errors.Is(err, ErrNoSession) {
		t.Errorf("Get(unknown) error = %v, want ErrNoSession", err)
	}
	if _, err := svc.GetByClientIP(ctx, "10.0.0.1"); !errors.Is(err, ErrNoSession) {
		t.Errorf("GetByClientIP(unknown) error = %v, want ErrNoSession", err)
	}
}

// Revoking must clear both keys, or the VPN would keep granting access after
// the user logged out.
func TestRevokeClearsBothKeys(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)

	created, err := svc.Create(ctx, Session{
		UserID: 1, Email: "sam@acme.com", ClientIP: "172.30.100.5",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Revoke(ctx, created.Token); err != nil {
		t.Fatalf("Revoke: %v", err)
	}

	if _, err := svc.Get(ctx, created.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("after revoke, Get error = %v, want ErrNoSession", err)
	}
	if _, err := svc.GetByClientIP(ctx, "172.30.100.5"); !errors.Is(err, ErrNoSession) {
		t.Errorf("after revoke, GetByClientIP error = %v, want ErrNoSession", err)
	}
}

// This is what the client-disconnect hook calls.
func TestRevokeByClientIP(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)

	created, err := svc.Create(ctx, Session{
		UserID: 1, Email: "sam@acme.com", ClientIP: "172.30.100.9",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.RevokeByClientIP(ctx, "172.30.100.9"); err != nil {
		t.Fatalf("RevokeByClientIP: %v", err)
	}
	if _, err := svc.Get(ctx, created.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("session survived disconnect: %v", err)
	}

	// Disconnecting an unauthenticated client is not an error.
	if err := svc.RevokeByClientIP(ctx, "172.30.100.99"); err != nil {
		t.Errorf("RevokeByClientIP(unknown) = %v, want nil", err)
	}
}

// An expired session must not grant access even if the key still exists.
func TestExpiredSessionRejected(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)
	svc.ttl = 50 * time.Millisecond

	created, err := svc.Create(ctx, Session{UserID: 1, Email: "sam@acme.com"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if _, err := svc.Get(ctx, created.Token); !errors.Is(err, ErrNoSession) {
		t.Errorf("expired session was accepted: %v", err)
	}
}

// Tokens must not be predictable.
func TestTokensAreUnique(t *testing.T) {
	ctx := context.Background()
	svc := newTestSessions(t)

	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		s, err := svc.Create(ctx, Session{UserID: uint(i), Email: "x@acme.com"})
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		if seen[s.Token] {
			t.Fatalf("duplicate token generated: %s", s.Token)
		}
		seen[s.Token] = true
	}
}
