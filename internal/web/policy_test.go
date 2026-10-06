package web

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// set stores settings for a test.
func (ts *testServer) set(values map[string]string) {
	ts.t.Helper()
	for name, value := range values {
		if err := ts.app.Config.Set(ts.t.Context(), name, value); err != nil {
			ts.t.Fatal(err)
		}
	}
}

func TestPasswordRulesFollowTheSettings(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)
	ts.set(map[string]string{config.AuthPasswordMinLength: "14", config.AuthPasswordComplexity: "False"})

	b := ts.browser()
	b.login("person@example.com", "Password1")
	change := func(password string) int {
		b.post("/password/process", url.Values{
			"current_password": {"Password1"}, "password": {password}, "password_confirm": {password},
		}, true)
		user, _ := ts.app.Store.Users.ByEmail(t.Context(), "person@example.com")
		if user.CheckPassword(password) {
			return 1
		}
		return 0
	}

	if change("Short1Aa") == 1 {
		t.Error("an 8 character password was accepted with a 14 character minimum")
	}
	if change("correct horse battery staple") == 0 {
		t.Error("a long passphrase without mixed characters was refused with complexity off")
	}
	if _, page := b.page("/password"); !strings.Contains(page, "At least 14 characters.") {
		t.Error("the password page does not describe the rules in force")
	}
}

func TestLockoutFollowsTheSettings(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)
	ts.set(map[string]string{config.AuthLockoutAttempts: "5"})

	b := ts.browser()
	for range 5 {
		b.login("person@example.com", "wrong")
	}
	user, _ := ts.app.Store.Users.ByEmail(t.Context(), "person@example.com")
	if user.LockedUntil == nil {
		t.Error("five failures did not lock the account with a limit of five")
	}
}

func TestAdministrationCanBeLimitedToNetworks(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")

	// A list without the administrator's own address would lock them out,
	// so it is refused.
	if status, body := b.do(http.MethodPut, "/api/admin/settings/security",
		`{"auth_admin_networks": "10.0.0.0/8"}`); status != http.StatusBadRequest || !strings.Contains(string(body), "lock you out") {
		t.Fatalf("saving a list without our address = %d: %s", status, body)
	}
	if status, body := b.do(http.MethodPut, "/api/admin/settings/security",
		`{"auth_admin_networks": "127.0.0.1\n10.0.0.0/8"}`); status != http.StatusNoContent {
		t.Fatalf("saving a list with our address = %d: %s", status, body)
	}
	if status, _ := b.do(http.MethodGet, "/api/admin/users", ""); status != http.StatusOK {
		t.Errorf("administration from an allowed address = %d, want 200", status)
	}

	// From anywhere else it is closed, though the rest of the application
	// is not.
	ts.set(map[string]string{config.AuthAdminNetworks: "10.0.0.0/8"})
	if status, _ := b.do(http.MethodGet, "/api/admin/users", ""); status != http.StatusForbidden {
		t.Errorf("administration from elsewhere = %d, want 403", status)
	}
	if status, _ := b.do(http.MethodGet, "/api/profile", ""); status != http.StatusOK {
		t.Errorf("the profile from elsewhere = %d, want 200", status)
	}
}

func TestSingleSignOnOnlyKeepsPasswordsForAdministrators(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)
	ts.addMember("member@example.com", "")
	ts.set(map[string]string{
		config.OAuth2Provider: ProviderOIDC, config.OAuth2Issuer: "https://idp.example.com",
		config.OAuth2ClientID: "id", config.OAuth2ClientSecret: "secret", config.OAuth2Name: "Okta",
		config.OAuth2Only: "True",
	})

	if _, page := ts.browser().page("/login"); strings.Contains(page, `name="password"`) || !strings.Contains(page, "Sign in with a password") {
		t.Error("the sign in page still offers a password, or no way to ask for one")
	}

	member := ts.browser()
	member.login("member@example.com", "Password1")
	if status, _ := member.do(http.MethodGet, "/api/profile", ""); status == http.StatusOK {
		t.Error("a member signed in with a password")
	}

	admin := ts.browser()
	admin.login("admin@example.com", "Password1")
	if status, _ := admin.do(http.MethodGet, "/api/profile", ""); status != http.StatusOK {
		t.Error("an administrator could not sign in with a password")
	}
}

func TestFirstSingleSignOnCreatesAnAccountInTheChosenGroup(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	idp := newFakeIdP(t)
	group := &model.Group{Name: "Newcomers", IsEnabled: true, MaxDevices: 1}
	ts.app.Store.Groups.Save(t.Context(), group)

	ts.set(map[string]string{
		config.OAuth2Provider: ProviderOIDC, config.OAuth2Issuer: idp.server.URL,
		config.OAuth2ClientID: "client-id", config.OAuth2ClientSecret: "secret", config.OAuth2Name: "Okta",
		config.OAuth2AllowedDomain: "example.com", config.OAuth2AutoGroup: group.ID.String(),
	})

	b := ts.browser()
	if got := idp.signIn(t, b, map[string]any{"email": "new@example.com", "email_verified": true, "name": "New Person"}, nil); got != "/" {
		t.Fatalf("first sign in went to %q, want /", got)
	}
	user, err := ts.app.Store.Users.ByEmail(t.Context(), "new@example.com")
	if err != nil || user.GroupID != group.ID || user.Name != "New Person" {
		t.Fatalf("created %+v, %v", user, err)
	}

	// Outside the domain, nobody gets one.
	other := ts.browser()
	if got := idp.signIn(t, other, map[string]any{"email": "eve@elsewhere.test", "email_verified": true}, nil); got != "/login" {
		t.Errorf("a sign in from another domain went to %q, want /login", got)
	}
	if _, err := ts.app.Store.Users.ByEmail(t.Context(), "eve@elsewhere.test"); err == nil {
		t.Error("an account was made for another domain")
	}
}

func TestLogoIsServedSafely(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)
	b := ts.browser()
	b.login("admin@example.com", "Password1")

	svg := `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`
	logo := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
	if status, body := b.do(http.MethodPut, "/api/admin/settings/app", `{"app_logo": "`+logo+`"}`); status != http.StatusNoContent {
		t.Fatalf("saving a logo = %d: %s", status, body)
	}
	if status, _ := b.do(http.MethodPut, "/api/admin/settings/app", `{"app_logo": "data:text/html;base64,PGI+"}`); status != http.StatusBadRequest {
		t.Errorf("an HTML logo = %d, want 400", status)
	}

	resp := ts.browser().get("/logo")
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/svg+xml" ||
		!strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("/logo = %d %q %q", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header.Get("Content-Security-Policy"))
	}
	if _, page := ts.browser().page("/login"); !strings.Contains(page, `src="/logo?v=`) {
		t.Error("the sign in page does not show the logo")
	}
}

func TestSupportContactIsLinkedFromSignIn(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)
	b := ts.browser()
	b.login("admin@example.com", "Password1")

	for _, contact := range []string{"javascript:alert(1)", "http://help.example.com", "not an address"} {
		if status, _ := b.do(http.MethodPut, "/api/admin/settings/app", `{"app_support_contact": "`+contact+`"}`); status != http.StatusBadRequest {
			t.Errorf("a help contact of %q = %d, want 400", contact, status)
		}
	}

	if _, page := ts.browser().page("/login"); strings.Contains(page, "Need help?") {
		t.Error("the sign in page offers help with none set")
	}

	for contact, href := range map[string]string{
		"it@example.com":               `href="mailto:it@example.com"`,
		"https://help.example.com/vpn": `href="https://help.example.com/vpn"`,
	} {
		if status, body := b.do(http.MethodPut, "/api/admin/settings/app", `{"app_support_contact": "`+contact+`"}`); status != http.StatusNoContent {
			t.Fatalf("saving %q = %d: %s", contact, status, body)
		}
		if _, page := ts.browser().page("/login"); !strings.Contains(page, href) {
			t.Errorf("the sign in page does not link %s", href)
		}
	}
}
