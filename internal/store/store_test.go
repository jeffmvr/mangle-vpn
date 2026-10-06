package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// open returns a store backed by a scratch database.
func open(t *testing.T) *Store {
	t.Helper()

	s, err := Open(filepath.Join(t.TempDir(), "mangle.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// seedGroup stores a group, reusing one that already has the given name.
func seedGroup(t *testing.T, s *Store, name string) *model.Group {
	t.Helper()
	ctx := t.Context()

	if existing, err := s.Groups.ByName(ctx, name); err == nil {
		return existing
	}

	group := &model.Group{Name: name, IsEnabled: true, MaxDevices: 2, MFAEnforced: true}
	if err := s.Groups.Save(ctx, group); err != nil {
		t.Fatalf("Groups.Save: %v", err)
	}
	return group
}

// seedUser stores a user in the default group.
func seedUser(t *testing.T, s *Store, email string) (*model.Group, *model.User) {
	t.Helper()
	ctx := t.Context()

	group := seedGroup(t, s, "Default")

	user := &model.User{Email: email, GroupID: group.ID, IsEnabled: true}
	user.SetPassword("Password1")
	if err := s.Users.Save(ctx, user); err != nil {
		t.Fatalf("Users.Save: %v", err)
	}
	return group, user
}

func TestStorageFormatMatchesDjango(t *testing.T) {
	s := open(t)
	_, user := seedUser(t, s, "person@example.com")

	// Django's UUIDField writes 32 unseparated hex characters on SQLite, and
	// its DateTimeField writes naive UTC text. An existing mangle.db depends
	// on both.
	// CAST defeats the driver's decltype based conversion so that the test
	// sees the bytes that actually land on disk, which is what an existing
	// Django installation will read back.
	var id, createdAt string
	err := s.DB().QueryRow(
		`SELECT CAST(id AS BLOB), CAST(created_at AS BLOB) FROM "users" WHERE email = ?`,
		"person@example.com").Scan(&id, &createdAt)
	if err != nil {
		t.Fatalf("reading raw row: %v", err)
	}

	if len(id) != 32 {
		t.Errorf("stored id = %q (%d chars), want 32 unseparated hex characters", id, len(id))
	}
	if id != user.ID.Hex() {
		t.Errorf("stored id = %q, want %q", id, user.ID.Hex())
	}
	if _, err := time.Parse("2006-01-02 15:04:05.999999", createdAt); err != nil {
		t.Errorf("stored created_at = %q, which Django could not read: %v", createdAt, err)
	}
}

func TestUserRoundTrip(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	group, user := seedUser(t, s, "Person@Example.com")

	// Lookups by e-mail are case insensitive, as they were in Django.
	got, err := s.Users.ByEmail(ctx, "person@EXAMPLE.com")
	if err != nil {
		t.Fatalf("Users.ByEmail: %v", err)
	}
	if got.ID != user.ID {
		t.Fatalf("ByEmail returned %v, want %v", got.ID, user.ID)
	}

	// Every user carries its group, because permissions depend on it.
	if got.Group == nil || got.Group.ID != group.ID {
		t.Fatal("ByEmail did not attach the user's group")
	}
	if !got.IsActive() {
		t.Error("a user in an enabled group should be active")
	}
	if !got.CheckPassword("Password1") || got.CheckPassword("wrong") {
		t.Error("the password did not survive the round trip")
	}

	// Saving without an explicit secret must still leave one in place, so
	// that enrolment works the moment a group starts enforcing it.
	if got.MFASecret == "" {
		t.Error("a saved user has no two-factor secret")
	}
}

func TestUserIsActiveFollowsGroup(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	group, user := seedUser(t, s, "person@example.com")

	group.IsEnabled = false
	if err := s.Groups.Save(ctx, group); err != nil {
		t.Fatalf("Groups.Save: %v", err)
	}

	got, err := s.Users.Get(ctx, user.ID)
	if err != nil {
		t.Fatalf("Users.Get: %v", err)
	}
	if got.IsActive() {
		t.Error("a user in a disabled group must not be active")
	}
}

func TestUserListSearchAndPaging(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	seedUser(t, s, "alice@example.com")

	group, _ := seedUser(t, s, "bob@example.com")
	for _, email := range []string{"carol@example.com", "dave@elsewhere.com"} {
		u := &model.User{Email: email, GroupID: group.ID, IsEnabled: true}
		if err := s.Users.Save(ctx, u); err != nil {
			t.Fatalf("Users.Save: %v", err)
		}
	}

	// Searching spans the user and their group, and the total counts every
	// match rather than just the returned page.
	page, err := s.Users.List(ctx, Query{Search: "example.com", Page: 1, Size: 2}, UserFilter{})
	if err != nil {
		t.Fatalf("Users.List: %v", err)
	}
	if page.Total != 3 {
		t.Errorf("total = %d, want 3", page.Total)
	}
	if len(page.Items) != 2 {
		t.Fatalf("page holds %d users, want 2", len(page.Items))
	}
	// Users are ordered by e-mail address.
	if page.Items[0].Email != "alice@example.com" || page.Items[1].Email != "bob@example.com" {
		t.Errorf("page = %q, %q; want alice then bob", page.Items[0].Email, page.Items[1].Email)
	}

	second, err := s.Users.List(ctx, Query{Search: "example.com", Page: 2, Size: 2}, UserFilter{})
	if err != nil {
		t.Fatalf("Users.List page 2: %v", err)
	}
	if len(second.Items) != 1 || second.Items[0].Email != "carol@example.com" {
		t.Errorf("second page = %v, want just carol", second.Items)
	}
}

func TestDeviceAndClientCascade(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	device := &model.Device{Name: "laptop", UserID: user.ID, Fingerprint: "AA:BB"}
	if err := s.Devices.Save(ctx, device); err != nil {
		t.Fatalf("Devices.Save: %v", err)
	}

	client := &model.Client{
		CommonName: "person@example.com:laptop",
		DeviceID:   device.ID,
		RemoteIP:   "198.51.100.4:1194",
		VirtualIP:  "172.25.0.6",
	}
	if err := s.Clients.Create(ctx, client); err != nil {
		t.Fatalf("Clients.Create: %v", err)
	}

	// A client resolves all the way back to the group, which is what the
	// firewall needs to pick a chain.
	got, err := s.Clients.ByCommonName(ctx, client.CommonName)
	if err != nil {
		t.Fatalf("Clients.ByCommonName: %v", err)
	}
	if got.User() == nil || got.User().ID != user.ID {
		t.Error("client did not resolve its user")
	}
	if got.Group() == nil || got.Group().Name != "Default" {
		t.Error("client did not resolve its group")
	}
	if got.Device.CommonName() != client.CommonName {
		t.Errorf("device common name = %q, want %q", got.Device.CommonName(), client.CommonName)
	}

	// Dependants are torn down by the app layer rather than by SQL, because
	// each one has side effects: a deleted device is added to the revocation
	// list and a deleted client has its firewall rule withdrawn. The store
	// refuses the delete so a missed cascade cannot pass unnoticed.
	if err := s.Devices.Delete(ctx, device.ID); err == nil {
		t.Error("deleting a device out from under its client was allowed")
	}

	if err := s.Clients.Delete(ctx, client.ID); err != nil {
		t.Fatalf("Clients.Delete: %v", err)
	}
	if err := s.Devices.Delete(ctx, device.ID); err != nil {
		t.Fatalf("Devices.Delete after its client: %v", err)
	}
}

func TestReadsDatabaseWrittenByDjango(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	// Rows exactly as the Django release wrote them: identifiers as bare
	// hex, timestamps as naive UTC text, booleans as integers, and a
	// password hashed by Django's PBKDF2 hasher.
	const (
		groupID = "2b4c3f1e8a7d4b6c9e0f1a2b3c4d5e6f"
		userID  = "9f8e7d6c5b4a39281706f5e4d3c2b1a0"
		hash    = "pbkdf2_sha256$120000$NSVqbRsd9Rhl$uRk5YCo0ctJ9lLHb8VEGa9bnh4ISxE6CSgx4g3LC3U8="
	)

	_, err := s.DB().ExecContext(ctx,
		`INSERT INTO "groups" (id, created_at, updated_at, description, is_enabled,
			max_devices, name, mfa_enforced)
		 VALUES (?, '2019-03-08 00:30:12.123456', '2019-03-08 00:30:12.123456',
			'the default group', 1, 3, 'Default', 1)`, groupID)
	if err != nil {
		t.Fatalf("seeding Django group row: %v", err)
	}

	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO "users" (password, last_login, id, created_at, updated_at, email,
			is_admin, is_enabled, name, mfa_enabled, mfa_secret, group_id, mfa_enforced,
			password_change, password_expire)
		 VALUES (?, NULL, ?, '2019-03-08 00:30:12', '2019-03-08 00:30:12',
			'legacy@example.com', 1, 1, 'Legacy User', 1, 'JBSWY3DPEHPK3PXP', ?, NULL, 0, NULL)`,
		hash, userID, groupID)
	if err != nil {
		t.Fatalf("seeding Django user row: %v", err)
	}

	user, err := s.Users.ByEmail(ctx, "legacy@example.com")
	if err != nil {
		t.Fatalf("Users.ByEmail on a Django row: %v", err)
	}

	if user.ID.Hex() != userID {
		t.Errorf("id = %q, want %q", user.ID.Hex(), userID)
	}
	// A timestamp with no fractional part is as valid as one with.
	if want := "2019-03-08 00:30:12"; user.CreatedAt.Format("2006-01-02 15:04:05") != want {
		t.Errorf("created_at = %v, want %s", user.CreatedAt, want)
	}
	if !user.CreatedAt.Equal(user.CreatedAt.UTC()) {
		t.Error("created_at was not read as UTC")
	}
	if user.LastLogin != nil || user.MFAEnforced != nil {
		t.Error("NULL columns were not read as absent")
	}
	if !user.IsAdmin || !user.IsEnabled {
		t.Error("integer booleans were not read as true")
	}
	if !user.CheckPassword("Password1") {
		t.Error("a password hashed by Django no longer verifies")
	}
	if user.Group == nil || user.Group.MaxDevices != 3 {
		t.Error("the joined group did not come back intact")
	}
}

func TestSettingsUpsert(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	if err := s.Settings.Set(ctx, "app_organization", "Mangle"); err != nil {
		t.Fatalf("Settings.Set: %v", err)
	}
	if err := s.Settings.Set(ctx, "app_organization", "Acme"); err != nil {
		t.Fatalf("Settings.Set overwrite: %v", err)
	}

	all, err := s.Settings.All(ctx)
	if err != nil {
		t.Fatalf("Settings.All: %v", err)
	}
	if all["app_organization"] != "Acme" {
		t.Errorf("app_organization = %q, want Acme", all["app_organization"])
	}

	if err := s.Settings.Delete(ctx, "app_organization"); err != nil {
		t.Fatalf("Settings.Delete: %v", err)
	}
	if all, _ := s.Settings.All(ctx); len(all) != 0 {
		t.Errorf("settings remained after delete: %v", all)
	}
}

func TestJobQueueClaimIsExclusive(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	if err := s.Jobs.Enqueue(ctx, "send_email", `{"to":"person@example.com"}`); err != nil {
		t.Fatalf("Jobs.Enqueue: %v", err)
	}

	job, err := s.Jobs.Claim(ctx, time.Minute)
	if err != nil {
		t.Fatalf("Jobs.Claim: %v", err)
	}
	if job.Kind != "send_email" || job.Attempts != 1 {
		t.Errorf("claimed %+v, want kind send_email on attempt 1", job)
	}

	// A claimed job is invisible to the next worker until its claim expires.
	if _, err := s.Jobs.Claim(ctx, time.Minute); err != ErrNotFound {
		t.Errorf("a claimed job was handed out twice: %v", err)
	}
	// Once the claim goes stale the job is picked up again, so work is not
	// lost when a worker dies mid-flight.
	if _, err := s.Jobs.Claim(ctx, 0); err != nil {
		t.Errorf("a stale claim was not released: %v", err)
	}

	if err := s.Jobs.Done(ctx, job.ID); err != nil {
		t.Fatalf("Jobs.Done: %v", err)
	}
	if _, err := s.Jobs.Claim(ctx, 0); err != ErrNotFound {
		t.Error("a finished job stayed in the queue")
	}
}

func TestOpenIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mangle.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	group := &model.Group{Name: "Default", IsEnabled: true, MaxDevices: 1}
	if err := first.Groups.Save(context.Background(), group); err != nil {
		t.Fatalf("Groups.Save: %v", err)
	}
	first.Close()

	// Reopening an existing database must leave its contents alone.
	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer second.Close()

	if _, err := second.Groups.ByName(context.Background(), "Default"); err != nil {
		t.Errorf("reopening the database lost its data: %v", err)
	}
}

func TestDeviceOSRoundTripAndDjangoInsert(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	device := &model.Device{Name: "laptop", OS: "macos", UserID: user.ID}
	if err := s.Devices.Save(ctx, device); err != nil {
		t.Fatalf("Devices.Save: %v", err)
	}
	got, err := s.Devices.Get(ctx, device.ID)
	if err != nil {
		t.Fatalf("Devices.Get: %v", err)
	}
	if got.OS != "macos" {
		t.Errorf("os = %q, want macos", got.OS)
	}

	// The Django release does not know the column, so its inserts leave it
	// out; the default must accept them and read back as unknown.
	_, err = s.DB().ExecContext(ctx,
		`INSERT INTO "devices" (id, created_at, updated_at, fingerprint, last_login,
			name, serial, user_id)
		 VALUES ('0123456789abcdef0123456789abcdef', '2019-03-08 00:30:12',
			'2019-03-08 00:30:12', 'CC:DD', NULL, 'desktop', '', ?)`, user.ID)
	if err != nil {
		t.Fatalf("inserting a device the way Django does: %v", err)
	}
	devices, err := s.Devices.ByUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("Devices.ByUser: %v", err)
	}
	for _, d := range devices {
		if d.Name == "desktop" && d.OS != "" {
			t.Errorf("os = %q for a Django-written row, want empty", d.OS)
		}
	}
}

func TestUserListFilters(t *testing.T) {
	s := open(t)
	ctx := t.Context()

	group, _ := seedUser(t, s, "member@example.com")
	admin := &model.User{Email: "admin@example.com", GroupID: group.ID, IsAdmin: true, IsEnabled: true}
	disabled := &model.User{Email: "gone@example.com", GroupID: group.ID}
	for _, u := range []*model.User{admin, disabled} {
		if err := s.Users.Save(ctx, u); err != nil {
			t.Fatalf("Users.Save: %v", err)
		}
	}

	yes, no := true, false
	emails := func(f UserFilter, search string) []string {
		t.Helper()
		page, err := s.Users.List(ctx, Query{Search: search, Page: 1, Size: 10}, f)
		if err != nil {
			t.Fatalf("Users.List: %v", err)
		}
		var out []string
		for _, u := range page.Items {
			out = append(out, u.Email)
		}
		if page.Total != len(out) {
			t.Errorf("total = %d, but %d users came back", page.Total, len(out))
		}
		return out
	}

	if got := emails(UserFilter{Admin: &yes}, ""); len(got) != 1 || got[0] != "admin@example.com" {
		t.Errorf("admins = %q, want just admin@", got)
	}
	if got := emails(UserFilter{Enabled: &no}, ""); len(got) != 1 || got[0] != "gone@example.com" {
		t.Errorf("disabled = %q, want just gone@", got)
	}
	if got := emails(UserFilter{Admin: &no, Enabled: &yes}, ""); len(got) != 1 || got[0] != "member@example.com" {
		t.Errorf("enabled non-admins = %q, want just member@", got)
	}
	// Filters combine with the search rather than replacing it.
	if got := emails(UserFilter{Enabled: &yes}, "admin"); len(got) != 1 || got[0] != "admin@example.com" {
		t.Errorf("enabled matching admin = %q, want just admin@", got)
	}
}

func TestClaimMFAStepRefusesReuse(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	for _, tc := range []struct {
		step int64
		want bool
	}{
		{100, true},  // first use of a code
		{100, false}, // the same code again
		{99, false},  // an older code
		{101, true},  // the next code
	} {
		got, err := s.Users.ClaimMFAStep(ctx, user, tc.step)
		if err != nil {
			t.Fatalf("ClaimMFAStep: %v", err)
		}
		if got != tc.want {
			t.Errorf("ClaimMFAStep(%d) = %v, want %v", tc.step, got, tc.want)
		}
	}

	reloaded, _ := s.Users.Get(ctx, user.ID)
	if reloaded.MFALastStep != 101 {
		t.Errorf("mfa_last_step = %d, want 101", reloaded.MFALastStep)
	}
}

func TestRecordFailedLoginLocksAtTheLimit(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	for i := 1; i <= 3; i++ {
		lockedNow, err := s.Users.RecordFailedLogin(ctx, user, 3, time.Hour)
		if err != nil {
			t.Fatalf("RecordFailedLogin: %v", err)
		}
		reloaded, _ := s.Users.Get(ctx, user.ID)
		locked := reloaded.IsLocked(time.Now())
		if want := i == 3; locked != want || lockedNow != want {
			t.Errorf("after %d failures locked = %v (reported %v), want %v", i, locked, lockedNow, want)
		}
	}

	if err := s.Users.ClearFailedLogins(ctx, user); err != nil {
		t.Fatalf("ClearFailedLogins: %v", err)
	}
	reloaded, _ := s.Users.Get(ctx, user.ID)
	if reloaded.IsLocked(time.Now()) || reloaded.FailedLogins != 0 {
		t.Error("clearing the failures did not unlock the account")
	}
}

func TestSaveDoesNotOverwriteSignInCounters(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	// A copy loaded before the failures, saved after them, as an
	// administrator's edit racing a sign in would be.
	stale, _ := s.Users.Get(ctx, user.ID)
	s.Users.RecordFailedLogin(ctx, user, 1, time.Hour)
	stale.Name = "Edited"
	if err := s.Users.Save(ctx, stale); err != nil {
		t.Fatalf("Users.Save: %v", err)
	}

	reloaded, _ := s.Users.Get(ctx, user.ID)
	if !reloaded.IsLocked(time.Now()) {
		t.Error("saving a stale copy of the user cleared their lockout")
	}
}

func TestCreateWithinLimit(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	for i, want := range []bool{true, true, false} {
		device := &model.Device{Name: "device", UserID: user.ID}
		created, err := s.Devices.CreateWithinLimit(ctx, device, 2)
		if err != nil {
			t.Fatalf("CreateWithinLimit: %v", err)
		}
		if created != want {
			t.Errorf("device %d created = %v, want %v", i+1, created, want)
		}
		if created && device.ID.IsZero() {
			t.Error("a created device was given no ID")
		}
	}
}

func TestClaimCertificateOnlyOnce(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	device := &model.Device{Name: "laptop", UserID: user.ID}
	if _, err := s.Devices.CreateWithinLimit(ctx, device, 5); err != nil {
		t.Fatalf("CreateWithinLimit: %v", err)
	}

	first, err := s.Devices.ClaimCertificate(ctx, device, "AA", "1")
	if err != nil || !first {
		t.Fatalf("first claim = %v, %v; want true", first, err)
	}
	second, err := s.Devices.ClaimCertificate(ctx, &model.Device{Base: device.Base}, "BB", "2")
	if err != nil || second {
		t.Fatalf("second claim = %v, %v; want false", second, err)
	}

	reloaded, _ := s.Devices.Get(ctx, device.ID)
	if reloaded.Fingerprint != "AA" || reloaded.Serial != "1" {
		t.Errorf("certificate = %s/%s, want the first claim's AA/1", reloaded.Fingerprint, reloaded.Serial)
	}
}

func TestImportTokenIsSingleUseAndLapses(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	device := &model.Device{Name: "laptop", UserID: user.ID}
	if _, err := s.Devices.CreateWithinLimit(ctx, device, 5); err != nil {
		t.Fatal(err)
	}

	s.Devices.SetImportToken(ctx, device.ID, "live", time.Now().Add(time.Minute))
	if id, err := s.Devices.TakeImportToken(ctx, "live"); err != nil || id != device.ID {
		t.Fatalf("TakeImportToken = %v, %v; want the device", id, err)
	}
	if _, err := s.Devices.TakeImportToken(ctx, "live"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second take = %v, want ErrNotFound", err)
	}

	s.Devices.SetImportToken(ctx, device.ID, "stale", time.Now().Add(-time.Second))
	if _, err := s.Devices.TakeImportToken(ctx, "stale"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired take = %v, want ErrNotFound", err)
	}
	if _, err := s.Devices.TakeImportToken(ctx, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("empty take = %v, want ErrNotFound", err)
	}
}

func TestDeleteUserRemovesEverythingTheyOwn(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")
	_, other := seedUser(t, s, "other@example.com")

	// Everything a user who has signed in and connected leaves behind.
	if err := s.Events.Record(ctx, user.ID, "web.login", "signed in"); err != nil {
		t.Fatal(err)
	}
	device := &model.Device{Name: "laptop", UserID: user.ID, Fingerprint: "AA", Serial: "1234"}
	if err := s.Devices.Save(ctx, device); err != nil {
		t.Fatal(err)
	}
	if err := s.Clients.Create(ctx, &model.Client{CommonName: "person@example.com:laptop",
		DeviceID: device.ID, VirtualIP: "10.8.0.6"}); err != nil {
		t.Fatal(err)
	}
	s.Events.Record(ctx, other.ID, "web.login", "someone else")

	if err := s.Users.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Users.Delete: %v", err)
	}

	if _, err := s.Users.Get(ctx, user.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("the user is still there: %v", err)
	}
	var left int
	s.DB().QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM events WHERE user_id = ?) +
		(SELECT COUNT(*) FROM devices WHERE user_id = ?) +
		(SELECT COUNT(*) FROM clients)`, user.ID, user.ID).Scan(&left)
	if left != 0 {
		t.Errorf("%d events, devices or clients were left behind", left)
	}

	serials, _ := s.RevokedDevices.Serials(ctx)
	if len(serials) != 1 || serials[0] != "1234" {
		t.Errorf("revoked serials = %v, want the deleted device's", serials)
	}
	var id string
	s.DB().QueryRowContext(ctx, `SELECT id FROM revoked_devices`).Scan(&id)
	if len(id) != 32 {
		t.Errorf("revocation id %q is not in Django's 32 hex digit form", id)
	}

	// Someone else's history is untouched.
	var others int
	s.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE user_id = ?`, other.ID).Scan(&others)
	if others != 1 {
		t.Errorf("another user's events = %d, want 1", others)
	}
}

func TestDeleteGroupTakesItsRules(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	group := seedGroup(t, s, "Contractors")
	rule := &model.FirewallRule{GroupID: group.ID, Action: model.ActionAccept, Destination: "10.0.0.0/8"}
	if err := s.FirewallRules.Save(ctx, rule); err != nil {
		t.Fatal(err)
	}

	if err := s.Groups.Delete(ctx, group.ID); err != nil {
		t.Fatalf("Groups.Delete: %v", err)
	}
	if rules, _ := s.FirewallRules.ByGroup(ctx, group.ID); len(rules) != 0 {
		t.Errorf("%d rules were left behind", len(rules))
	}
}

func TestSelfServiceUpdatesKeepAnAdminsChange(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	// The user's request loaded them before an administrator disabled them.
	stale, _ := s.Users.Get(ctx, user.ID)
	user.IsEnabled = false
	if err := s.Users.Save(ctx, user); err != nil {
		t.Fatal(err)
	}

	stale.SetPassword("Password2")
	stale.MFAEnabled = true
	stale.Name = "New name"
	for name, update := range map[string]func(context.Context, *model.User) error{
		"SetPassword": s.Users.SetPassword, "SetMFAEnabled": s.Users.SetMFAEnabled, "SetName": s.Users.SetName,
	} {
		if err := update(ctx, stale); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
	}

	reloaded, _ := s.Users.Get(ctx, user.ID)
	if reloaded.IsEnabled {
		t.Error("a self-service update re-enabled a user an administrator disabled")
	}
	if !reloaded.CheckPassword("Password2") || !reloaded.MFAEnabled || reloaded.Name != "New name" {
		t.Error("the self-service updates were not stored")
	}
}

func TestDeviceSaveLeavesTheCertificateAlone(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	device := &model.Device{Name: "laptop", UserID: user.ID}
	s.Devices.CreateWithinLimit(ctx, device, 5)
	stale, _ := s.Devices.Get(ctx, device.ID)

	if ok, _ := s.Devices.ClaimCertificate(ctx, device, "AA", "1"); !ok {
		t.Fatal("ClaimCertificate failed")
	}
	stale.Name = "renamed"
	if err := s.Devices.Save(ctx, stale); err != nil {
		t.Fatal(err)
	}
	if err := s.Devices.TouchLastLogin(ctx, stale); err != nil {
		t.Fatal(err)
	}

	reloaded, _ := s.Devices.Get(ctx, device.ID)
	if reloaded.Fingerprint != "AA" || reloaded.Serial != "1" {
		t.Errorf("a stale save cleared the certificate: %q/%q", reloaded.Fingerprint, reloaded.Serial)
	}
	if reloaded.Name != "renamed" || reloaded.LastLogin == nil {
		t.Error("the rename or the connection time was not stored")
	}
}

func TestDeleteEventsOlderThan(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")

	s.Events.Record(ctx, user.ID, "web.login", "recent")
	s.DB().ExecContext(ctx, `INSERT INTO events (id, created_at, updated_at, detail, name, user_id)
		VALUES ('0123456789abcdef0123456789abcdef', '2020-01-01 00:00:00', '2020-01-01 00:00:00', 'old', 'web.login', ?)`, user.ID)

	removed, err := s.Events.DeleteOlderThan(ctx, time.Now().AddDate(0, 0, -90))
	if err != nil || removed != 1 {
		t.Fatalf("DeleteOlderThan = %d, %v; want 1 removed", removed, err)
	}
	page, _ := s.Events.List(ctx, Query{}, EventFilter{})
	if len(page.Items) != 1 || page.Items[0].Detail != "recent" {
		t.Errorf("events left = %v, want just the recent one", page.Items)
	}
}

func TestGroupCounts(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	group, _ := seedUser(t, s, "one@example.com")
	seedUser(t, s, "two@example.com")
	empty := seedGroup(t, s, "Empty")
	rule := &model.FirewallRule{GroupID: group.ID, Action: model.ActionAccept, Destination: "10.0.0.0/8"}
	if err := s.FirewallRules.Save(ctx, rule); err != nil {
		t.Fatal(err)
	}

	counts, err := s.Groups.Counts(ctx)
	if err != nil {
		t.Fatalf("Groups.Counts: %v", err)
	}
	if got := counts[group.ID]; got != (GroupCounts{Members: 2, Rules: 1}) {
		t.Errorf("Default = %+v, want 2 members and 1 rule", got)
	}
	if got := counts[empty.ID]; got != (GroupCounts{}) {
		t.Errorf("Empty = %+v, want none", got)
	}
}

func TestTrafficByDevice(t *testing.T) {
	s := open(t)
	ctx := t.Context()
	_, user := seedUser(t, s, "person@example.com")
	_, other := seedUser(t, s, "other@example.com")

	laptop := &model.Device{Name: "laptop", UserID: user.ID}
	phone := &model.Device{Name: "phone", UserID: user.ID}
	theirs := &model.Device{Name: "laptop", UserID: other.ID}
	for _, device := range []*model.Device{laptop, phone, theirs} {
		if err := s.Devices.Save(ctx, device); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Now()
	for _, session := range []*model.VPNSession{
		{DeviceID: laptop.ID, UserID: user.ID, BytesReceived: 100, BytesSent: 1000},
		{DeviceID: laptop.ID, UserID: user.ID, BytesReceived: 5, BytesSent: 50},
		{DeviceID: theirs.ID, UserID: other.ID, BytesReceived: 9999},
	} {
		session.StartedAt, session.EndedAt = now.Add(-time.Hour), now
		if err := s.VPNSessions.Record(ctx, session); err != nil {
			t.Fatal(err)
		}
	}

	traffic, err := s.VPNSessions.TrafficByDevice(ctx, user.ID)
	if err != nil {
		t.Fatalf("TrafficByDevice: %v", err)
	}
	if traffic[laptop.ID] != 1155 || traffic[phone.ID] != 0 || len(traffic) != 1 {
		t.Errorf("traffic = %v, want only the laptop at 1155 bytes", traffic)
	}
}
