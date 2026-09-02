package web

import (
	"errors"
	"fmt"
	"testing"

	"dvarpala/internal/auth"
)

// stepAfter reproduces the decision send() makes about which step to show
// when asking for a code returns an error.
func stepAfter(err error) string {
	step := "code"
	if err != nil && !errors.Is(err, auth.ErrTooSoon) {
		step = "email"
	}
	return step
}

// A captive-portal browser submitting the form twice sends a code on the
// first attempt and trips the cooldown on the second. If the second reply
// takes the person back to the email box, they are left holding a working
// code with nowhere to type it - which is what happened on a real phone.
func TestCooldownKeepsThePersonOnTheCodeStep(t *testing.T) {
	if got := stepAfter(fmt.Errorf("%w (43 seconds)", auth.ErrTooSoon)); got != "code" {
		t.Errorf("after a cooldown the page showed the %q step, want %q", got, "code")
	}
}

// Errors that mean no code exists must send them back to the start, because
// there is nothing for them to type.
func TestOtherFailuresReturnToTheEmailStep(t *testing.T) {
	for _, err := range []error{
		errors.New("sending code to sam@acme.com: smtp: connection refused"),
		auth.ErrDailyLimit,
	} {
		if got := stepAfter(err); got != "email" {
			t.Errorf("after %v the page showed the %q step, want %q", err, got, "email")
		}
	}
}

func TestASuccessfulRequestShowsTheCodeStep(t *testing.T) {
	if got := stepAfter(nil); got != "code" {
		t.Errorf("after a successful send the page showed the %q step, want %q", got, "code")
	}
}
