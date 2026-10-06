package web

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/auth"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
)

// testServer is the web application over a scratch installation, reached
// through a real HTTP server.
type testServer struct {
	t      *testing.T
	app    *app.App
	server *Server
	http   *httptest.Server
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	a, err := app.Open(t.Context(), t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("app.Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })

	s := New(a, Options{Insecure: true})
	ts := httptest.NewServer(s)
	t.Cleanup(ts.Close)

	return &testServer{t: t, app: a, server: s, http: ts}
}

// install marks setup as done, without going through the setup page.
func (ts *testServer) install() {
	ts.t.Helper()
	if err := ts.app.Config.SetBool(ts.t.Context(), config.AppInstalled, true); err != nil {
		ts.t.Fatal(err)
	}
}

// addUser stores an active user in a group that does or does not enforce
// two-factor authentication.
func (ts *testServer) addUser(email string, mfa bool) *model.User {
	ts.t.Helper()
	ctx := ts.t.Context()

	name := "NoMFA"
	if mfa {
		name = "MFA"
	}
	group, err := ts.app.Store.Groups.ByName(ctx, name)
	if err != nil {
		group = &model.Group{Name: name, IsEnabled: true, MaxDevices: 2, MFAEnforced: mfa}
		if err := ts.app.Store.Groups.Save(ctx, group); err != nil {
			ts.t.Fatal(err)
		}
	}

	// The secret is set up front: the store issues one to a user saved
	// without, and doing so marks them as not yet enrolled.
	user := &model.User{Email: email, GroupID: group.ID, IsEnabled: true, IsAdmin: true,
		MFAEnabled: mfa, MFASecret: auth.NewTOTPSecret()}
	user.SetPassword("Password1")
	if err := ts.app.Store.Users.Save(ctx, user); err != nil {
		ts.t.Fatal(err)
	}
	return user
}

// temporaryPassword gives the user a temporary password that must be
// changed at the next sign in, as the Django release and earlier versions
// of this one issued, and returns it.
func temporaryPassword(user *model.User) string {
	expires := time.Now().Add(7 * 24 * time.Hour)
	user.SetPassword("Temporary1")
	user.PasswordChange = true
	user.PasswordExpire = &expires
	return "Temporary1"
}

// browser is a client with its own cookies that does not follow redirects,
// so each response can be inspected.
type browser struct {
	ts     *testServer
	client *http.Client
}

func (ts *testServer) browser() *browser {
	jar, _ := cookiejar.New(nil)
	return &browser{ts: ts, client: &http.Client{
		Jar: jar,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}
}

func (b *browser) get(path string) *http.Response {
	b.ts.t.Helper()
	resp, err := b.client.Get(b.ts.http.URL + path)
	if err != nil {
		b.ts.t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

// csrf returns the CSRF token, loading a page first when there is none yet.
func (b *browser) csrf() string {
	u, _ := url.Parse(b.ts.http.URL)
	for _, c := range b.client.Jar.Cookies(u) {
		if c.Name == csrfCookie {
			return c.Value
		}
	}
	b.get("/login")
	return b.csrf()
}

// post submits a form, with the CSRF token unless withToken is false.
func (b *browser) post(path string, values url.Values, withToken bool) *http.Response {
	b.ts.t.Helper()
	if values == nil {
		values = url.Values{}
	}
	if withToken {
		values.Set("csrfmiddlewaretoken", b.csrf())
	}
	resp, err := b.client.PostForm(b.ts.http.URL+path, values)
	if err != nil {
		b.ts.t.Fatal(err)
	}
	resp.Body.Close()
	return resp
}

// postJSON sends a JSON body to the API and decodes the JSON reply.
func (b *browser) postJSON(path, body string) map[string]any {
	b.ts.t.Helper()
	req, _ := http.NewRequest(http.MethodPost, b.ts.http.URL+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(csrfHeader, b.csrf())
	resp, err := b.client.Do(req)
	if err != nil {
		b.ts.t.Fatal(err)
	}
	defer resp.Body.Close()

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || resp.StatusCode >= 300 {
		b.ts.t.Fatalf("POST %s = %d (%v): %v", path, resp.StatusCode, err, out)
	}
	return out
}

func (b *browser) login(email, password string) *http.Response {
	return b.post("/login/process", url.Values{"username": {email}, "password": {password}}, true)
}

func (b *browser) enterCode(user *model.User) *http.Response {
	return b.submitCode(currentCode(user))
}

func (b *browser) submitCode(code string) *http.Response {
	return b.post("/mfa/process", url.Values{"code": {code}}, true)
}

// currentCode returns the user's authenticator code, first waiting out the
// last two seconds of a 30 second window, so the code is still current when
// the server checks it.
func currentCode(user *model.User) string {
	if into := time.Now().Unix() % 30; into >= 28 {
		time.Sleep(time.Duration(31-into) * time.Second)
	}
	return auth.TOTPCode(user.MFASecret, time.Now())
}

func TestSigningInAgainDoesNotCarryOverTwoFactor(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	insider := ts.addUser("insider@example.com", false)
	victim := ts.addUser("victim@example.com", true)
	_ = insider

	b := ts.browser()

	// The insider's own account needs no code, so their session is fully
	// confirmed.
	b.login("insider@example.com", "Password1")
	if got := b.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Fatalf("insider profile = %d, want 200", got)
	}

	// Signing in as the victim from the same browser, password only, must
	// not inherit that confirmation.
	b.login("victim@example.com", "Password1")
	if got := b.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Fatalf("victim profile without a code = %d, want 403", got)
	}

	// With the victim's code, it works.
	b.enterCode(victim)
	if got := b.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Fatalf("victim profile after the code = %d, want 200", got)
	}
}

func TestTwoFactorCodeCannotBeReused(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	user := ts.addUser("person@example.com", true)

	code := currentCode(user)

	first := ts.browser()
	first.login("person@example.com", "Password1")
	first.submitCode(code)
	if got := first.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Fatalf("first sign in = %d, want 200", got)
	}

	// Someone who saw the code tries the very same one from elsewhere.
	second := ts.browser()
	second.login("person@example.com", "Password1")
	second.submitCode(code)
	if got := second.get("/api/profile").StatusCode; got == http.StatusOK {
		t.Fatal("the same code signed in twice")
	}
}

func TestLockedAccountRefusesTheRightPassword(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)

	b := ts.browser()
	for range app.LoginFailureLimit {
		b.login("person@example.com", "wrong")
	}

	// A fresh address budget, so only the account lockout is in play.
	ts.server.attempts = newAttemptLimiter()

	b.login("person@example.com", "Password1")
	if got := b.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Fatalf("profile after lockout = %d, want 403", got)
	}
}

func TestAddressBudget(t *testing.T) {
	l := newAttemptLimiter()
	for i := range attemptBurst {
		if !l.Allow("192.0.2.1") {
			t.Fatalf("attempt %d refused within the burst", i+1)
		}
	}
	if l.Allow("192.0.2.1") {
		t.Error("an attempt beyond the burst was allowed")
	}
	if !l.Allow("192.0.2.2") {
		t.Error("another address was refused")
	}
}

func TestSetupNeedsTheCodeFromInstall(t *testing.T) {
	ts := newTestServer(t)
	organization := url.Values{"app_organization": {"Acme"}, "app_hostname": {"vpn.example.com"}}

	// With no code issued, or the wrong one, nobody gets past the first step.
	b := ts.browser()
	b.post("/install/organization", organization, true)
	if status, _ := b.page("/install/vpn"); status != http.StatusFound {
		t.Fatal("moved on without a setup code")
	}

	if err := os.WriteFile(ts.app.Paths.SetupToken, []byte("the-real-code\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	organization.Set("setup_token", "a-guess")
	b.post("/install/organization", organization, true)
	if status, _ := b.page("/install/vpn"); status != http.StatusFound {
		t.Fatal("moved on with the wrong setup code")
	}

	organization.Set("setup_token", "the-real-code")
	b.post("/install/organization", organization, true)
	if status, page := b.page("/install/vpn"); status != http.StatusOK || !strings.Contains(page, "The VPN") {
		t.Fatal("did not move on with the right setup code")
	}

	// Skipping ahead in another session gets nowhere.
	other := ts.browser()
	other.post("/install/admin", url.Values{
		"admin_email": {"eve@example.com"}, "admin_password": {"Password1"}, "admin_password_confirm": {"Password1"},
	}, true)
	if ts.app.Config.Bool(config.AppInstalled, false) {
		t.Fatal("setup completed without its earlier steps")
	}
}

func TestSetupWizard(t *testing.T) {
	ts := newTestServer(t)
	if err := os.WriteFile(ts.app.Paths.SetupToken, []byte("the-real-code\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := ts.browser()

	// The link install prints carries the code, so it is not asked for.
	if status, _ := b.page("/install?token=the-real-code"); status != http.StatusFound {
		t.Error("the start of setup does not lead to its first step")
	}
	if _, page := b.page("/install/organization?token=the-real-code"); strings.Contains(page, `name="setup_token"`) {
		t.Error("the setup code was asked for although the link carried it")
	}
	b.post("/install/organization", url.Values{"app_organization": {"Acme"}, "app_hostname": {"Web.Example.com"}}, true)

	// Routing only some networks needs some networks.
	vpn := url.Values{
		"vpn_hostname": {"vpn.example.com"}, "vpn_protocol": {"udp"}, "vpn_port": {"1194"},
		"vpn_access": {"networks"}, "vpn_routes": {""}, "vpn_subnet": {"172.25.0.0/16"}, "pki_key": {"ecdsa"},
	}
	b.post("/install/vpn", vpn, true)
	if status, _ := b.page("/install/admin"); status != http.StatusFound {
		t.Fatal("moved on with no networks to route")
	}
	b.post("/install/vpn", vpn, true)
	if _, page := b.page("/install/vpn"); !strings.Contains(page, "Add at least one network") {
		t.Error("the VPN step does not say what was wrong")
	}
	vpn.Set("vpn_routes", "10.0.0.0/8\n 192.168.1.0/24 ")
	vpn.Set("vpn_nameservers", "10.0.0.2")
	b.post("/install/vpn", vpn, true)

	// Nothing is created until the last step.
	if ts.app.Config.Get(config.CACertificate) != "" || ts.app.Config.Get(config.VPNHostname) != "" {
		t.Fatal("setup created something before its last step")
	}

	admin := url.Values{
		"admin_email": {"Admin@Example.com"}, "admin_password": {"Password1"}, "admin_password_confirm": {"Password1"},
	}
	b.post("/install/admin", admin, true)
	if ts.app.Config.Bool(config.AppInstalled, false) {
		t.Fatal("setup completed without the administrator's name")
	}
	admin.Set("admin_name", " Ada   Lovelace ")
	b.post("/install/admin", admin, true)
	if !ts.app.Config.Bool(config.AppInstalled, false) {
		t.Fatal("setup did not complete")
	}

	cfg := ts.app.Config
	for name, want := range map[string]string{
		config.AppOrganization: "Acme",
		config.AppHostname:     "web.example.com",
		config.VPNHostname:     "vpn.example.com",
		config.VPNRoutes:       "10.0.0.0/8\n192.168.1.0/24",
		config.VPNNameservers:  "10.0.0.2",
		config.VPNRedirectGW:   "False",
	} {
		if got := cfg.Get(name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}

	ca, err := ts.app.Authority()
	if err != nil {
		t.Fatalf("no certificate authority: %v", err)
	}
	if ca.Certificate.Subject.CommonName != "Acme VPN CA" || ca.KeyType != pki.KeyECDSA {
		t.Errorf("authority = %q, %s", ca.Certificate.Subject.CommonName, ca.KeyType)
	}
	if cfg.Get(config.VPNCertificate) == "" || cfg.Get(config.VPNTLSAuthKey) == "" {
		t.Error("the OpenVPN server's keys were not created")
	}
	if user, err := ts.app.Store.Users.ByEmail(t.Context(), "admin@example.com"); err != nil {
		t.Errorf("the administrator was not created: %v", err)
	} else if user.Name != "Ada Lovelace" {
		t.Errorf("administrator's name = %q", user.Name)
	}
	if _, err := os.Stat(ts.app.Paths.SetupToken); !os.IsNotExist(err) {
		t.Error("the setup code was left on disk")
	}

	// The new administrator is signed in and shown what was done.
	if _, page := b.page("/install/done"); !strings.Contains(page, "Acme VPN CA") || !strings.Contains(page, "10.0.0.0/8") {
		t.Errorf("the finished page does not describe the setup:\n%s", page)
	}
	// And setup cannot be run again.
	if status, _ := b.page("/install/organization"); status != http.StatusFound {
		t.Error("setup was offered again after it finished")
	}
}

func TestSignOutIsAProtectedPost(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)

	b := ts.browser()
	b.login("person@example.com", "Password1")

	if got := b.get("/logout").StatusCode; got != http.StatusMethodNotAllowed {
		t.Errorf("GET /logout = %d, want 405", got)
	}
	if got := b.post("/logout", nil, false).StatusCode; got != http.StatusForbidden {
		t.Errorf("POST /logout without a token = %d, want 403", got)
	}
	if got := b.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Fatalf("still signed in = %d, want 200", got)
	}

	b.post("/logout", nil, true)
	if got := b.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Errorf("profile after signing out = %d, want 403", got)
	}
}

func TestOpenVPNControlsRefuseGet(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")

	for _, path := range []string{"/api/admin/openvpn/toggle", "/api/admin/openvpn/restart"} {
		if got := b.get(path).StatusCode; got != http.StatusMethodNotAllowed {
			t.Errorf("GET %s = %d, want 405", path, got)
		}
	}
}

func TestTemporaryPasswordBlocksTheAPI(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	user := ts.addUser("person@example.com", false)

	password := temporaryPassword(user)
	if err := ts.app.Store.Users.Save(t.Context(), user); err != nil {
		t.Fatal(err)
	}

	b := ts.browser()
	b.login("person@example.com", password)
	if got := b.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Errorf("profile with a temporary password = %d, want 403", got)
	}

	// Once the window has passed, the temporary password stops working.
	past := time.Now().Add(-time.Minute)
	user.PasswordExpire = &past
	ts.app.Store.Users.Save(t.Context(), user)

	late := ts.browser()
	resp := late.login("person@example.com", password)
	if loc := resp.Header.Get("Location"); loc != "/login" {
		t.Errorf("sign in with an expired password went to %q, want /login", loc)
	}
}

func TestSecurityHeaders(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.browser().get("/login")

	csp := resp.Header.Get("Content-Security-Policy")
	for _, directive := range []string{"script-src 'self'", "frame-ancestors 'none'", "object-src 'none'"} {
		if !strings.Contains(csp, directive) {
			t.Errorf("Content-Security-Policy %q lacks %q", csp, directive)
		}
	}
}

func TestImportLinkWorksOnceWithoutASession(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	if err := ts.app.CreateAuthority(t.Context()); err != nil {
		t.Fatalf("CreateAuthority: %v", err)
	}
	ts.addUser("person@example.com", false)

	b := ts.browser()
	b.login("person@example.com", "Password1")

	device := b.postJSON("/api/devices", `{"name":"Laptop","os":"macos"}`)
	link := b.postJSON("/api/devices/"+device["id"].(string)+"/import-link", `{}`)

	const prefix = "openvpn://import-profile/"
	raw, _ := link["url"].(string)
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("link = %q, want the %s prefix", raw, prefix)
	}
	profileURL := strings.TrimPrefix(raw, prefix)

	// OpenVPN Connect fetches the link with no cookies at all.
	resp, err := http.Get(profileURL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import = %d %s, want 200", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Type"); got != "application/x-openvpn-profile" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(string(body), "PRIVATE KEY") {
		t.Error("the profile carries no private key")
	}

	// The link works once, and the device's one download is now spent.
	again, _ := http.Get(profileURL)
	again.Body.Close()
	if again.StatusCode != http.StatusNotFound {
		t.Errorf("second import = %d, want 404", again.StatusCode)
	}
	if got := b.get("/api/devices/" + device["id"].(string) + "?os=macos").StatusCode; got != http.StatusBadRequest {
		t.Errorf("download after import = %d, want 400", got)
	}
}

func TestAdminCanDeleteAUserWhoHasSignedIn(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)
	member := ts.addUser("member@example.com", false)

	// Signing in records an audit event against the member.
	ts.browser().login("member@example.com", "Password1")

	admin := ts.browser()
	admin.login("admin@example.com", "Password1")
	req, _ := http.NewRequest(http.MethodDelete, ts.http.URL+"/api/admin/users/"+member.ID.String(), nil)
	req.Header.Set(csrfHeader, admin.csrf())
	resp, err := admin.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", resp.StatusCode)
	}
	if _, err := ts.app.Store.Users.Get(t.Context(), member.ID); err == nil {
		t.Error("the member is still there")
	}
}

func TestEachSignInStageLandsOnItsPage(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("enrolled@example.com", true)
	ts.addUser("plain@example.com", false)

	unenrolled := ts.addUser("unenrolled@example.com", true)
	unenrolled.MFAEnabled = false
	ts.app.Store.Users.Save(t.Context(), unenrolled)

	temporary := ts.addUser("temporary@example.com", false)
	password := temporaryPassword(temporary)
	ts.app.Store.Users.SetPassword(t.Context(), temporary)

	for _, tc := range []struct {
		name, email, password, want string
	}{
		{"signed out", "", "", "/login"},
		{"needs to enrol", "unenrolled@example.com", "Password1", "/mfa/setup"},
		{"needs a code", "enrolled@example.com", "Password1", "/mfa"},
		{"temporary password", "temporary@example.com", password, "/password"},
		{"fully signed in", "plain@example.com", "Password1", ""},
	} {
		b := ts.browser()
		if tc.email != "" {
			b.login(tc.email, tc.password)
		}
		resp := b.get("/")
		if got := resp.Header.Get("Location"); got != tc.want {
			t.Errorf("%s: / went to %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestChangingPasswordSignsOutOtherSessions(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)

	laptop, phone := ts.browser(), ts.browser()
	laptop.login("person@example.com", "Password1")
	phone.login("person@example.com", "Password1")

	laptop.post("/password/process", url.Values{
		"current_password": {"Password1"}, "password": {"Password2"}, "password_confirm": {"Password2"},
	}, true)

	if got := laptop.get("/api/profile").StatusCode; got != http.StatusOK {
		t.Errorf("the session that changed the password = %d, want 200", got)
	}
	if got := phone.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Errorf("another session after the change = %d, want 403", got)
	}
}

func TestAdminResetSignsTheUserOut(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	user := ts.addUser("person@example.com", false)

	b := ts.browser()
	b.login("person@example.com", "Password1")
	if _, err := ts.app.ResetUserPassword(t.Context(), user); err != nil {
		t.Fatal(err)
	}
	if got := b.get("/api/profile").StatusCode; got != http.StatusForbidden {
		t.Errorf("session after an administrator's password reset = %d, want 403", got)
	}
}

func TestSignInReplacesTheCSRFToken(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)

	b := ts.browser()
	before := b.csrf()
	b.login("person@example.com", "Password1")
	if after := b.csrf(); after == before {
		t.Error("the CSRF token survived signing in")
	}
}

func TestInfoNeedsASignIn(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("person@example.com", false)

	if got := ts.browser().get("/api/info").StatusCode; got != http.StatusForbidden {
		t.Errorf("info signed out = %d, want 403", got)
	}
	b := ts.browser()
	b.login("person@example.com", "Password1")
	if got := b.get("/api/info").StatusCode; got != http.StatusOK {
		t.Errorf("info signed in = %d, want 200", got)
	}
}

// Page links are built from the path and query alone, so nothing a client
// sends in its Host header can appear in them.
func TestPageLinksAreRelative(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)
	ts.addUser("other@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")
	req, _ := http.NewRequest(http.MethodGet, ts.http.URL+"/api/admin/users?page=1&size=1", nil)
	resp, err := b.client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var page map[string]any
	json.NewDecoder(resp.Body).Decode(&page)
	resp.Body.Close()

	next, _ := page["next"].(string)
	if next != "/api/admin/users?page=2&size=1" {
		t.Errorf("next = %q, want a relative link", next)
	}
}
