package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// deviceColumns lists the device columns followed by those of its owner, in
// the order scanDevice expects.
const deviceColumns = `d.id, d.created_at, d.updated_at, d.fingerprint, d.last_login,
	d.name, d.os, d.serial, d.user_id, d.static_ip, ` + userColumns

// deviceJoin attaches the owning user and their group to every device.
const deviceJoin = ` FROM "devices" d
	JOIN "users" u ON u.id = d.user_id
	JOIN "groups" g ON g.id = u.group_id`

// newDeviceScan allocates a scanner for deviceColumns.
func newDeviceScan() entityScan[*model.Device] { return &deviceScan{} }

// DeviceStore reads and writes devices.
type DeviceStore struct{ s *Store }

// Get returns the device with the given ID.
func (ds *DeviceStore) Get(ctx context.Context, id uuid.UUID) (*model.Device, error) {
	return queryOne(ds.s.db.QueryRowContext(ctx,
		`SELECT `+deviceColumns+deviceJoin+` WHERE d.id = ?`, id), &deviceScan{})
}

// ByFingerprint returns the device with the given certificate fingerprint.
func (ds *DeviceStore) ByFingerprint(ctx context.Context, fingerprint string) (*model.Device, error) {
	return queryOne(ds.s.db.QueryRowContext(ctx,
		`SELECT `+deviceColumns+deviceJoin+` WHERE d.fingerprint = ? COLLATE NOCASE`,
		fingerprint), &deviceScan{})
}

// ByUser returns every device belonging to the user, oldest first.
func (ds *DeviceStore) ByUser(ctx context.Context, userID uuid.UUID) ([]*model.Device, error) {
	rows, err := ds.s.db.QueryContext(ctx,
		`SELECT `+deviceColumns+deviceJoin+` WHERE d.user_id = ? ORDER BY d.created_at`, userID)
	return queryAll(rows, err, newDeviceScan)
}

// Save inserts or updates the device.
func (ds *DeviceStore) Save(ctx context.Context, d *model.Device) error {
	now := time.Now().UTC()
	d.UpdatedAt = now

	if d.ID.IsZero() {
		d.ID, d.CreatedAt = uuid.New(), now

		_, err := ds.s.db.ExecContext(ctx,
			`INSERT INTO "devices" (id, created_at, updated_at, fingerprint, last_login,
				name, os, serial, user_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.ID, formatTime(d.CreatedAt), formatTime(d.UpdatedAt), d.Fingerprint,
			nullTime(d.LastLogin), d.Name, d.OS, d.Serial, d.UserID)
		return translate(err)
	}

	// The certificate is written only by ClaimCertificate, which makes a
	// download happen once, and the last connection only by
	// TouchLastLogin, so that neither can be put back by a stale copy.
	_, err := ds.s.db.ExecContext(ctx,
		`UPDATE "devices" SET updated_at = ?, name = ?, os = ? WHERE id = ?`,
		formatTime(d.UpdatedAt), d.Name, d.OS, d.ID)
	return translate(err)
}

// CreateWithinLimit inserts a new device unless its user already has limit
// devices, reporting whether it was inserted. Counting and inserting in one
// statement means requests racing each other cannot overshoot the limit.
func (ds *DeviceStore) CreateWithinLimit(ctx context.Context, d *model.Device, limit int) (bool, error) {
	now := time.Now().UTC()
	id := uuid.New()

	result, err := ds.s.db.ExecContext(ctx,
		`INSERT INTO "devices" (id, created_at, updated_at, fingerprint, last_login,
			name, os, serial, user_id)
		 SELECT ?, ?, ?, '', NULL, ?, ?, '', ?
		 WHERE (SELECT COUNT(*) FROM "devices" WHERE user_id = ?) < ?`,
		id, formatTime(now), formatTime(now), d.Name, d.OS, d.UserID, d.UserID, limit)
	if err != nil {
		return false, translate(err)
	}
	inserted, err := result.RowsAffected()
	if err != nil || inserted != 1 {
		return false, translate(err)
	}

	d.ID, d.CreatedAt, d.UpdatedAt = id, now, now
	return true, nil
}

// ClaimCertificate records the certificate issued for a device, unless one
// already was, reporting whether this one was recorded. It is what makes a
// device's configuration downloadable exactly once, even under concurrent
// requests.
func (ds *DeviceStore) ClaimCertificate(ctx context.Context, d *model.Device, fingerprint, serial string) (bool, error) {
	now := time.Now().UTC()

	result, err := ds.s.db.ExecContext(ctx,
		`UPDATE "devices" SET updated_at = ?, fingerprint = ?, serial = ?
		 WHERE id = ? AND fingerprint = '' AND serial = ''`,
		formatTime(now), fingerprint, serial, d.ID)
	if err != nil {
		return false, translate(err)
	}
	claimed, err := result.RowsAffected()
	if err != nil || claimed != 1 {
		return false, translate(err)
	}

	d.UpdatedAt, d.Fingerprint, d.Serial = now, fingerprint, serial
	return true, nil
}

// IdleInGroup returns the devices of the group's members that have not
// connected since cutoff, counting from when they were made for a device
// that never has, and that are not connected now.
func (ds *DeviceStore) IdleInGroup(ctx context.Context, groupID uuid.UUID, cutoff time.Time) ([]*model.Device, error) {
	rows, err := ds.s.db.QueryContext(ctx,
		`SELECT `+deviceColumns+deviceJoin+`
		 WHERE u.group_id = ? AND COALESCE(d.last_login, d.created_at) < ?
		   AND NOT EXISTS (SELECT 1 FROM "clients" c WHERE c.device_id = d.id)
		 ORDER BY d.created_at`,
		groupID, formatTime(cutoff.UTC()))
	return queryAll(rows, err, newDeviceScan)
}

// SetStaticIP gives the device a fixed VPN address, or takes it away when
// address is empty. ErrConflict means another device already has it.
func (ds *DeviceStore) SetStaticIP(ctx context.Context, d *model.Device, address string) error {
	now := time.Now().UTC()
	_, err := ds.s.db.ExecContext(ctx,
		`UPDATE "devices" SET updated_at = ?, static_ip = ? WHERE id = ?`,
		formatTime(now), address, d.ID)
	if err != nil {
		return translate(err)
	}
	d.UpdatedAt, d.StaticIP = now, address
	return nil
}

// StaticIPs returns every fixed address in use.
func (ds *DeviceStore) StaticIPs(ctx context.Context) (map[string]bool, error) {
	rows, err := ds.s.db.QueryContext(ctx, `SELECT static_ip FROM "devices" WHERE static_ip != ''`)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	used := map[string]bool{}
	for rows.Next() {
		var address string
		if err := rows.Scan(&address); err != nil {
			return nil, translate(err)
		}
		used[address] = true
	}
	return used, translate(rows.Err())
}

// TouchLastLogin records that the device has just connected.
func (ds *DeviceStore) TouchLastLogin(ctx context.Context, d *model.Device) error {
	now := time.Now().UTC()
	d.LastLogin = &now
	_, err := ds.s.db.ExecContext(ctx,
		`UPDATE "devices" SET last_login = ? WHERE id = ?`, formatTime(now), d.ID)
	return translate(err)
}

// SetImportToken records the hash of a device's import code and when it
// lapses, replacing any earlier code.
func (ds *DeviceStore) SetImportToken(ctx context.Context, id uuid.UUID, hash string, expires time.Time) error {
	_, err := ds.s.db.ExecContext(ctx,
		`UPDATE "devices" SET import_token = ?, import_expires = ? WHERE id = ?`,
		hash, formatTime(expires.UTC()), id)
	return translate(err)
}

// TakeImportToken spends the import code with the given hash, returning the
// device it belongs to, or ErrNotFound when no unexpired code matches. The
// code is cleared in the same statement that finds it, so it works once.
func (ds *DeviceStore) TakeImportToken(ctx context.Context, hash string) (uuid.UUID, error) {
	var id uuid.UUID
	err := ds.s.db.QueryRowContext(ctx,
		`UPDATE "devices" SET import_token = '', import_expires = NULL
		 WHERE import_token = ? AND import_token != '' AND import_expires > ?
		 RETURNING id`,
		hash, formatTime(time.Now().UTC())).Scan(&id)
	return id, translate(err)
}

// Delete removes the device and its connection history.
func (ds *DeviceStore) Delete(ctx context.Context, id uuid.UUID) error {
	return ds.s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM "vpn_sessions" WHERE device_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM "devices" WHERE id = ?`, id)
		return err
	})
}

// RevokedDeviceStore records the certificate serials of deleted devices.
type RevokedDeviceStore struct{ s *Store }

// Create records a revoked certificate serial.
func (rs *RevokedDeviceStore) Create(ctx context.Context, serial string) error {
	now := time.Now().UTC()

	_, err := rs.s.db.ExecContext(ctx,
		`INSERT INTO "revoked_devices" (id, created_at, updated_at, serial)
		 VALUES (?, ?, ?, ?)`,
		uuid.New(), formatTime(now), formatTime(now), serial)
	return translate(err)
}

// Serials returns every revoked certificate serial, oldest first.
func (rs *RevokedDeviceStore) Serials(ctx context.Context) ([]string, error) {
	rows, err := rs.s.db.QueryContext(ctx,
		`SELECT serial FROM "revoked_devices" ORDER BY created_at`)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	var serials []string
	for rows.Next() {
		var serial string
		if err := rows.Scan(&serial); err != nil {
			return nil, translate(err)
		}
		serials = append(serials, serial)
	}
	return serials, translate(rows.Err())
}
