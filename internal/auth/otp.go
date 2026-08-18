package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"dvarpala/internal/redis"

	goredis "github.com/go-redis/redis/v8"
)

// Errors returned by the OTP flow.
var (
	ErrCodeExpired   = errors.New("that code has expired, or none was requested")
	ErrCodeWrong     = errors.New("that code is not right")
	ErrCodeExhausted = errors.New("too many wrong attempts; request a new code")
	ErrTooSoon       = errors.New("a code was just sent; wait before asking for another")
	ErrDailyLimit    = errors.New("too many codes requested today")
	ErrNotVerified   = errors.New("no verified code for this login attempt")
)

// OTP policy. Deliberately strict: with codes as the only way in, these
// limits are the whole defence against someone guessing a six-digit number.
const (
	otpLength     = 6
	otpTTL        = 5 * time.Minute
	otpCooldown   = 60 * time.Second
	otpDailyLimit = 10
	otpMaxTries   = 5

	// verifiedTTL is how long a verified attempt may wait to be exchanged.
	// Short, because by then the person is mid-redirect.
	verifiedTTL = 2 * time.Minute
)

// OTPStore issues and checks one-time sign-in codes, held in Redis.
type OTPStore struct {
	rdb    *redis.Client
	mailer Mailer
}

// NewOTPStore creates the store.
func NewOTPStore(rdb *redis.Client, mailer Mailer) *OTPStore {
	return &OTPStore{rdb: rdb, mailer: mailer}
}

func otpKey(email string) string         { return "otp:" + email }
func otpCooldownKey(email string) string { return "otp_cooldown:" + email }
func otpDailyKey(email string) string    { return "otp_daily:" + email }
func otpAttemptsKey(email string) string { return "otp_attempts:" + email }
func otpVerifiedKey(state string) string { return "otp_verified:" + state }

// generateCode returns a random code of otpLength digits.
//
// crypto/rand, not math/rand: this is a credential, and a predictable one is
// no credential at all.
func generateCode() (string, error) {
	max := big.NewInt(1)
	for i := 0; i < otpLength; i++ {
		max.Mul(max, big.NewInt(10))
	}

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generating code: %w", err)
	}
	return fmt.Sprintf("%0*d", otpLength, n), nil
}

// Request issues a code for an address and sends it.
//
// The caller is responsible for having established that this address may sign
// in at all - the domain allow-list and the database check both happen before
// we are willing to send anything.
func (s *OTPStore) Request(ctx context.Context, email string) error {
	email = strings.ToLower(strings.TrimSpace(email))

	// One code per minute, so the mailbox cannot be used as a weapon and a
	// guesser cannot refresh their way through the keyspace.
	if ttl, err := s.rdb.TTL(ctx, otpCooldownKey(email)).Result(); err == nil && ttl > 0 {
		return fmt.Errorf("%w (%d seconds)", ErrTooSoon, int(ttl.Seconds()))
	}

	if count, err := s.rdb.Get(ctx, otpDailyKey(email)).Int(); err == nil && count >= otpDailyLimit {
		return ErrDailyLimit
	}

	code, err := generateCode()
	if err != nil {
		return err
	}

	pipe := s.rdb.TxPipeline()
	pipe.Set(ctx, otpKey(email), code, otpTTL)
	pipe.Set(ctx, otpCooldownKey(email), "1", otpCooldown)
	pipe.Incr(ctx, otpDailyKey(email))
	// Always (re)set the expiry inside the same transaction, so the daily
	// counter can never be left without one and lock somebody out forever.
	pipe.Expire(ctx, otpDailyKey(email), 24*time.Hour)
	// A new code invalidates the attempts spent on the previous one.
	pipe.Del(ctx, otpAttemptsKey(email))
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("storing code: %w", err)
	}

	// Sent last, and its failure is returned: a code nobody receives is worse
	// than no code at all, because the person waits instead of asking for help.
	return s.mailer.SendCode(email, code)
}

// Verify checks a code and, if it is right, marks the login attempt verified.
//
// The verification is recorded against the state token rather than handed back
// to the browser. That way the only thing travelling through the redirect is an
// opaque token that had to be earned - knowing somebody's address is not enough
// to complete a login as them.
func (s *OTPStore) Verify(ctx context.Context, email, code, state string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	code = strings.TrimSpace(code)

	stored, err := s.rdb.Get(ctx, otpKey(email)).Result()
	if errors.Is(err, goredis.Nil) {
		return ErrCodeExpired
	}
	if err != nil {
		return fmt.Errorf("reading code: %w", err)
	}

	// Constant-time, and safe on a length mismatch - it returns 0 rather than
	// failing, so a wrong-length guess is simply wrong.
	if subtle.ConstantTimeCompare([]byte(stored), []byte(code)) != 1 {
		attempts, err := s.rdb.Incr(ctx, otpAttemptsKey(email)).Result()
		if err == nil && attempts == 1 {
			s.rdb.Expire(ctx, otpAttemptsKey(email), otpTTL)
		}
		if attempts >= otpMaxTries {
			s.rdb.Del(ctx, otpKey(email), otpAttemptsKey(email))
			return ErrCodeExhausted
		}
		return ErrCodeWrong
	}

	// Correct: spend the code, and remember that this attempt earned it.
	pipe := s.rdb.TxPipeline()
	pipe.Del(ctx, otpKey(email), otpAttemptsKey(email))
	pipe.Set(ctx, otpVerifiedKey(state), email, verifiedTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("recording verification: %w", err)
	}
	return nil
}

// OTPProvider presents the code flow as an ordinary login provider, so the
// state check, domain allow-list, database authorisation, session creation and
// audit record all run exactly as they do for any other provider.
type OTPProvider struct {
	store *OTPStore
}

// NewOTPProvider creates the provider.
func NewOTPProvider(store *OTPStore) *OTPProvider { return &OTPProvider{store: store} }

// Name implements Provider.
func (p *OTPProvider) Name() string { return "otp" }

// DisplayName implements Provider.
func (p *OTPProvider) DisplayName() string { return "Email me a code" }

// AuthURL sends the browser to the local form that asks for an address.
// Relative, for the same reason as the development provider: any hostname we
// could name here is wrong for somebody.
func (p *OTPProvider) AuthURL(state string) string {
	return fmt.Sprintf("/otp/login?state=%s", url.QueryEscape(state))
}

// Exchange turns a verified login attempt into an identity.
//
// The code handed back is the state token, not an email address. Only an
// attempt that presented the right code has a verification recorded against
// it, and consuming it here makes it single-use.
func (p *OTPProvider) Exchange(ctx context.Context, code string) (*UserInfo, error) {
	email, err := p.store.rdb.Get(ctx, otpVerifiedKey(code)).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, ErrNotVerified
	}
	if err != nil {
		return nil, fmt.Errorf("reading verification: %w", err)
	}
	p.store.rdb.Del(ctx, otpVerifiedKey(code))

	return &UserInfo{Email: email, Name: email, Provider: p.Name()}, nil
}
