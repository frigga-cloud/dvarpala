package services

import "testing"

// The prefix lengths matter more than they look. A fixed table used to sit in
// the route builder, and anything outside it fell back to a single host - so a
// /20 covering four thousand addresses quietly reached one.
func TestSplitCIDR(t *testing.T) {
	cases := []struct {
		addr    string
		network string
		netmask string
		ok      bool
	}{
		{"10.20.1.55", "10.20.1.55", "255.255.255.255", true},
		{"  10.20.1.55  ", "10.20.1.55", "255.255.255.255", true},
		{"10.20.0.0/16", "10.20.0.0", "255.255.0.0", true},
		{"10.20.0.0/24", "10.20.0.0", "255.255.255.0", true},

		// The lengths the old fixed table had no entry for.
		{"10.20.0.0/20", "10.20.0.0", "255.255.240.0", true},
		{"10.20.1.0/28", "10.20.1.0", "255.255.255.240", true},
		{"10.20.0.0/22", "10.20.0.0", "255.255.252.0", true},

		// Host bits below the netmask are dropped: OpenVPN refuses a route
		// that carries them.
		{"10.20.1.5/24", "10.20.1.0", "255.255.255.0", true},

		{"", "", "", false},
		{"not-an-address", "", "", false},
		{"10.20.1.5 5", "", "", false},
		{"10.20..1.5", "", "", false},
		{"10.20.0.0/33", "", "", false},
		{"10.20.0.0/-1", "", "", false},

		// IPv6 would produce a netmask nothing else here understands.
		{"2001:db8::1", "", "", false},
		{"2001:db8::/32", "", "", false},
	}

	for _, c := range cases {
		network, netmask, ok := SplitCIDR(c.addr)
		if ok != c.ok || network != c.network || netmask != c.netmask {
			t.Errorf("SplitCIDR(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.addr, network, netmask, ok, c.network, c.netmask, c.ok)
		}
	}
}
