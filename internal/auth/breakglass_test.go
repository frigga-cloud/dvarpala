package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"dvarpala/internal/config"
	"dvarpala/internal/database/models"
	"dvarpala/internal/redis"
	"dvarpala/internal/services"
)

// Break-glass is the path nobody exercises until the day everything else is
// broken, which is the worst possible time to discover it does not work.

func TestBreakGlassIssuesAWorkingSession(t *testing.T) {
	h := newBreakGlassHarness(t)

	grant, err := h.bg.Issue(context.Background(), h.email, "", "mail server down", 0)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	if grant.Code == "" {
		t.Fatal("no redemption code")
	}
	if grant.Expires.After(time.Now().Add(breakGlassDefault + time.Minute)) {
		t.Fatalf("session outlives the default window: %s", grant.Expires)
	}

	sess, err := h.bg.Redeem(context.Background(), grant.Code)
	if err != nil {
		t.Fatalf("redeeming: %v", err)
	}
	if sess.Email != h.email {
		t.Errorf("session belongs to %q, want %q", sess.Email, h.email)
	}
	// Named in the session, so everything the session later does is traceable
	// to how it was obtained.
	if sess.Provider != "break-glass" {
		t.Errorf("provider is %q, want %q", sess.Provider, "break-glass")
	}
}

func TestARedemptionCodeWorksOnlyOnce(t *testing.T) {
	h := newBreakGlassHarness(t)

	grant, err := h.bg.Issue(context.Background(), h.email, "", "mail server down", 0)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}

	if _, err := h.bg.Redeem(context.Background(), grant.Code); err != nil {
		t.Fatalf("first redemption: %v", err)
	}

	// The code travels in a URL, and URLs end up in access logs. A second use
	// must fail, so a copy recovered from a log later is worthless.
	if _, err := h.bg.Redeem(context.Background(), grant.Code); err != ErrNoBreakGlass {
		t.Fatalf("second redemption returned %v, want %v", err, ErrNoBreakGlass)
	}
}

func TestAnUnknownCodeIsRefused(t *testing.T) {
	h := newBreakGlassHarness(t)

	for _, code := range []string{"", "deadbeef", strings.Repeat("a", 64)} {
		if _, err := h.bg.Redeem(context.Background(), code); err != ErrNoBreakGlass {
			t.Errorf("code %q returned %v, want %v", code, err, ErrNoBreakGlass)
		}
	}
}

// Break-glass gets past a broken mail server, not past a decision somebody
// made about an account.
func TestBreakGlassRefusesAnAccountThatMayNotSignIn(t *testing.T) {
	h := newBreakGlassHarness(t)

	cases := []struct {
		name  string
		email string
	}{
		{"never existed", "nobody@example.com"},
		{"deactivated", h.inactiveEmail},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := h.bg.Issue(context.Background(), tc.email, "", "reason", 0); err == nil {
				t.Fatal("expected a refusal")
			}
		})
	}
}

func TestBreakGlassRequiresAReasonAndABoundedWindow(t *testing.T) {
	h := newBreakGlassHarness(t)
	ctx := context.Background()

	if _, err := h.bg.Issue(ctx, h.email, "", "", 0); err == nil {
		t.Error("expected a refusal with no reason given")
	}

	if _, err := h.bg.Issue(ctx, h.email, "", "reason", breakGlassMax+time.Minute); err == nil {
		t.Error("expected a refusal for a window beyond the maximum")
	}
}

// Binding the tunnel address is what turns console access into network
// access, so it has to actually reach the lookup the VPN performs.
func TestBindingATunnelAddressOpensTheNetworkToo(t *testing.T) {
	h := newBreakGlassHarness(t)
	const tunnelIP = "172.30.100.77"

	grant, err := h.bg.Issue(context.Background(), h.email, tunnelIP, "mail server down", 0)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	if _, err := h.bg.Redeem(context.Background(), grant.Code); err != nil {
		t.Fatalf("redeeming: %v", err)
	}

	// This is the lookup the OpenVPN connect hook makes.
	sess, err := h.sessions.GetByClientIP(context.Background(), tunnelIP)
	if err != nil {
		t.Fatalf("the VPN cannot find the session: %v", err)
	}
	if sess.Email != h.email {
		t.Errorf("tunnel session belongs to %q, want %q", sess.Email, h.email)
	}
}

// Without an address, only the browser is signed in.
func TestWithoutATunnelAddressTheNetworkStaysClosed(t *testing.T) {
	h := newBreakGlassHarness(t)

	grant, err := h.bg.Issue(context.Background(), h.email, "", "mail server down", 0)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	if _, err := h.bg.Redeem(context.Background(), grant.Code); err != nil {
		t.Fatalf("redeeming: %v", err)
	}

	if _, err := h.sessions.GetByClientIP(context.Background(), "172.30.100.78"); err == nil {
		t.Fatal("an unbound emergency session opened the network")
	}
}

// --- harness ---------------------------------------------------------------

// fakeUsers answers the one question break-glass asks of the database.
type fakeUsers struct {
	active   map[string]*models.User
	inactive map[string]bool
}

func (f *fakeUsers) IsAuthorised(_ context.Context, email string) (*models.User, error) {
	if f.inactive[email] {
		return nil, fmt.Errorf("user is not active: %s is inactive", email)
	}
	u, ok := f.active[email]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", email)
	}
	return u, nil
}

// recordingAudit keeps entries so a test can assert what was written down.
type recordingAudit struct{ entries []services.Entry }

func (r *recordingAudit) Log(_ context.Context, e services.Entry) {
	r.entries = append(r.entries, e)
}

func (r *recordingAudit) actions() []string {
	out := make([]string, 0, len(r.entries))
	for _, e := range r.entries {
		out = append(out, e.Action)
	}
	return out
}

type breakGlassHarness struct {
	bg            *BreakGlass
	sessions      *SessionService
	audit         *recordingAudit
	email         string
	inactiveEmail string
}

func newBreakGlassHarness(t *testing.T) *breakGlassHarness {
	t.Helper()

	rdb, err := redis.NewClient(config.RedisConfig{Addr: "localhost:6379", DB: 15})
	if err != nil {
		t.Skipf("redis not available on localhost:6379: %v", err)
	}
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
	})

	const email = "admin@example.com"
	users := &fakeUsers{
		active: map[string]*models.User{
			email: {
				ID:     1,
				Email:  email,
				Status: models.UserStatusActive,
				Groups: []models.Group{{Name: "system_admins"}},
			},
		},
		inactive: map[string]bool{"gone@example.com": true},
	}

	audit := &recordingAudit{}
	sessions := NewSessionService(rdb, 8*time.Hour)

	return &breakGlassHarness{
		bg:            &BreakGlass{rdb: rdb, sessions: sessions, users: users, audit: audit},
		sessions:      sessions,
		audit:         audit,
		email:         email,
		inactiveEmail: "gone@example.com",
	}
}

// Emergency access with no record of it is indistinguishable, months later,
// from an intrusion.
func TestEveryOutcomeIsRecorded(t *testing.T) {
	h := newBreakGlassHarness(t)
	ctx := context.Background()

	grant, err := h.bg.Issue(ctx, h.email, "", "mail server down", 0)
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	if _, err := h.bg.Redeem(ctx, grant.Code); err != nil {
		t.Fatalf("redeeming: %v", err)
	}
	if _, err := h.bg.Issue(ctx, h.inactiveEmail, "", "trying it on", 0); err == nil {
		t.Fatal("expected a refusal")
	}

	want := []string{"breakglass_issued", "breakglass_redeemed", "breakglass_refused"}
	got := h.audit.actions()
	if len(got) != len(want) {
		t.Fatalf("recorded %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("record %d is %q, want %q", i, got[i], want[i])
		}
	}

	// The reason is the whole point of demanding one.
	if r, _ := h.audit.entries[0].Details["reason"].(string); r != "mail server down" {
		t.Errorf("the stated reason was not recorded, got %q", r)
	}
}
