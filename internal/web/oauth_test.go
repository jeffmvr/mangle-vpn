package web

import "testing"

func TestAcceptOAuth2Profile(t *testing.T) {
	work := oauth2Profile{Email: "ada@example.com", VerifiedEmail: true, HostedDomain: "example.com"}
	personal := oauth2Profile{Email: "ada@gmail.com", VerifiedEmail: true}
	unverified := oauth2Profile{Email: "ada@example.com", HostedDomain: "example.com"}

	for _, tc := range []struct {
		name    string
		profile oauth2Profile
		domain  string
		ok      bool
	}{
		{"no restriction", personal, "", true},
		{"right domain", work, "example.com", true},
		{"domain case", work, "EXAMPLE.com", true},
		{"personal account", personal, "example.com", false},
		{"other domain", work, "acme.test", false},
		{"unverified address", unverified, "", false},
	} {
		if err := acceptOAuth2Profile(tc.profile, tc.domain); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok = %v", tc.name, err, tc.ok)
		}
	}
}
