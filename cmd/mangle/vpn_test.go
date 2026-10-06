package main

import (
	"io"
	"log/slog"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// The decisions are tested in internal/app. This checks only the wiring: that
// the hook reads the environment variables OpenVPN sets, and refuses when
// the decision does.
func TestClientAuthenticateHookReadsOpenVPNsEnvironment(t *testing.T) {
	ctx := t.Context()
	a, err := app.Open(ctx, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	group := &model.Group{Name: "Staff", IsEnabled: true, MaxDevices: 1}
	a.Store.Groups.Save(ctx, group)
	user := &model.User{Email: "owner@example.com", GroupID: group.ID, IsEnabled: true}
	a.Store.Users.Save(ctx, user)
	a.Store.Devices.Save(ctx, &model.Device{Name: "laptop", UserID: user.ID, Fingerprint: "AA:BB", Serial: "1"})

	hook := vpnActions["client-authenticate"]

	t.Setenv("username", "owner@example.com")
	t.Setenv("password", "")
	t.Setenv("tls_digest_0", "AA:BB")
	if err := hook(a, ctx, nil); err != nil {
		t.Errorf("the owner's connection was refused: %v", err)
	}

	t.Setenv("tls_digest_0", "CC:DD")
	if err := hook(a, ctx, nil); err == nil {
		t.Error("a certificate that is not the user's was accepted")
	}
}

func TestUnknownHookActionIsRefusedBeforeOpeningAnything(t *testing.T) {
	if err := runVPN(t.Context(), []string{"no-such-action", "-root", "/nonexistent"}); err == nil ||
		err.Error() != `unknown vpn action "no-such-action"` {
		t.Errorf("err = %v", err)
	}
}
