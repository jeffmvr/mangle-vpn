package store

import (
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

// rawSecret reads a user's two-factor secret as it is stored.
func rawSecret(t *testing.T, s *Store, user *model.User) string {
	t.Helper()
	var value string
	if err := s.DB().QueryRowContext(t.Context(), `SELECT mfa_secret FROM users WHERE id = ?`, user.ID).Scan(&value); err != nil {
		t.Fatal(err)
	}
	return value
}

func rawSetting(t *testing.T, s *Store, name string) string {
	t.Helper()
	var value string
	s.DB().QueryRowContext(t.Context(), `SELECT value FROM setting WHERE name = ?`, name).Scan(&value)
	return value
}

func TestSecretsAreStoredEncrypted(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	if err := s.UseSecretKey(testKey, []string{"ca_key"}); err != nil {
		t.Fatal(err)
	}

	_, user := seedUser(t, s, "person@example.com")
	plain := user.MFASecret

	if raw := rawSecret(t, s, user); !strings.HasPrefix(raw, sealedPrefix) || strings.Contains(raw, plain) {
		t.Errorf("the two-factor secret is stored as %q", raw)
	}
	loaded, _ := s.Users.ByEmail(ctx, "person@example.com")
	if loaded.MFASecret != plain {
		t.Error("the two-factor secret did not read back")
	}

	s.Settings.Set(ctx, "ca_key", "PRIVATE KEY")
	s.Settings.Set(ctx, "app_organization", "Acme")
	if raw := rawSetting(t, s, "ca_key"); !strings.HasPrefix(raw, sealedPrefix) {
		t.Errorf("a secret setting is stored as %q", raw)
	}
	if raw := rawSetting(t, s, "app_organization"); raw != "Acme" {
		t.Errorf("an ordinary setting is stored as %q", raw)
	}
	all, _ := s.Settings.All(ctx)
	if all["ca_key"] != "PRIVATE KEY" {
		t.Error("a secret setting did not read back")
	}
}

func TestSealExistingAndUnsealAll(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	// Written before encryption was on, as the Django release would have.
	_, user := seedUser(t, s, "person@example.com")
	plain := user.MFASecret
	s.Settings.Set(ctx, "ca_key", "PRIVATE KEY")

	s.UseSecretKey(testKey, []string{"ca_key"})
	if err := s.SealExisting(ctx); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rawSecret(t, s, user), sealedPrefix) || !strings.HasPrefix(rawSetting(t, s, "ca_key"), sealedPrefix) {
		t.Fatal("existing plaintext was not encrypted")
	}
	// Running it again changes nothing.
	before := rawSecret(t, s, user)
	s.SealExisting(ctx)
	if rawSecret(t, s, user) != before {
		t.Error("an already encrypted secret was encrypted again")
	}

	if err := s.UnsealAll(ctx); err != nil {
		t.Fatal(err)
	}
	if rawSecret(t, s, user) != plain || rawSetting(t, s, "ca_key") != "PRIVATE KEY" {
		t.Error("decrypting did not restore the plaintext Django reads")
	}
}

func TestSavingAJoinedUserKeepsTheSecret(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	s.UseSecretKey(testKey, nil)
	_, user := seedUser(t, s, "person@example.com")
	plain := user.MFASecret

	// A device's owner comes from a join and holds the stored form.
	device := &model.Device{Name: "laptop", UserID: user.ID}
	s.Devices.CreateWithinLimit(ctx, device, 5)
	loaded, _ := s.Devices.Get(ctx, device.ID)
	if err := s.Users.Save(ctx, loaded.User); err != nil {
		t.Fatal(err)
	}

	reloaded, _ := s.Users.Get(ctx, user.ID)
	if reloaded.MFASecret != plain {
		t.Error("saving a joined user corrupted the two-factor secret")
	}
}

func TestAWrongKeyIsAnError(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	s.UseSecretKey(testKey, nil)
	seedUser(t, s, "person@example.com")

	s.UseSecretKey([]byte("a different secret key entirely!"), nil)
	if _, err := s.Users.ByEmail(ctx, "person@example.com"); err == nil {
		t.Error("a secret encrypted with another key was read without complaint")
	}
}
