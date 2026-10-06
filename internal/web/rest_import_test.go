package web

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
)

// androidChallenge is the pattern OpenVPN for Android matches a challenge
// against (ImportRemoteConfig.kt), copied as it is: it only accepts the
// flags "R,E" in that order.
var androidChallenge = regexp.MustCompile(`(?s).*<Message>CRV1:R,E:(.*):(.*):(.*)</Message>.*`)

// appImport makes the request OpenVPN Connect makes for "Import Profile >
// URL", with credentials sent up front.
func (ts *testServer) appImport(method, username, password string) (*http.Response, string) {
	ts.t.Helper()
	req, _ := http.NewRequest(http.MethodGet, ts.http.URL+"/rest/"+method+"?tls-cryptv2=1&action=import", nil)
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		ts.t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, string(body)
}

func newImportServer(t *testing.T) *testServer {
	ts := newTestServer(t)
	ts.install()
	if err := ts.app.CreateAuthority(t.Context()); err != nil {
		t.Fatalf("CreateAuthority: %v", err)
	}
	return ts
}

func TestAppImportAsksForCredentialsWithTheRealm(t *testing.T) {
	ts := newImportServer(t)

	resp, _ := ts.appImport("GetUserlogin", "", "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
	if got := resp.Header.Get("WWW-Authenticate"); got != `Basic realm="OpenVPN Access Server"` {
		t.Errorf("WWW-Authenticate = %q", got)
	}
}

func TestAppImportWithoutTwoFactor(t *testing.T) {
	ts := newImportServer(t)
	user := ts.addUser("person@example.com", false) // the group allows two devices

	resp, body := ts.appImport("GetUserlogin", "person@example.com", "Password1")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("import = %d %s", resp.StatusCode, body)
	}
	if !strings.HasPrefix(body, "# OVPN_ACCESS_SERVER_PROFILE=person@example.com@") {
		t.Errorf("profile does not name itself:\n%.200s", body)
	}
	if !strings.Contains(body, "PRIVATE KEY") || !strings.Contains(body, "auth-user-pass") {
		t.Error("the profile is incomplete")
	}

	// A second import is a second device; a third goes over the limit.
	if resp, body := ts.appImport("GetUserlogin", "person@example.com", "Password1"); resp.StatusCode != http.StatusOK {
		t.Fatalf("second import = %d %s", resp.StatusCode, body)
	}
	resp, body = ts.appImport("GetUserlogin", "person@example.com", "Password1")
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(body, "as many devices") {
		t.Errorf("third import = %d %s, want a 403 about the limit", resp.StatusCode, body)
	}

	devices, _ := ts.app.Store.Devices.ByUser(t.Context(), user.ID)
	var names []string
	for _, d := range devices {
		names = append(names, d.Name)
	}
	if strings.Join(names, ",") != "OpenVPN Connect,OpenVPN Connect 2" {
		t.Errorf("devices = %v", names)
	}
}

func TestAppImportWithTwoFactor(t *testing.T) {
	ts := newImportServer(t)
	user := ts.addUser("person@example.com", true)

	resp, body := ts.appImport("GetUserlogin", "person@example.com", "Password1")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("first step = %d %s, want a 401 challenge", resp.StatusCode, body)
	}
	match := androidChallenge.FindStringSubmatch(body)
	if match == nil {
		t.Fatalf("the Android client would not recognise this challenge: %s", body)
	}
	state, login := match[1], match[2]
	if decoded, _ := base64.StdEncoding.DecodeString(login); string(decoded) != "person@example.com" {
		t.Errorf("challenge username = %q", decoded)
	}
	if strings.Contains(match[3], ":") {
		t.Error("the prompt holds a colon, which splits the Android client's match")
	}

	// A wrong code is refused, and spends the challenge.
	resp, _ = ts.appImport("GetUserlogin", "person@example.com", "CRV1::"+state+"::000000")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong code = %d, want 401", resp.StatusCode)
	}

	// A fresh challenge, answered properly.
	_, body = ts.appImport("GetUserlogin", "person@example.com", "Password1")
	state = androidChallenge.FindStringSubmatch(body)[1]
	answer := "CRV1::" + state + "::" + currentCode(user)

	resp, body = ts.appImport("GetUserlogin", "person@example.com", answer)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, "PRIVATE KEY") {
		t.Fatalf("answered challenge = %d %.200s", resp.StatusCode, body)
	}

	// The same answer cannot be sent twice.
	if resp, _ := ts.appImport("GetUserlogin", "person@example.com", answer); resp.StatusCode == http.StatusOK {
		t.Error("an answered challenge was accepted again")
	}
}

func TestAppImportRefusals(t *testing.T) {
	ts := newImportServer(t)
	ts.addUser("person@example.com", false)

	if resp, body := ts.appImport("GetUserlogin", "person@example.com", "wrong"); resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "AUTH_FAILED") {
		t.Errorf("wrong password = %d %s", resp.StatusCode, body)
	}
	if resp, body := ts.appImport("GetUserlogin", "nobody@example.com", "Password1"); resp.StatusCode != http.StatusUnauthorized || !strings.Contains(body, "AUTH_FAILED") {
		t.Errorf("unknown user = %d %s", resp.StatusCode, body)
	}
	if resp, body := ts.appImport("GetAutologin", "person@example.com", "Password1"); resp.StatusCode != http.StatusForbidden || !strings.Contains(body, "NEED_AUTOLOGIN") {
		t.Errorf("auto-login = %d %s", resp.StatusCode, body)
	}
}

func TestAppImportNeedsTwoFactorSetUpFirst(t *testing.T) {
	ts := newImportServer(t)
	user := ts.addUser("person@example.com", true)
	user.MFAEnabled = false
	ts.app.Store.Users.Save(t.Context(), user)

	resp, body := ts.appImport("GetUserlogin", "person@example.com", "Password1")
	if resp.StatusCode != http.StatusForbidden || !strings.Contains(body, "two-factor") {
		t.Errorf("unenrolled user = %d %s, want a 403 sending them to the website", resp.StatusCode, body)
	}
}

func TestProfilesAskForTheCodeWhenThePasswordIsRequired(t *testing.T) {
	ts := newImportServer(t)
	ts.addUser("person@example.com", false)

	_, before := ts.appImport("GetUserlogin", "person@example.com", "Password1")
	if strings.Contains(before, "static-challenge") {
		t.Error("a profile asks for a code while the password is not required")
	}

	ts.app.Config.SetBool(t.Context(), config.VPNRequirePassword, true)
	_, after := ts.appImport("GetUserlogin", "person@example.com", "Password1")
	if !strings.Contains(after, "\nstatic-challenge \"") {
		t.Error("a profile does not ask for the code while the password is required")
	}

	b := ts.browser()
	b.login("person@example.com", "Password1")
	resp, _ := b.client.Get(ts.http.URL + "/api/profile")
	var profile map[string]any
	json.NewDecoder(resp.Body).Decode(&profile)
	resp.Body.Close()
	if profile["vpn_password_required"] != true {
		t.Errorf("profile vpn_password_required = %v, want true", profile["vpn_password_required"])
	}
}
