package validate

import "testing"

func TestIsIPRefusesZones(t *testing.T) {
	for _, value := range []string{
		"fe80::1%eth0",
		"fe80::1%a\nscript-security 2\nup /tmp/x",
		"fe80::1%x -j ACCEPT",
	} {
		if IsIP(value) || IsCIDR(value) || IsHostname(value) {
			t.Errorf("%q was accepted", value)
		}
	}
	for _, value := range []string{"10.0.0.1", "2001:db8::1", "fe80::1"} {
		if !IsIP(value) {
			t.Errorf("IsIP(%q) = false, want true", value)
		}
	}
}
