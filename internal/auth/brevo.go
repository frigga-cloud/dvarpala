package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// brevoEndpoint is where a transactional message is posted.
const brevoEndpoint = "https://api.brevo.com/v3/smtp/email"

// BrevoMailer sends codes through Brevo's HTTP interface.
//
// SMTP remains the ordinary way to send, because it works with whatever mail
// server an organisation already has and ties this product to nobody. This
// exists alongside it for two reasons.
//
// Brevo issues two credentials that are not interchangeable - an API key for
// this interface and a separate SMTP key - and an organisation that already
// has the first should not have to go and find the second. Frigga's own
// user-service already sends through this interface, so a deployment here
// shares an account, a sender and a reputation with the rest of the platform.
//
// And it needs no SMTP ports at all, which matters more than it sounds:
// providers block outbound port 25 by default, and a mail server that judges
// where a connection came from will refuse a datacenter address while
// accepting the same credentials from a laptop.
type BrevoMailer struct {
	APIKey   string
	From     string
	FromName string

	// Timeout bounds the request. Zero means mailTimeout.
	Timeout time.Duration

	// Endpoint is overridable in tests.
	Endpoint string

	// HTTP is overridable in tests.
	HTTP *http.Client
}

// brevoRequest is the message as Brevo expects it.
type brevoRequest struct {
	Sender      brevoAddress   `json:"sender"`
	To          []brevoAddress `json:"to"`
	Subject     string         `json:"subject"`
	TextContent string         `json:"textContent"`
}

type brevoAddress struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email"`
}

// SendCode delivers the code, or reports why it could not.
//
// A failure here must reach the caller: with codes as the only way in, a mail
// service that silently swallows messages locks every user out of the network
// with no indication of why.
func (m *BrevoMailer) SendCode(to, code string) error {
	if m.APIKey == "" {
		return errors.New("brevo: no api key configured")
	}
	if m.From == "" {
		return errors.New("brevo: no from address configured")
	}
	if err := headerSafe(to); err != nil {
		return fmt.Errorf("brevo: recipient address: %w", err)
	}

	name := m.FromName
	if name == "" {
		name = "Dvarpala"
	}

	body, err := json.Marshal(brevoRequest{
		Sender:      brevoAddress{Name: name, Email: m.From},
		To:          []brevoAddress{{Email: to}},
		Subject:     "Your Dvarpala sign-in code",
		TextContent: codeMessage(code),
	})
	if err != nil {
		return fmt.Errorf("brevo: encoding the message: %w", err)
	}

	timeout := m.Timeout
	if timeout <= 0 {
		timeout = mailTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	endpoint := m.Endpoint
	if endpoint == "" {
		endpoint = brevoEndpoint
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("brevo: building the request: %w", err)
	}
	req.Header.Set("api-key", m.APIKey)
	req.Header.Set("content-type", "application/json")
	req.Header.Set("accept", "application/json")

	client := m.HTTP
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("brevo: sending to %s: %w", to, err)
	}
	defer resp.Body.Close()

	// Read a bounded amount: the reply is small, and an unbounded read of
	// somebody else's response is a way to be handed a very large one.
	payload, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Brevo explains itself in the body; the status alone says little. A 401
	// here usually means the SMTP key was supplied instead of the API key.
	return fmt.Errorf("brevo refused the message for %s: %s: %s",
		to, resp.Status, brevoReason(payload))
}

// brevoReason pulls the explanation out of a refusal, falling back to the raw
// body when it is not the shape we expect.
func brevoReason(payload []byte) string {
	var reply struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(payload, &reply); err == nil && reply.Message != "" {
		if reply.Code != "" {
			return reply.Code + ": " + reply.Message
		}
		return reply.Message
	}

	text := strings.TrimSpace(string(payload))
	if text == "" {
		return "no explanation given"
	}
	return text
}

// codeMessage is the body of a sign-in code email, shared by every way of
// sending one so that changing the wording changes it everywhere.
func codeMessage(code string) string {
	return strings.Join([]string{
		fmt.Sprintf("Your sign-in code is %s", code),
		"",
		"It expires in five minutes and can be used once.",
		"",
		"If you did not ask to sign in, ignore this message - somebody",
		"typed your address by mistake, and nothing has happened to your",
		"account.",
		"",
		"Never share this code. Dvarpala will never ask you for it.",
	}, "\n")
}
