package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

// brevoStub stands in for Brevo's API, recording what it was sent.
type brevoStub struct {
	status  int
	body    string
	gotKey  string
	gotBody brevoRequest
	raw     string
}

func (b *brevoStub) server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.gotKey = r.Header.Get("api-key")
		var buf [4096]byte
		n, _ := r.Body.Read(buf[:])
		b.raw = string(buf[:n])
		_ = json.Unmarshal(buf[:n], &b.gotBody)

		status := b.status
		if status == 0 {
			status = http.StatusCreated
		}
		w.WriteHeader(status)
		if b.body != "" {
			_, _ = w.Write([]byte(b.body))
		}
	}))
}

func TestBrevoSendsTheCodeToTheRightPerson(t *testing.T) {
	stub := &brevoStub{}
	srv := stub.server()
	defer srv.Close()

	m := &BrevoMailer{
		APIKey: "xkeysib-secret", From: "noreply@example.com",
		FromName: "Frigga Accounts", Endpoint: srv.URL,
	}

	if err := m.SendCode("sam@example.com", "481920"); err != nil {
		t.Fatalf("sending: %v", err)
	}

	if stub.gotKey != "xkeysib-secret" {
		t.Errorf("api key sent as %q", stub.gotKey)
	}
	if stub.gotBody.Sender.Email != "noreply@example.com" {
		t.Errorf("sender is %q", stub.gotBody.Sender.Email)
	}
	if stub.gotBody.Sender.Name != "Frigga Accounts" {
		t.Errorf("sender name is %q", stub.gotBody.Sender.Name)
	}
	if len(stub.gotBody.To) != 1 || stub.gotBody.To[0].Email != "sam@example.com" {
		t.Errorf("recipients are %v", stub.gotBody.To)
	}
	if !strings.Contains(stub.gotBody.TextContent, "481920") {
		t.Errorf("the code is not in the message: %q", stub.gotBody.TextContent)
	}
	// Somebody who did not ask for this needs to be told it is harmless.
	if !strings.Contains(stub.gotBody.TextContent, "did not ask") {
		t.Error("the message does not say what to do if you did not request it")
	}
}

// A refusal has to carry Brevo's own words. The commonest cause is the SMTP
// key being supplied where the API key belongs, and the status alone would
// never say so.
func TestBrevoRefusalCarriesTheReason(t *testing.T) {
	stub := &brevoStub{
		status: http.StatusUnauthorized,
		body:   `{"code":"unauthorized","message":"Key not found"}`,
	}
	srv := stub.server()
	defer srv.Close()

	m := &BrevoMailer{APIKey: "wrong", From: "noreply@example.com", Endpoint: srv.URL}

	err := m.SendCode("sam@example.com", "481920")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"sam@example.com", "401", "unauthorized", "Key not found"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestBrevoRefusalWithAnUnexpectedBodyIsStillReported(t *testing.T) {
	stub := &brevoStub{status: http.StatusBadGateway, body: "<html>gateway</html>"}
	srv := stub.server()
	defer srv.Close()

	m := &BrevoMailer{APIKey: "k", From: "noreply@example.com", Endpoint: srv.URL}
	err := m.SendCode("sam@example.com", "481920")
	if err == nil || !strings.Contains(err.Error(), "gateway") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Somebody is waiting on a page while this runs.
func TestBrevoDoesNotHangWhenTheServiceGoesQuiet(t *testing.T) {
	// Released when the test ends, so closing the server does not sit waiting
	// for a handler that is deliberately asleep.
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	m := &BrevoMailer{
		APIKey: "k", From: "noreply@example.com",
		Endpoint: srv.URL, Timeout: 2 * time.Second,
	}

	done := make(chan error, 1)
	go func() { done <- m.SendCode("sam@example.com", "481920") }()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error from a service that never answered")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("SendCode did not return: a hung service would hang the portal")
	}
}

func TestBrevoRefusesIncompleteConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		mailer *BrevoMailer
		want   string
	}{
		{"no key", &BrevoMailer{From: "noreply@example.com"}, "api key"},
		{"no from", &BrevoMailer{APIKey: "k"}, "from address"},
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

func TestBrevoRefusesHeaderInjection(t *testing.T) {
	m := &BrevoMailer{APIKey: "k", From: "noreply@example.com"}
	err := m.SendCode("sam@example.com\r\nBcc: everyone@example.com", "481920")
	if err == nil || !strings.Contains(err.Error(), "line break") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// The server and the CLI must not disagree about how mail is sent.
func TestTheMailerChosenMatchesWhatWasConfigured(t *testing.T) {
	cases := []struct {
		name     string
		settings MailerSettings
		want     string
	}{
		{
			"an api key means brevo",
			MailerSettings{BrevoAPIKey: "k", BrevoFrom: "noreply@example.com",
				SMTPHost: "smtp.example.com", SMTPFrom: "other@example.com"},
			"*auth.BrevoMailer",
		},
		{
			"a mail server, with no api key",
			MailerSettings{SMTPHost: "smtp.example.com", SMTPFrom: "noreply@example.com"},
			"*auth.SMTPMailer",
		},
		{
			"neither, in debug",
			MailerSettings{ServerMode: "debug"},
			"auth.LogMailer",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, described, err := NewMailer(tc.settings)
			if err != nil {
				t.Fatalf("choosing: %v", err)
			}
			if got := typeName(m); got != tc.want {
				t.Errorf("chose %s, want %s", got, tc.want)
			}
			if described == "" {
				t.Error("the choice was not described, so no log can report it")
			}
		})
	}

	// Outside debug, with nothing configured, there is no safe fallback.
	if _, _, err := NewMailer(MailerSettings{ServerMode: "release"}); err == nil {
		t.Error("a release server with no mail service should refuse to start")
	}
}

func typeName(v interface{}) string {
	return reflect.TypeOf(v).String()
}

// A stray space either side of an address in a YAML file is easy to leave
// behind, and it produces a refusal that talks about credentials rather than
// about the address - which sends somebody looking in entirely the wrong
// place. Seen in a real deployment.
func TestSurroundingSpaceInAnAddressIsIgnored(t *testing.T) {
	m, described, err := NewMailer(MailerSettings{
		BrevoAPIKey:   " xkeysib-key ",
		BrevoFrom:     " noreply@example.com ",
		BrevoFromName: " Frigga Accounts ",
	})
	if err != nil {
		t.Fatalf("choosing: %v", err)
	}

	b, ok := m.(*BrevoMailer)
	if !ok {
		t.Fatalf("chose %T", m)
	}
	if b.From != "noreply@example.com" {
		t.Errorf("from is %q", b.From)
	}
	if b.FromName != "Frigga Accounts" {
		t.Errorf("from name is %q", b.FromName)
	}
	if b.APIKey != "xkeysib-key" {
		t.Errorf("api key is %q", b.APIKey)
	}
	if strings.Contains(described, "  ") {
		t.Errorf("the log line would read oddly: %q", described)
	}
}
