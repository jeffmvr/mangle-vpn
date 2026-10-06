package app

import (
	"encoding/base64"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/auth"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// vpnFixture is a scratch installation with one user and one device.
type vpnFixture struct {
	a      *App
	user   *model.User
	device *model.Device
}

func newVPNFixture(t *testing.T, mfa bool) *vpnFixture {
	t.Helper()
	ctx := t.Context()

	a, err := Open(ctx, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })

	group := &model.Group{Name: "Staff", IsEnabled: true, MaxDevices: 5, MFAEnforced: mfa}
	if err := a.Store.Groups.Save(ctx, group); err != nil {
		t.Fatal(err)
	}

	user := &model.User{Email: "owner@example.com", GroupID: group.ID, IsEnabled: true,
		MFAEnabled: mfa, MFASecret: auth.NewTOTPSecret()}
	user.SetPassword("Password1")
	if err := a.Store.Users.Save(ctx, user); err != nil {
		t.Fatal(err)
	}

	device := &model.Device{Name: "laptop", UserID: user.ID, Fingerprint: "AA:BB", Serial: "1"}
	if err := a.Store.Devices.Save(ctx, device); err != nil {
		t.Fatal(err)
	}

	return &vpnFixture{a: a, user: user, device: device}
}

// login is what a connecting client presents.
func login(username, password, fingerprint string) VPNLogin {
	return VPNLogin{Username: username, Password: password, Fingerprint: fingerprint}
}

func TestVPNAuthenticateRequiresTheCertificateOwner(t *testing.T) {
	f := newVPNFixture(t, false)

	// Another active user, with no two-factor, whose name a holder of
	// owner@'s profile tries to connect under.
	other := &model.User{Email: "other@example.com", GroupID: f.user.GroupID, IsEnabled: true}
	other.SetPassword("Password1")
	if err := f.a.Store.Users.Save(t.Context(), other); err != nil {
		t.Fatal(err)
	}

	attempt := login("other@example.com", "", "AA:BB")
	if err := f.a.AuthenticateVPNClient(t.Context(), attempt); err == nil {
		t.Error("a certificate was accepted under another user's name")
	}

	attempt = login("owner@example.com", "", "AA:BB")
	if err := f.a.AuthenticateVPNClient(t.Context(), attempt); err != nil {
		t.Errorf("the owner was refused: %v", err)
	}

	attempt = login("owner@example.com", "", "")
	if err := f.a.AuthenticateVPNClient(t.Context(), attempt); err == nil {
		t.Error("a connection with no certificate fingerprint was accepted")
	}
}

func TestVPNAuthenticateRefusesReusedCodes(t *testing.T) {
	f := newVPNFixture(t, true)
	code := currentCode(f.user)

	attempt := login("owner@example.com", code, "AA:BB")
	if err := f.a.AuthenticateVPNClient(t.Context(), attempt); err != nil {
		t.Fatalf("a fresh code was refused: %v", err)
	}
	if err := f.a.AuthenticateVPNClient(t.Context(), attempt); err == nil {
		t.Error("the same code was accepted twice")
	}
}

func TestVPNAuthenticateLocksOutGuessing(t *testing.T) {
	f := newVPNFixture(t, true)

	attempt := login("owner@example.com", "000000", "AA:BB")
	for range LoginFailureLimit {
		f.a.AuthenticateVPNClient(t.Context(), attempt)
	}

	attempt = login("owner@example.com", currentCode(f.user), "AA:BB")
	if err := f.a.AuthenticateVPNClient(t.Context(), attempt); err == nil {
		t.Error("a correct code was accepted on a locked account")
	}
}

func TestVPNConnectRefusesAnInactiveOwner(t *testing.T) {
	f := newVPNFixture(t, false)

	f.user.IsEnabled = false
	if err := f.a.Store.Users.Save(t.Context(), f.user); err != nil {
		t.Fatal(err)
	}

	conn := VPNConnection{Fingerprint: "AA:BB", CommonName: "owner@example.com:laptop"}
	if err := f.a.ConnectVPNClient(t.Context(), conn); err == nil {
		t.Error("a disabled user's device was allowed to connect")
	}
}

// currentCode returns the user's authenticator code, first waiting out the
// last two seconds of a 30 second window, so the code is still current when
// it is checked.
func currentCode(user *model.User) string {
	if into := time.Now().Unix() % 30; into >= 28 {
		time.Sleep(time.Duration(31-into) * time.Second)
	}
	return auth.TOTPCode(user.MFASecret, time.Now())
}

// scrv1 is what a profile with static-challenge sends: the password and the
// code, together.
func scrv1(password, code string) string {
	enc := base64.StdEncoding.EncodeToString
	return "SCRV1:" + enc([]byte(password)) + ":" + enc([]byte(code))
}

func requirePassword(t *testing.T, a *App, on bool) {
	t.Helper()
	if err := a.Config.SetBool(t.Context(), config.VPNRequirePassword, on); err != nil {
		t.Fatal(err)
	}
}

func TestVPNPasswordAndCode(t *testing.T) {
	f := newVPNFixture(t, true)
	ctx := t.Context()

	// Off, as today: the code alone works.
	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", currentCode(f.user), "AA:BB")); err != nil {
		t.Fatalf("code alone, password not required: %v", err)
	}

	requirePassword(t, f.a, true)

	// Forget the code just used, so this window's code can be tried again
	// below rather than waiting for the next one.
	allowCodeAgain := func() {
		if _, err := f.a.Store.DB().ExecContext(ctx, `UPDATE users SET mfa_last_step = 0`); err != nil {
			t.Fatal(err)
		}
	}
	allowCodeAgain()

	// On: a profile that sends only the code is refused, so stolen old
	// profiles stop working.
	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", currentCode(f.user), "AA:BB")); err == nil {
		t.Error("the code alone was accepted while the password is required")
	}

	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", scrv1("wrong", currentCode(f.user)), "AA:BB")); err == nil {
		t.Error("a wrong password was accepted")
	}
	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", scrv1("Password1", "000000"), "AA:BB")); err == nil {
		t.Error("a wrong code was accepted with the right password")
	}
	allowCodeAgain()
	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", scrv1("Password1", currentCode(f.user)), "AA:BB")); err != nil {
		t.Errorf("the right password and code were refused: %v", err)
	}
}

func TestVPNPasswordWithoutTwoFactor(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	requirePassword(t, f.a, true)

	// No code to give, so the response is left empty; the password decides.
	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", scrv1("Password1", ""), "AA:BB")); err != nil {
		t.Errorf("the right password was refused: %v", err)
	}
	if err := f.a.AuthenticateVPNClient(ctx, login("owner@example.com", scrv1("nope", ""), "AA:BB")); err == nil {
		t.Error("a wrong password was accepted")
	}
}

func TestVPNChallengePromptFollowsTheSetting(t *testing.T) {
	f := newVPNFixture(t, false)
	if f.a.VPNChallengePrompt() != "" {
		t.Error("profiles ask for a code while the password is not required")
	}
	requirePassword(t, f.a, true)
	if f.a.VPNChallengePrompt() == "" {
		t.Error("profiles do not ask for the code while the password is required")
	}
}
