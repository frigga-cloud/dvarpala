package auth

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a mail server that speaks just enough SMTP to be talked to.
//
// The mailer is the one piece of Dvarpala whose whole job is a conversation
// with someone else's software, so the only tests worth writing are ones that
// actually hold that conversation.
type fakeSMTP struct {
	t *testing.T

	// rejectRcpt makes the server refuse the recipient, as a real one does
	// for an address it does not host.
	rejectRcpt bool

	// offerStartTLS advertises STARTTLS. The fake cannot complete it, so
	// tests that set this only check what the client does about the offer.
	offerStartTLS bool

	// silent accepts the connection and then says nothing at all.
	silent bool

	mu       sync.Mutex
	received string
	authSeen bool
}

func (f *fakeSMTP) start() (addr string, stop func()) {
	f.t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		f.t.Fatalf("listening: %v", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go f.serve(conn)
		}
	}()

	return ln.Addr().String(), func() { ln.Close(); <-done }
}

func (f *fakeSMTP) serve(conn net.Conn) {
	defer conn.Close()

	if f.silent {
		// Hold the connection open without a greeting. A client with no
		// deadline waits here forever.
		time.Sleep(90 * time.Second)
		return
	}

	_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	r := bufio.NewReader(conn)
	say := func(s string) { fmt.Fprintf(conn, "%s\r\n", s) }

	say("220 fake.test ESMTP")

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.TrimSpace(line))

		switch {
		case strings.HasPrefix(cmd, "EHLO"):
			say("250-fake.test")
			if f.offerStartTLS {
				say("250-STARTTLS")
			}
			say("250 AUTH PLAIN")

		case strings.HasPrefix(cmd, "HELO"):
			say("250 fake.test")

		case strings.HasPrefix(cmd, "STARTTLS"):
			// Advertised but not implemented: the client should fail here
			// rather than continue in the clear.
			say("454 TLS not available")

		case strings.HasPrefix(cmd, "AUTH"):
			f.mu.Lock()
			f.authSeen = true
			f.mu.Unlock()
			say("235 accepted")

		case strings.HasPrefix(cmd, "MAIL FROM"):
			say("250 ok")

		case strings.HasPrefix(cmd, "RCPT TO"):
			if f.rejectRcpt {
				say("550 no such user here")
				continue
			}
			say("250 ok")

		case strings.HasPrefix(cmd, "DATA"):
			say("354 go ahead")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(l, "\r\n") == "." {
					break
				}
				b.WriteString(l)
			}
			f.mu.Lock()
			f.received = b.String()
			f.mu.Unlock()
			say("250 queued")

		case strings.HasPrefix(cmd, "QUIT"):
			say("221 bye")
			return

		default:
			say("500 unrecognised")
		}
	}
}

func (f *fakeSMTP) message() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.received
}

func mailerFor(addr string) *SMTPMailer {
	host, port, _ := net.SplitHostPort(addr)
	p := 0
	fmt.Sscanf(port, "%d", &p)
	return &SMTPMailer{Host: host, Port: p, From: "noreply@example.com", FromName: "Dvarpala"}
}

func TestSendCodeProducesADeliverableMessage(t *testing.T) {
	srv := &fakeSMTP{t: t}
	addr, stop := srv.start()
	defer stop()

	if err := mailerFor(addr).SendCode("sam@example.com", "481920"); err != nil {
		t.Fatalf("sending: %v", err)
	}

	msg := srv.message()
	for _, want := range []string{
		"From: Dvarpala <noreply@example.com>",
		"To: sam@example.com",
		"Subject: Your Dvarpala sign-in code",
		"Date: ",
		"Message-ID: <",
		"@example.com>",
		"Content-Type: text/plain; charset=utf-8",
		"481920",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message is missing %q\n---\n%s", want, msg)
		}
	}
}

// A mail server that accepts the connection and then goes quiet must not hold
// the person's browser open until it gives up.
func TestASilentServerFailsRatherThanHanging(t *testing.T) {
	srv := &fakeSMTP{t: t, silent: true}
	addr, stop := srv.start()
	defer stop()

	m := mailerFor(addr)
	m.Timeout = 2 * time.Second

	done := make(chan error, 1)
	go func() { done <- m.SendCode("sam@example.com", "481920") }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from a server that never answered")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("SendCode did not return: a hung mail server would hang the portal")
	}
}

// Credentials must never cross an unencrypted connection to another machine.
func TestCredentialsAreNotSentInClearText(t *testing.T) {
	srv := &fakeSMTP{t: t} // no STARTTLS offered
	addr, stop := srv.start()
	defer stop()

	_, port, _ := net.SplitHostPort(addr)
	p := 0
	fmt.Sscanf(port, "%d", &p)

	// A non-loopback hostname that still resolves to the test server.
	m := &SMTPMailer{
		Host: "localtest.me", Port: p,
		Username: "noreply@example.com", Password: "hunter2",
		From: "noreply@example.com",
	}

	err := m.SendCode("sam@example.com", "481920")
	if err == nil {
		t.Fatal("expected a refusal to authenticate in the clear")
	}
	if !strings.Contains(err.Error(), "clear text") {
		t.Fatalf("unexpected error: %v", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if srv.authSeen {
		t.Fatal("credentials were sent over an unencrypted connection")
	}
}

// The same server on loopback is fine: nothing leaves the machine.
func TestCredentialsMayBeSentOverLoopback(t *testing.T) {
	srv := &fakeSMTP{t: t}
	addr, stop := srv.start()
	defer stop()

	m := mailerFor(addr)
	m.Username = "noreply@example.com"
	m.Password = "hunter2"

	if err := m.SendCode("sam@example.com", "481920"); err != nil {
		t.Fatalf("sending over loopback: %v", err)
	}
	srv.mu.Lock()
	defer srv.mu.Unlock()
	if !srv.authSeen {
		t.Fatal("expected the client to authenticate")
	}
}

// A rejection has to name what was rejected, or nobody can act on it.
func TestARejectedRecipientIsReportedUsefully(t *testing.T) {
	srv := &fakeSMTP{t: t, rejectRcpt: true}
	addr, stop := srv.start()
	defer stop()

	err := mailerFor(addr).SendCode("nobody@example.com", "481920")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"nobody@example.com", "no such user"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

// An advertised STARTTLS that then fails must stop the send, not fall back.
func TestAFailedUpgradeStopsTheSend(t *testing.T) {
	srv := &fakeSMTP{t: t, offerStartTLS: true}
	addr, stop := srv.start()
	defer stop()

	err := mailerFor(addr).SendCode("sam@example.com", "481920")
	if err == nil {
		t.Fatal("expected an error when the upgrade failed")
	}
	if !strings.Contains(err.Error(), "securing connection") {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.message() != "" {
		t.Fatal("the message was sent anyway")
	}
}

func TestHeaderInjectionIsRefused(t *testing.T) {
	m := &SMTPMailer{Host: "127.0.0.1", Port: 25, From: "noreply@example.com"}

	err := m.SendCode("sam@example.com\r\nBcc: everyone@example.com", "481920")
	if err == nil {
		t.Fatal("expected a refusal")
	}
	if !strings.Contains(err.Error(), "line break") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMissingConfigurationIsNamed(t *testing.T) {
	cases := []struct {
		name   string
		mailer *SMTPMailer
		want   string
	}{
		{"no from", &SMTPMailer{Host: "mail.example.com"}, "from address"},
		{"no host", &SMTPMailer{From: "noreply@example.com"}, "mail server"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mailer.SendCode("sam@example.com", "481920")
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want an error mentioning %q, got %v", tc.want, err)
			}
		})
	}
}

// A large mail service is many machines behind one name, and an individual
// one can be transiently unreachable. Observed against Gmail: a connection
// that timed out succeeded moments later. Somebody signing in should not have
// to notice that.
func TestAConnectionIsRetried(t *testing.T) {
	// A listener that refuses the first connections, then accepts.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close() // nothing is listening yet: connections are refused

	srv := &fakeSMTP{t: t}
	var reopen sync.Once
	go func() {
		// Bring the real server up on the same address a moment later, as a
		// rotating endpoint becoming reachable again.
		time.Sleep(200 * time.Millisecond)
		reopen.Do(func() {
			l, err := net.Listen("tcp", addr)
			if err != nil {
				return
			}
			t.Cleanup(func() { l.Close() })
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				go srv.serve(conn)
			}
		})
	}()

	m := mailerFor(addr)
	m.Timeout = 5 * time.Second

	if err := m.SendCode("sam@example.com", "481920"); err != nil {
		t.Fatalf("a transient connection failure was not retried: %v", err)
	}
	if !strings.Contains(srv.message(), "481920") {
		t.Error("the message did not arrive after the retry")
	}
}

// Retrying must not turn a genuine outage into a long wait.
func TestRetryingStaysWithinTheOverallBudget(t *testing.T) {
	// Port 1 on loopback: nothing listens, connections are refused at once.
	m := &SMTPMailer{Host: "127.0.0.1", Port: 1, From: "noreply@example.com"}
	m.Timeout = 3 * time.Second

	started := time.Now()
	err := m.SendCode("sam@example.com", "481920")
	took := time.Since(started)

	if err == nil {
		t.Fatal("expected an error")
	}
	if took > m.Timeout+2*time.Second {
		t.Fatalf("took %s, which is beyond the %s budget", took, m.Timeout)
	}
}
