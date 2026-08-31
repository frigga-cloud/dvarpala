package providers

import (
	"strings"
	"testing"
)

// Empty must stay permissive. A deployment that never set allowed_ips would
// otherwise get a server with no way in at all, including for the person who
// just ran the installer.
func TestAdminSourceRanges(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"nothing configured opens to everywhere", nil, []string{"0.0.0.0/0"}},
		{"empty slice opens to everywhere", []string{}, []string{"0.0.0.0/0"}},
		{"blank entries are not addresses", []string{"", "   "}, []string{"0.0.0.0/0"}},
		{"one address is honoured", []string{"49.207.203.139/32"}, []string{"49.207.203.139/32"}},
		{"several are kept in order", []string{"10.0.0.0/8", "49.207.203.139/32"},
			[]string{"10.0.0.0/8", "49.207.203.139/32"}},
		{"surrounding space is trimmed", []string{"  49.207.203.139/32  "},
			[]string{"49.207.203.139/32"}},
		{"a blank among real ones is dropped", []string{"10.0.0.0/8", "", "1.2.3.4/32"},
			[]string{"10.0.0.0/8", "1.2.3.4/32"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := adminSourceRanges(c.in)
			if strings.Join(got, ",") != strings.Join(c.want, ",") {
				t.Errorf("adminSourceRanges(%v) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}
