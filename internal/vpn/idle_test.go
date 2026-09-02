package vpn

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeTunnels stands in for OpenVPN's management interface.
type fakeTunnels struct {
	clients []ConnectedClient
	err     error
	killed  []int
	killErr error
}

func (f *fakeTunnels) Clients(context.Context) ([]ConnectedClient, error) {
	return f.clients, f.err
}

func (f *fakeTunnels) KillClient(_ context.Context, id int, _ string) error {
	if f.killErr != nil {
		return f.killErr
	}
	f.killed = append(f.killed, id)
	return nil
}

func connected(id int, addr string, ago time.Duration) ConnectedClient {
	return ConnectedClient{
		ClientID:       id,
		CommonName:     addr + "@example.com",
		VirtualAddress: addr,
		ConnectedSince: time.Now().Add(-ago),
	}
}

// signedInSet answers for a fixed set of addresses.
func signedInSet(addrs ...string) SignedIn {
	set := make(map[string]bool, len(addrs))
	for _, a := range addrs {
		set[a] = true
	}
	return func(_ context.Context, ip string) bool { return set[ip] }
}

func TestATunnelNobodySignedInOnIsClosed(t *testing.T) {
	f := &fakeTunnels{clients: []ConnectedClient{
		connected(1, "172.30.100.5", 40*time.Minute),
	}}

	s := NewIdleSweeper(f, signedInSet(), 30*time.Minute)
	closed, err := s.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if closed != 1 || len(f.killed) != 1 || f.killed[0] != 1 {
		t.Fatalf("closed %d, killed %v; want the one idle tunnel", closed, f.killed)
	}
}

// The whole point is that signing in keeps you connected. Closing a tunnel
// somebody is working over would be far worse than leaving an idle one open.
func TestASignedInTunnelIsLeftAlone(t *testing.T) {
	f := &fakeTunnels{clients: []ConnectedClient{
		connected(1, "172.30.100.5", 40*time.Minute),
		connected(2, "172.30.100.6", 9*time.Hour),
	}}

	s := NewIdleSweeper(f, signedInSet("172.30.100.5", "172.30.100.6"), 30*time.Minute)
	closed, err := s.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if closed != 0 || len(f.killed) != 0 {
		t.Fatalf("closed %d (%v); signed-in tunnels must survive any age", closed, f.killed)
	}
}

func TestAnUnidentifiedTunnelInsideTheWindowIsLeftAlone(t *testing.T) {
	f := &fakeTunnels{clients: []ConnectedClient{
		connected(1, "172.30.100.5", 5*time.Minute),
	}}

	s := NewIdleSweeper(f, signedInSet(), 30*time.Minute)
	closed, _ := s.Sweep(context.Background())
	if closed != 0 {
		t.Fatalf("closed a tunnel after 5 minutes of a 30 minute window")
	}
}

// One person may hold two tunnels: an abandoned one and the one they are
// using. Closing by common name would take both, which is why the sweep kills
// by connection id.
func TestOnlyTheAbandonedTunnelOfTwoIsClosed(t *testing.T) {
	idle := connected(1, "172.30.100.5", 40*time.Minute)
	working := connected(2, "172.30.100.6", 40*time.Minute)
	working.CommonName = idle.CommonName // the same person

	f := &fakeTunnels{clients: []ConnectedClient{idle, working}}

	s := NewIdleSweeper(f, signedInSet("172.30.100.6"), 30*time.Minute)
	closed, _ := s.Sweep(context.Background())

	if closed != 1 || len(f.killed) != 1 || f.killed[0] != 1 {
		t.Fatalf("closed %d, killed %v; want only the abandoned connection", closed, f.killed)
	}
}

// Without an address there is no session to look up, so there is no basis on
// which to judge the tunnel. Closing it anyway would be a guess.
func TestATunnelWithNoAddressIsNotJudged(t *testing.T) {
	c := connected(1, "", 40*time.Minute)
	f := &fakeTunnels{clients: []ConnectedClient{c}}

	s := NewIdleSweeper(f, signedInSet(), 30*time.Minute)
	closed, _ := s.Sweep(context.Background())
	if closed != 0 {
		t.Fatal("closed a tunnel it could not evaluate")
	}
}

// OpenVPN not running is the ordinary case on a machine that is only serving
// the portal, and must not stop the sweep from running again later.
func TestAnUnreachableVPNIsReportedNotFatal(t *testing.T) {
	f := &fakeTunnels{err: errors.New("connection refused")}

	s := NewIdleSweeper(f, signedInSet(), 30*time.Minute)
	if _, err := s.Sweep(context.Background()); err == nil {
		t.Fatal("expected the error to be reported")
	}
}

// A tunnel that closed between listing and killing is not a failure, and must
// not stop the rest of the sweep.
func TestAFailedKillDoesNotStopTheSweep(t *testing.T) {
	f := &fakeTunnels{
		clients: []ConnectedClient{
			connected(1, "172.30.100.5", 40*time.Minute),
			connected(2, "172.30.100.6", 40*time.Minute),
		},
		killErr: errors.New("no such client"),
	}

	s := NewIdleSweeper(f, signedInSet(), 30*time.Minute)
	closed, err := s.Sweep(context.Background())
	if err != nil {
		t.Fatalf("sweep returned an error: %v", err)
	}
	if closed != 0 {
		t.Fatalf("counted %d closed when every kill failed", closed)
	}
}

// A timeout of zero means the operator did not ask for this.
func TestNoTimeoutMeansNoSweeper(t *testing.T) {
	for _, d := range []time.Duration{0, -time.Minute} {
		if s := NewIdleSweeper(&fakeTunnels{}, signedInSet(), d); s != nil {
			t.Errorf("timeout %s produced a sweeper", d)
		}
	}
}

// A nil sweeper is what a deployment without the setting holds, and every
// method must tolerate it rather than crash the server at startup.
func TestANilSweeperIsSafe(t *testing.T) {
	var s *IdleSweeper
	if closed, err := s.Sweep(context.Background()); closed != 0 || err != nil {
		t.Fatalf("Sweep on nil returned %d, %v", closed, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	s.Run(ctx) // must return rather than panic
}

// The interval has to be short enough that the timeout means roughly what it
// says, and never so short that it hammers OpenVPN.
func TestTheCheckIntervalIsSensible(t *testing.T) {
	cases := []struct {
		timeout time.Duration
		want    time.Duration
	}{
		{30 * time.Minute, 5 * time.Minute},
		{6 * time.Minute, time.Minute},
		{time.Minute, 30 * time.Second}, // floor applies
		{10 * time.Second, 30 * time.Second},
	}

	for _, tc := range cases {
		s := NewIdleSweeper(&fakeTunnels{}, signedInSet(), tc.timeout)
		if s.every != tc.want {
			t.Errorf("timeout %s checks every %s, want %s", tc.timeout, s.every, tc.want)
		}
	}
}

func TestEachClosureIsReported(t *testing.T) {
	f := &fakeTunnels{clients: []ConnectedClient{
		connected(1, "172.30.100.5", 40*time.Minute),
	}}

	var seen []ConnectedClient
	s := NewIdleSweeper(f, signedInSet(), 30*time.Minute)
	s.OnReap = func(c ConnectedClient, idle time.Duration) {
		if idle < 30*time.Minute {
			t.Errorf("reported idle time of %s", idle)
		}
		seen = append(seen, c)
	}

	if _, err := s.Sweep(context.Background()); err != nil {
		t.Fatalf("sweep: %v", err)
	}
	if len(seen) != 1 || seen[0].VirtualAddress != "172.30.100.5" {
		t.Fatalf("reported %v, want the closed tunnel", seen)
	}
}
