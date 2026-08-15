package auth

import (
	"context"
	"strings"
	"testing"
)

func TestDevProviderExchange(t *testing.T) {
	p := NewDevProvider("http://localhost:8080")

	tests := []struct {
		code      string
		wantEmail string
		wantErr   bool
	}{
		{"sam@acme.com", "sam@acme.com", false},
		{"  Sam@ACME.com  ", "sam@acme.com", false}, // normalised
		{"not-an-email", "", true},
		{"", "", true},
	}

	for _, tc := range tests {
		info, err := p.Exchange(context.Background(), tc.code)
		if tc.wantErr {
			if err == nil {
				t.Errorf("Exchange(%q) expected an error, got %+v", tc.code, info)
			}
			continue
		}
		if err != nil {
			t.Errorf("Exchange(%q) unexpected error: %v", tc.code, err)
			continue
		}
		if info.Email != tc.wantEmail {
			t.Errorf("Exchange(%q) email = %q, want %q", tc.code, info.Email, tc.wantEmail)
		}
		if info.Provider != "dev" {
			t.Errorf("Exchange(%q) provider = %q, want dev", tc.code, info.Provider)
		}
	}
}

func TestDevProviderAuthURLCarriesState(t *testing.T) {
	p := NewDevProvider("http://localhost:8080/")

	got := p.AuthURL("abc123")
	if !strings.Contains(got, "state=abc123") {
		t.Errorf("AuthURL = %q, expected it to carry the state", got)
	}
	if strings.Contains(got, "//auth") {
		t.Errorf("AuthURL = %q, trailing slash in base URL was not trimmed", got)
	}
}

// The dev provider accepts any identity, so it must refuse to run outside
// debug mode.
func TestDevProviderRefusedOutsideDebug(t *testing.T) {
	p := NewDevProvider("http://localhost:8080")

	if err := p.Guard("debug"); err != nil {
		t.Errorf("Guard(debug) = %v, want nil", err)
	}
	for _, mode := range []string{"release", "production", ""} {
		if err := p.Guard(mode); err == nil {
			t.Errorf("Guard(%q) = nil, expected refusal", mode)
		}
	}
}

func TestDomainAllowList(t *testing.T) {
	tests := []struct {
		name    string
		allowed []string
		email   string
		want    bool
	}{
		{"empty list allows anything", nil, "sam@anywhere.com", true},
		{"listed domain allowed", []string{"acme.com"}, "sam@acme.com", true},
		{"case insensitive", []string{"ACME.com"}, "sam@Acme.COM", true},
		{"other domain blocked", []string{"acme.com"}, "sam@evil.com", false},
		{"subdomain is not the domain", []string{"acme.com"}, "sam@mail.acme.com", false},
		{"second entry matches", []string{"acme.com", "contractor.com"}, "x@contractor.com", true},
		{"no at sign", []string{"acme.com"}, "notanemail", false},
		{"suffix trick blocked", []string{"acme.com"}, "sam@notacme.com", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := &Service{allowedDomains: normaliseDomains(tc.allowed)}
			if got := s.domainAllowed(tc.email); got != tc.want {
				t.Errorf("domainAllowed(%q) with %v = %v, want %v",
					tc.email, tc.allowed, got, tc.want)
			}
		})
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()
	if r.Len() != 0 {
		t.Errorf("new registry Len = %d, want 0", r.Len())
	}

	r.Add(NewDevProvider("http://localhost:8080"))
	if r.Len() != 1 {
		t.Errorf("Len after Add = %d, want 1", r.Len())
	}

	if _, err := r.Get("dev"); err != nil {
		t.Errorf("Get(dev) = %v, want a provider", err)
	}
	if _, err := r.Get("google"); err == nil {
		t.Error("Get(google) succeeded, expected an error for an unregistered provider")
	}
}
