package auth

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

// Mailer delivers a sign-in code to a person.
type Mailer interface {
	SendCode(to, code string) error
}

// SMTPMailer sends codes through an ordinary mail server.
type SMTPMailer struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	FromName string
}

// SendCode delivers the code, or reports why it could not.
//
// A failure here must reach the caller: with OTP as the only way in, a mail
// server that silently swallows messages locks every user out of the network
// with no indication of why.
func (m *SMTPMailer) SendCode(to, code string) error {
	from := m.From
	if from == "" {
		return fmt.Errorf("smtp: no from address configured")
	}

	name := m.FromName
	if name == "" {
		name = "Dvarpala"
	}

	msg := strings.Join([]string{
		fmt.Sprintf("From: %s <%s>", name, from),
		fmt.Sprintf("To: %s", to),
		"Subject: Your Dvarpala sign-in code",
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
		"",
		fmt.Sprintf("Your sign-in code is %s", code),
		"",
		"It expires in five minutes and can be used once.",
		"If you did not ask to sign in, ignore this message - somebody",
		"typed your address by mistake, and nothing has happened to your",
		"account.",
		"",
		"Never share this code. Dvarpala will never ask you for it.",
	}, "\r\n")

	addr := fmt.Sprintf("%s:%d", m.Host, m.Port)

	var auth smtp.Auth
	if m.Username != "" {
		auth = smtp.PlainAuth("", m.Username, m.Password, m.Host)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("sending code to %s: %w", to, err)
	}
	return nil
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
