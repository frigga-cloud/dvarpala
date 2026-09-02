package auth

import (
	"context"
	"errors"
	"testing"

	"dvarpala/internal/config"
	"dvarpala/internal/redis"
)

// captureMailer keeps the last code instead of sending it, so a test can act
// as the person reading their email.
type captureMailer struct {
	code string
	sent int
	err  error
}

func (m *captureMailer) SendCode(to, code string) error {
	if m.err != nil {
		return m.err
	}
	m.code = code
	m.sent++
	return nil
}

func newTestOTP(t *testing.T) (*OTPStore, *captureMailer) {
	t.Helper()

	rdb, err := redis.NewClient(config.RedisConfig{Addr: "localhost:6379", DB: 15})
	if err != nil {
		t.Skipf("redis not available on localhost:6379: %v", err)
	}
	t.Cleanup(func() {
		rdb.FlushDB(context.Background())
		rdb.Close()
	})

	mailer := &captureMailer{}
	return NewOTPStore(rdb, mailer), mailer
}

func TestGenerateCodeIsSixDigits(t *testing.T) {
	seen := make(map[string]bool)

	for i := 0; i < 200; i++ {
		code, err := generateCode()
		if err != nil {
			t.Fatalf("generateCode() error: %v", err)
		}
		if len(code) != otpLength {
			t.Fatalf("generateCode() = %q, want %d characters", code, otpLength)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("generateCode() = %q, want digits only", code)
			}
		}
		seen[code] = true
	}

	// Not a randomness test, just a guard against a constant.
	if len(seen) < 2 {
		t.Error("generateCode() returned the same value every time")
	}
}

func TestRequestThenVerifySucceeds(t *testing.T) {
	ctx := context.Background()
	store, mailer := newTestOTP(t)

	if err := store.Request(ctx, "sam@acme.com"); err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	if mailer.sent != 1 {
		t.Fatalf("mailer sent %d codes, want 1", mailer.sent)
	}

	if err := store.Verify(ctx, "sam@acme.com", mailer.code, "state-1"); err != nil {
		t.Fatalf("Verify() error: %v", err)
	}

	// The verification is recorded against the state, so Exchange can find it.
	info, err := NewOTPProvider(store).Exchange(ctx, "state-1")
	if err != nil {
		t.Fatalf("Exchange() error: %v", err)
	}
	if info.Email != "sam@acme.com" {
		t.Errorf("Exchange() email = %q, want %q", info.Email, "sam@acme.com")
	}
}

// A code proves who you are exactly once.
func TestCodeCannotBeReused(t *testing.T) {
	ctx := context.Background()
	store, mailer := newTestOTP(t)

	if err := store.Request(ctx, "sam@acme.com"); err != nil {
		t.Fatalf("Request() error: %v", err)
	}
	if err := store.Verify(ctx, "sam@acme.com", mailer.code, "state-1"); err != nil {
		t.Fatalf("first Verify() error: %v", err)
	}

	if err := store.Verify(ctx, "sam@acme.com", mailer.code, "state-2"); !errors.Is(err, ErrCodeExpired) {
		t.Errorf("second Verify() error = %v, want %v", err, ErrCodeExpired)
	}
}

// Knowing an address is not enough: without the code, the login attempt is
// never marked verified and Exchange refuses it.
func TestExchangeRefusesAnUnverifiedAttempt(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestOTP(t)

	if _, err := NewOTPProvider(store).Exchange(ctx, "state-nobody-verified"); !errors.Is(err, ErrNotVerified) {
		t.Errorf("Exchange() error = %v, want %v", err, ErrNotVerified)
	}
}

// A six-digit code is guessable if guessing is free, so it must not be.
func TestWrongCodesAreExhausted(t *testing.T) {
	ctx := context.Background()
	store, mailer := newTestOTP(t)

	if err := store.Request(ctx, "sam@acme.com"); err != nil {
		t.Fatalf("Request() error: %v", err)
	}

	for i := 1; i < otpMaxTries; i++ {
		if err := store.Verify(ctx, "sam@acme.com", "000000", "state-1"); !errors.Is(err, ErrCodeWrong) {
			t.Fatalf("attempt %d error = %v, want %v", i, err, ErrCodeWrong)
		}
	}

	if err := store.Verify(ctx, "sam@acme.com", "000000", "state-1"); !errors.Is(err, ErrCodeExhausted) {
		t.Fatalf("final attempt error = %v, want %v", err, ErrCodeExhausted)
	}

	// The real code died with the attempts, so it cannot be used afterwards.
	if err := store.Verify(ctx, "sam@acme.com", mailer.code, "state-1"); !errors.Is(err, ErrCodeExpired) {
		t.Errorf("after exhaustion error = %v, want %v", err, ErrCodeExpired)
	}
}

// A wrong-length guess must be wrong, not a crash. The reference
// implementation this was ported from uses Node's timingSafeEqual, which
// throws when the buffers differ in length.
func TestWrongLengthCodeIsRejectedNotFatal(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestOTP(t)

	if err := store.Request(ctx, "sam@acme.com"); err != nil {
		t.Fatalf("Request() error: %v", err)
	}

	if err := store.Verify(ctx, "sam@acme.com", "1", "state-1"); !errors.Is(err, ErrCodeWrong) {
		t.Errorf("Verify() error = %v, want %v", err, ErrCodeWrong)
	}
}

// Asking twice in quick succession is refused, so a mailbox cannot be flooded.
func TestSecondRequestIsRefusedDuringCooldown(t *testing.T) {
	ctx := context.Background()
	store, _ := newTestOTP(t)

	if err := store.Request(ctx, "sam@acme.com"); err != nil {
		t.Fatalf("first Request() error: %v", err)
	}
	if err := store.Request(ctx, "sam@acme.com"); !errors.Is(err, ErrTooSoon) {
		t.Errorf("second Request() error = %v, want %v", err, ErrTooSoon)
	}
}

// A code nobody receives leaves the person waiting, so delivery failure is
// reported rather than swallowed.
func TestDeliveryFailureIsReported(t *testing.T) {
	ctx := context.Background()
	store, mailer := newTestOTP(t)
	mailer.err = errors.New("smtp: connection refused")

	if err := store.Request(ctx, "sam@acme.com"); err == nil {
		t.Error("Request() returned nil, want the delivery error")
	}
}
