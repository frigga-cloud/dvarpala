package vpn

import (
	"context"
	"log"
	"time"
)

// SignedIn reports whether a tunnel address belongs to somebody who has
// identified themselves.
//
// A function rather than an interface on the session store, because the
// services package already depends on this one and the dependency cannot run
// both ways.
type SignedIn func(ctx context.Context, clientIP string) bool

// tunnels is the part of the management interface a sweep needs.
type tunnels interface {
	Clients(ctx context.Context) ([]ConnectedClient, error)
	KillClient(ctx context.Context, clientID int, message string) error
}

// IdleSweeper closes tunnels that were opened and never signed in on.
//
// Holding a certificate is enough to open a tunnel; it is signing in that
// earns any access. Nothing until now ended the gap between the two, so a
// client could sit unidentified for as long as it liked. What it could reach
// was almost nothing - the sign-in page and the addresses needed to load it -
// so this is not an unauthorised-access hole. It is an abandoned tunnel
// holding an address and a slot in the server's client table indefinitely.
//
// The timeout has been configurable as auth.captive_portal_timeout since
// before this existed, and nothing read it. A setting that does nothing is
// worse than no setting, because it is read as a guarantee.
type IdleSweeper struct {
	tunnels  tunnels
	signedIn SignedIn

	// after is how long a client may stay unidentified.
	after time.Duration

	// every is how often to look.
	every time.Duration

	// OnReap is called for each tunnel closed, so the caller can record it.
	// Optional.
	OnReap func(client ConnectedClient, idle time.Duration)
}

// NewIdleSweeper creates the sweeper. A timeout of zero or less disables it,
// which NewIdleSweeper reports by returning nil.
func NewIdleSweeper(t tunnels, signedIn SignedIn, after time.Duration) *IdleSweeper {
	if after <= 0 {
		return nil
	}

	// Check often enough that the timeout means roughly what it says, without
	// asking OpenVPN for its client list every few seconds.
	every := after / 6
	if every < 30*time.Second {
		every = 30 * time.Second
	}

	return &IdleSweeper{tunnels: t, signedIn: signedIn, after: after, every: every}
}

// Run sweeps until the context is cancelled.
func (s *IdleSweeper) Run(ctx context.Context) {
	if s == nil {
		return
	}
	log.Printf("closing tunnels left unidentified for %s, checked every %s",
		s.after.Round(time.Second), s.every.Round(time.Second))

	ticker := time.NewTicker(s.every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Sweep(ctx); err != nil {
				// Expected whenever OpenVPN is not running, or its management
				// interface is not enabled. Not worth stopping over: the next
				// tick tries again.
				log.Printf("idle sweep: %v", err)
			}
		}
	}
}

// Sweep makes one pass and reports how many tunnels it closed.
func (s *IdleSweeper) Sweep(ctx context.Context) (int, error) {
	if s == nil {
		return 0, nil
	}

	clients, err := s.tunnels.Clients(ctx)
	if err != nil {
		return 0, err
	}

	closed := 0
	for _, c := range clients {
		// No tunnel address means nothing to look the session up by. Leave it
		// alone: closing a tunnel we could not evaluate is worse than leaving
		// one open that we could not judge.
		if c.VirtualAddress == "" {
			continue
		}
		if s.signedIn(ctx, c.VirtualAddress) {
			continue
		}

		idle := time.Since(c.ConnectedSince)
		if idle < s.after {
			continue
		}

		if err := s.tunnels.KillClient(ctx, c.ClientID, "HALT"); err != nil {
			log.Printf("idle sweep: could not close %s (%s): %v",
				c.VirtualAddress, c.CommonName, err)
			continue
		}

		closed++
		if s.OnReap != nil {
			s.OnReap(c, idle)
		}
	}

	return closed, nil
}
