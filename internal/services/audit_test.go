package services

import "testing"

// The details column is jsonb. An empty string is not valid JSON, so an entry
// with no context must still produce an object - otherwise the insert fails,
// and Log swallows the error, so the record is lost without a trace.
func TestMarshalDetailsNeverProducesEmptyString(t *testing.T) {
	cases := map[string]map[string]interface{}{
		"nil details":   nil,
		"empty details": {},
	}

	for name, details := range cases {
		t.Run(name, func(t *testing.T) {
			if got := marshalDetails(details); got != "{}" {
				t.Errorf("marshalDetails() = %q, want %q", got, "{}")
			}
		})
	}
}

func TestMarshalDetailsRendersContext(t *testing.T) {
	got := marshalDetails(map[string]interface{}{"email": "a@b.com"})

	if want := `{"email":"a@b.com"}`; got != want {
		t.Errorf("marshalDetails() = %q, want %q", got, want)
	}
}
