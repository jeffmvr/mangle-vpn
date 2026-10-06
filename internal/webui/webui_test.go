package webui

import "testing"

func TestImmutable(t *testing.T) {
	for name, want := range map[string]bool{
		"UsersView-BMYrZSJz.js":       true,
		"index-B65inttr.css":          true,
		"useApi-BPuI6ZR9-C7E7mo1H.js": true,
		"favicon.ico":                 false,
		"openvpn-client-installer.sh": false,
		"index.html":                  false,
		"app.js":                      false,
	} {
		if got := Immutable(name); got != want {
			t.Errorf("Immutable(%q) = %v, want %v", name, got, want)
		}
	}
}
