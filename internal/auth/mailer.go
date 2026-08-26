package auth

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Mailer delivers a sign-in code to a person.
type Mailer interface {
	SendCode(to, code string) error
}

// mailTimeout bounds the whole conversation with the mail server.
//
// Somebody is waiting on a web page while this runs. A mail server that
// accepts the connection and then stops answering would otherwise hold the
// request open until the browser gave up, with no explanation on either side.
const mailTimeout = 20 * time.Second

// Connecting is retried, briefly, because a large mail service is many
// machines behind one name and an individual one can be transiently
// unreachable. Observed against Gmail: a connection that timed out succeeded
// on the next attempt a moment later.
//
// Only the connection is retried. A server that answered and then refused has
// given a verdict, and asking again would not change it.
const (
	dialTimeout = 8 * time.Second
	dialTries   = 3

	// dialPause separates attempts. Without it the retries land in the same
	// instant as the failure and test nothing new - a refused connection
	// fails immediately, so three attempts would finish in microseconds and
	// hit whatever was briefly wrong all three times.
	dialPause = 300 * time.Millisecond
)

// SMTPMailer sends codes through a mail server.
//
// Any server will do - a provider such as Google Workspace, or a relay on the
// machine itself. Only the address changes.
type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string

	// Timeout bounds the whole conversation. Zero means mailTimeout.
	Timeout time.Duration

	// Now is the clock, overridable in tests.
	Now func() time.Time
}

// timeout is the configured bound, or the default.
func (m *SMTPMailer) timeout() time.Duration {
	if m.Timeout > 0 {
		return m.Timeout
	}
	return mailTimeout
}

// SendCode delivers the code, or reports why it could not.
//
// A failure here must reach the caller: with codes as the only way in, a mail
// server that silently swallows messages locks every user out of the network
// with no indication of why.
func (m *SMTPMailer) SendCode(to, code string) error {
	if m.From == "" {
		return errors.New("smtp: no from address configured")
	}
	if m.Host == "" {
		return errors.New("smtp: no mail server configured")
	}
	if err := headerSafe(to); err != nil {
		return fmt.Errorf("smtp: recipient address: %w", err)
	}

	msg, err := m.compose(to, code)
	if err != nil {
		return err
	}

	return m.send(to, msg)
}

// send runs the SMTP conversation.
//
// net/smtp's SendMail is not used, for two reasons. It cannot dial a server
// that expects TLS from the first byte, which is what port 465 means and what
// several providers still offer. And it dials with no deadline at all, so a
// server that never answers would hang the person's browser rather than
// failing.
func (m *SMTPMailer) send(to string, msg []byte) error {
	port := m.Port
	if port == 0 {
		port = 587
	}
	addr := net.JoinHostPort(m.Host, fmt.Sprint(port))

	conn, err := m.dialWithRetry(addr, port)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	// A deadline on the connection covers every later step in one place, so no
	// individual command can stall.
	_ = conn.SetDeadline(time.Now().Add(m.timeout()))
	defer conn.Close()

	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		return fmt.Errorf("starting session with %s: %w", addr, err)
	}
	defer c.Close()

	// Upgrade a plain connection if the server offers it. Not optional when
	// there are credentials to send: a password in clear text on the wire is
	// worse than not sending mail at all.
	if _, isTLS := conn.(*tls.Conn); !isTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
				return fmt.Errorf("securing connection to %s: %w", addr, err)
			}
		} else if m.Username != "" && !isLoopback(m.Host) {
			return fmt.Errorf("refusing to send credentials to %s in clear text: "+
				"the server does not offer STARTTLS", addr)
		}
	}

	if m.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", m.Username, m.Password, m.Host)); err != nil {
			return fmt.Errorf("signing in to %s as %s: %w", addr, m.Username, err)
		}
	}

	if err := c.Mail(m.From); err != nil {
		return fmt.Errorf("server rejected the sender %s: %w", m.From, err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("server rejected the recipient %s: %w", to, err)
	}

	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("server refused the message: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("writing the message: %w", err)
	}
	if err := w.Close(); err != nil {
		// The server's verdict on the finished message arrives here, so this
		// is where a rejection for content or reputation shows up.
		return fmt.Errorf("server did not accept the message: %w", err)
	}

	return c.Quit()
}

// dialWithRetry connects, trying again on a failure to reach the server.
//
// Each attempt is short and the whole loop stays inside the overall budget,
// so this makes a transient failure recoverable without making a genuine
// outage take any longer to report.
func (m *SMTPMailer) dialWithRetry(addr string, port int) (net.Conn, error) {
	deadline := time.Now().Add(m.timeout())

	var err error
	for attempt := 1; attempt <= dialTries; attempt++ {
		if attempt > 1 {
			if time.Until(deadline) <= dialPause {
				break
			}
			time.Sleep(dialPause)
		}

		var conn net.Conn
		conn, err = m.dial(addr, port, m.attemptTimeout(deadline))
		if err == nil {
			return conn, nil
		}
		if time.Now().After(deadline) {
			break
		}
	}
	return nil, err
}

// attemptTimeout is how long one attempt may take: the usual short window,
// or whatever remains of the overall budget if that is less.
func (m *SMTPMailer) attemptTimeout(deadline time.Time) time.Duration {
	remaining := time.Until(deadline)
	if remaining < dialTimeout {
		return remaining
	}
	return dialTimeout
}

// dial opens one connection, encrypted from the outset on port 465.
func (m *SMTPMailer) dial(addr string, port int, timeout time.Duration) (net.Conn, error) {
	d := &net.Dialer{Timeout: timeout}
	if port == 465 {
		return tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: m.Host})
	}
	return d.Dial("tcp", addr)
}

// compose builds the message.
func (m *SMTPMailer) compose(to, code string) ([]byte, error) {
	name := m.FromName
	if name == "" {
		name = "Dvarpala"
	}
	if err := headerSafe(name); err != nil {
		return nil, fmt.Errorf("smtp: sender name: %w", err)
	}

	id, err := messageID(m.From)
	if err != nil {
		return nil, err
	}

	now := time.Now
	if m.Now != nil {
		now = m.Now
	}

	// Date and Message-ID are not decoration. Receiving servers score mail
	// that lacks them as machine-generated bulk, which is exactly the
	// judgement that puts a sign-in code in a spam folder.
	headers := []string{
		fmt.Sprintf("From: %s <%s>", name, m.From),
		fmt.Sprintf("To: %s", to),
		"Subject: Your Dvarpala sign-in code",
		fmt.Sprintf("Date: %s", now().Format(time.RFC1123Z)),
		fmt.Sprintf("Message-ID: %s", id),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		// Tells well-behaved mailers not to reply, and keeps this out of
		// out-of-office loops.
		"Auto-Submitted: auto-generated",
		"X-Auto-Response-Suppress: All",
	}

	// The wording lives in one place, so that every way of sending a code
	// says the same thing. SMTP wants CRLF line endings.
	body := strings.ReplaceAll(codeMessage(code), "\n", "\r\n")

	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" +
		body + "\r\n"), nil
}

// messageID returns a unique identifier for one message.
func messageID(from string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating message id: %w", err)
	}

	domain := "dvarpala.local"
	if at := strings.LastIndex(from, "@"); at >= 0 && at < len(from)-1 {
		domain = from[at+1:]
	}
	return fmt.Sprintf("<%s@%s>", hex.EncodeToString(b), domain), nil
}

// headerSafe refuses a value that would break out of its header line.
//
// An address reaching this point has already been matched against the
// database, so it cannot realistically contain a newline. The check stays
// because the cost of being wrong is somebody dictating the headers of a
// message Dvarpala sends, and the cost of the check is nothing.
func headerSafe(v string) error {
	if strings.ContainsAny(v, "\r\n") {
		return errors.New("contains a line break")
	}
	return nil
}

// isLoopback reports whether a host is the machine itself, where an
// unencrypted connection never leaves the box.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// LogMailer writes codes to the server log instead of sending them.
//
// This exists so a server can be exercised without a mail server, and it is
// refused outside debug mode: anyone who can read the log could sign in as
// anyone. The application checks Guard() at startup, exactly as it does for
// the development login provider.
type LogMailer struct{}

// SendCode records the code where an operator with log access can read it.
func (LogMailer) SendCode(to, code string) error {
	log.Printf("otp: code for %s is %s (no mail server configured)", to, code)
	return nil
}

// Guard refuses the log mailer unless the server is in debug mode.
func (LogMailer) Guard(serverMode string) error {
	if strings.ToLower(strings.TrimSpace(serverMode)) != "debug" {
		return fmt.Errorf("log mailer refused: server.mode is %q, expected %q", serverMode, "debug")
	}
	return nil
}

// SMTPProbe checks mail credentials without sending anything.
//
// It exists for the installation check: proving that mail will work should not
// require putting a message in somebody's inbox, and an operator running a
// health check repeatedly should not be mailing themselves each time.
type SMTPProbe struct {
	Host     string
	Port     int
	Username string
	Password string
}

// Probe connects, secures the connection, authenticates, and hangs up.
func (p *SMTPProbe) Probe() error {
	addr := net.JoinHostPort(p.Host, strconv.Itoa(p.Port))

	conn, err := net.DialTimeout("tcp", addr, 8*time.Second)
	if err != nil {
		return fmt.Errorf("connecting to %s: %w", addr, err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	c, err := smtp.NewClient(conn, p.Host)
	if err != nil {
		return fmt.Errorf("greeting from %s: %w", addr, err)
	}
	defer c.Close()

	// Credentials must not cross the wire in the clear, and Go's SMTP client
	// refuses plain authentication on an unsecured connection anyway.
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err := c.StartTLS(&tls.Config{ServerName: p.Host}); err != nil {
			return fmt.Errorf("securing the connection: %w", err)
		}
	}

	if p.Username != "" {
		if err := c.Auth(smtp.PlainAuth("", p.Username, p.Password, p.Host)); err != nil {
			return err
		}
	}

	return c.Quit()
}

// MailerSettings is everything a deployment can say about sending mail.
type MailerSettings struct {
	// Brevo, when an API key is configured.
	BrevoAPIKey   string
	BrevoFrom     string
	BrevoFromName string

	// An ordinary mail server, when a host is configured.
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPFrom     string
	SMTPFromName string

	// ServerMode gates the fallback that writes codes to the log.
	ServerMode string
}

// NewMailer picks how this deployment sends mail, and says so.
//
// One function rather than two, because the server and the CLI must agree.
// They did not before: the server chose, and "dvarpala-cli mail test" built an
// SMTP mailer regardless - so a Brevo deployment would have had its delivery
// check fail while the real path worked, or worse, pass while the real path
// did not.
//
// The order is deliberate. An explicit API key means somebody chose Brevo; a
// mail server means they chose that; the log is what remains, and only in
// debug mode, because anybody who can read the log could then sign in as
// anybody.
func NewMailer(s MailerSettings) (Mailer, string, error) {
	if key := strings.TrimSpace(s.BrevoAPIKey); key != "" {
		from := s.BrevoFrom
		if from == "" {
			from = s.SMTPFrom
		}
		name := s.BrevoFromName
		if name == "" {
			name = s.SMTPFromName
		}
		if from == "" {
			return nil, "", errors.New("brevo is configured but no from address is set")
		}
		return &BrevoMailer{APIKey: key, From: from, FromName: name},
			fmt.Sprintf("Brevo, as %s", from), nil
	}

	if host := strings.TrimSpace(s.SMTPHost); host != "" {
		return &SMTPMailer{
				Host:     host,
				Port:     s.SMTPPort,
				Username: s.SMTPUsername,
				Password: s.SMTPPassword,
				From:     s.SMTPFrom,
				FromName: s.SMTPFromName,
			},
			fmt.Sprintf("%s, as %s", host, s.SMTPFrom), nil
	}

	log := LogMailer{}
	if err := log.Guard(s.ServerMode); err != nil {
		return nil, "", fmt.Errorf("no way to send mail is configured: %w", err)
	}
	return log, "this server's own log (no mail service configured)", nil
}
