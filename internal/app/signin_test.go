package app

import (
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

func TestAuthenticate(t *testing.T) {
	ctx := t.Context()
	a, err := Open(ctx, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()

	group := &model.Group{Name: "Staff", IsEnabled: true, MaxDevices: 1}
	a.Store.Groups.Save(ctx, group)
	add := func(email string, enabled bool) *model.User {
		u := &model.User{Email: email, GroupID: group.ID, IsEnabled: enabled}
		u.SetPassword("Password1")
		if err := a.Store.Users.Save(ctx, u); err != nil {
			t.Fatal(err)
		}
		return u
	}
	add("person@example.com", true)
	add("disabled@example.com", false)

	if u, err := a.Authenticate(ctx, " person@example.com ", "Password1"); err != nil || u.Email != "person@example.com" {
		t.Errorf("right password = %v, %v", u, err)
	}

	// Every refusal looks the same to the caller.
	for name, attempt := range map[string][2]string{
		"wrong password": {"person@example.com", "nope"},
		"unknown user":   {"nobody@example.com", "Password1"},
		"disabled":       {"disabled@example.com", "Password1"},
	} {
		if _, err := a.Authenticate(ctx, attempt[0], attempt[1]); !errors.Is(err, ErrBadCredentials) {
			t.Errorf("%s: err = %v, want ErrBadCredentials", name, err)
		}
	}

	// Enough wrong passwords lock the account, and then even the right one
	// is refused.
	for range LoginFailureLimit {
		a.Authenticate(ctx, "person@example.com", "nope")
	}
	if _, err := a.Authenticate(ctx, "person@example.com", "Password1"); !errors.Is(err, ErrBadCredentials) {
		t.Errorf("locked account: err = %v, want ErrBadCredentials", err)
	}
	locked, _ := a.Store.Users.ByEmail(ctx, "person@example.com")
	if !locked.IsLocked(time.Now()) {
		t.Error("the account was not locked")
	}
}

func TestAuthenticateUpgradesOldHashes(t *testing.T) {
	ctx := t.Context()
	a, _ := Open(ctx, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer a.Close()

	group := &model.Group{Name: "Staff", IsEnabled: true, MaxDevices: 1}
	a.Store.Groups.Save(ctx, group)
	// A hash as the Django release wrote it, at 120,000 iterations.
	u := &model.User{Email: "legacy@example.com", GroupID: group.ID, IsEnabled: true,
		Password: "pbkdf2_sha256$120000$NSVqbRsd9Rhl$uRk5YCo0ctJ9lLHb8VEGa9bnh4ISxE6CSgx4g3LC3U8="}
	a.Store.Users.Save(ctx, u)

	if _, err := a.Authenticate(ctx, "legacy@example.com", "Password1"); err != nil {
		t.Fatalf("a Django hash no longer verifies: %v", err)
	}
	upgraded, _ := a.Store.Users.ByEmail(ctx, "legacy@example.com")
	if upgraded.PasswordNeedsRehash() || !upgraded.CheckPassword("Password1") {
		t.Error("the hash was not upgraded on sign in")
	}
}
