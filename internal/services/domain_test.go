package services

import (
	"errors"
	"testing"
)

func TestNormaliseDomain(t *testing.T) {
	ok := []struct{ in, want string }{
		{"frigga.cloud", "frigga.cloud"},
		{"  Frigga.Cloud  ", "frigga.cloud"},
		{"@frigga.cloud", "frigga.cloud"},
		{"FRIGGA.CLOUD", "frigga.cloud"},
		{"mail.frigga.cloud", "mail.frigga.cloud"},

		// An administrator pasting a whole address is a likely mistake, and
		// the domain is unambiguous, so it is taken rather than refused.
		{"richa.t@frigga.cloud", "frigga.cloud"},
	}
	for _, c := range ok {
		got, err := normaliseDomain(c.in)
		if err != nil {
			t.Errorf("normaliseDomain(%q) returned %v, want %q", c.in, err, c.want)
			continue
		}
		if got != c.want {
			t.Errorf("normaliseDomain(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	bad := []string{
		"",
		"   ",
		"localhost",            // no dot, so not a full domain
		".frigga.cloud",        // leading dot
		"frigga.cloud.",        // trailing dot
		"frigga cloud",         // space
		"https://frigga.cloud", // a URL, not a domain
		"frigga.cloud/path",
	}
	for _, in := range bad {
		if got, err := normaliseDomain(in); err == nil {
			t.Errorf("normaliseDomain(%q) = %q, want an error", in, got)
		} else if !errors.Is(err, ErrInvalidDomain) {
			t.Errorf("normaliseDomain(%q) returned %v, want ErrInvalidDomain", in, err)
		}
	}
}

func TestDomainOf(t *testing.T) {
	if got, err := domainOf("Sam@Acme.COM"); err != nil || got != "acme.com" {
		t.Errorf(`domainOf("Sam@Acme.COM") = (%q, %v), want ("acme.com", nil)`, got, err)
	}
	if _, err := domainOf("notanemail"); err == nil {
		t.Error(`domainOf("notanemail") should fail`)
	}
}
