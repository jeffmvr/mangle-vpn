package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// SettingStore reads and writes application settings, which are stored as
// name and value text pairs.
type SettingStore struct{ s *Store }

// All returns every setting as a name to value map.
func (ss *SettingStore) All(ctx context.Context) (map[string]string, error) {
	rows, err := ss.s.db.QueryContext(ctx, `SELECT name, value FROM "setting" ORDER BY name`)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, translate(err)
		}
		if value, err = ss.s.unseal(value); err != nil {
			return nil, fmt.Errorf("store: setting %s: %w", name, err)
		}
		settings[name] = value
	}
	return settings, translate(rows.Err())
}

// Set stores the value of a setting, creating it when it does not exist.
func (ss *SettingStore) Set(ctx context.Context, name, value string) error {
	now := formatTime(time.Now().UTC())
	if ss.s.isSecretSetting(name) {
		value = ss.s.seal(value)
	}

	_, err := ss.s.db.ExecContext(ctx,
		`INSERT INTO "setting" (id, created_at, updated_at, name, value)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		uuid.New(), now, now, name, value)
	return translate(err)
}

// Delete removes a setting.
func (ss *SettingStore) Delete(ctx context.Context, name string) error {
	_, err := ss.s.db.ExecContext(ctx,
		`DELETE FROM "setting" WHERE name = ? COLLATE NOCASE`, name)
	return translate(err)
}
