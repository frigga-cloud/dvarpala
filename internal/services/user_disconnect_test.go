package services

import (
	"context"
	"errors"
	"testing"

	"dvarpala/internal/config"
	"dvarpala/internal/database"

	"gorm.io/gorm"
)

// fakeDisconnector records what it was asked to close, and can refuse.
type fakeDisconnector struct {
	killed []string
	err    error
	n      int
}

func (f *fakeDisconnector) Kill(_ context.Context, commonName string) (int, error) {
	f.killed = append(f.killed, commonName)
	return f.n, f.err
}

// newTestUsers connects to the development database, skipping when there is
// none, in the same way the session tests skip without Redis.
func newTestUsers(t *testing.T) (*UserService, *gorm.DB) {
	t.Helper()

	db, err := database.NewConnection(config.DatabaseConfig{
		Host: "localhost", Port: 5432, Name: "dvarpala_dev",
		User: "dvarpala", Password: "dvarpala_password", SSLMode: "disable",
	})
	if err != nil {
		t.Skipf("postgres not available: %v", err)
	}

	return NewUserService(db.DB, NewAuditService(db.DB)), db.DB
}

// makeUser creates somebody to deactivate, and removes them afterwards.
func makeUser(t *testing.T, s *UserService, db *gorm.DB, email string) {
	t.Helper()
	ctx := context.Background()

	if _, err := s.CreateUser(ctx, CreateUserRequest{Email: email, FullName: "Test"}); err != nil {
		t.Skipf("could not create a test user: %v", err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM audit_logs WHERE details::text LIKE ?", "%"+email+"%")
		db.Exec("DELETE FROM users WHERE email = ?", email)
	})
}

// Deactivating somebody must end the tunnel they are already holding. Without
// this, revocation only takes effect at their next connection, which for a
// VPN is minutes away at best.
func TestDeactivateClosesTheirLiveTunnel(t *testing.T) {
	svc, db := newTestUsers(t)
	const email = "disconnect-test@acme.com"
	makeUser(t, svc, db, email)

	killer := &fakeDisconnector{n: 1}
	svc.EnableDisconnect(killer)

	if err := svc.DeactivateUser(context.Background(), email); err != nil {
		t.Fatalf("DeactivateUser() error: %v", err)
	}

	if len(killer.killed) != 1 || killer.killed[0] != email {
		t.Fatalf("killed %v, want exactly [%s]", killer.killed, email)
	}
}

// The account must still be deactivated when the VPN cannot be reached.
// Reporting failure here would say the person still has access when they do
// not, which is the more dangerous of the two wrong answers.
func TestDeactivateSucceedsWhenTheVPNIsUnreachable(t *testing.T) {
	svc, db := newTestUsers(t)
	const email = "disconnect-fail@acme.com"
	makeUser(t, svc, db, email)

	svc.EnableDisconnect(&fakeDisconnector{err: errors.New("management interface unreachable")})

	if err := svc.DeactivateUser(context.Background(), email); err != nil {
		t.Fatalf("DeactivateUser() error = %v, want nil", err)
	}

	if _, err := svc.IsAuthorised(context.Background(), email); err == nil {
		t.Error("IsAuthorised() allowed a deactivated user")
	}
}

// A deployment whose VPN offers no control channel behaves as it did before.
func TestDeactivateWorksWithNoDisconnector(t *testing.T) {
	svc, db := newTestUsers(t)
	const email = "disconnect-none@acme.com"
	makeUser(t, svc, db, email)

	if err := svc.DeactivateUser(context.Background(), email); err != nil {
		t.Fatalf("DeactivateUser() error: %v", err)
	}
}
