package store

import (
	"context"
	"database/sql"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// groupColumns lists the group columns in the order groupScan expects.
const groupColumns = `g.id, g.created_at, g.updated_at, g.description, g.is_enabled,
	g.max_devices, g.mfa_enforced, g.name, g.routes, g.nameservers, g.device_idle_days`

// newGroupScan allocates a scanner for groupColumns.
func newGroupScan() entityScan[*model.Group] { return &groupScan{} }

// GroupStore reads and writes groups.
type GroupStore struct{ s *Store }

// Get returns the group with the given ID.
func (gs *GroupStore) Get(ctx context.Context, id uuid.UUID) (*model.Group, error) {
	return queryOne(gs.s.db.QueryRowContext(ctx,
		`SELECT `+groupColumns+` FROM "groups" g WHERE g.id = ?`, id), &groupScan{})
}

// ByName returns the group with the given name, matched case insensitively.
func (gs *GroupStore) ByName(ctx context.Context, name string) (*model.Group, error) {
	return queryOne(gs.s.db.QueryRowContext(ctx,
		`SELECT `+groupColumns+` FROM "groups" g WHERE g.name = ? COLLATE NOCASE`, name), &groupScan{})
}

// All returns every group, ordered by name.
func (gs *GroupStore) All(ctx context.Context) ([]*model.Group, error) {
	rows, err := gs.s.db.QueryContext(ctx,
		`SELECT `+groupColumns+` FROM "groups" g ORDER BY g.name`)
	return queryAll(rows, err, newGroupScan)
}

// Enabled returns every enabled group, ordered by name.
func (gs *GroupStore) Enabled(ctx context.Context) ([]*model.Group, error) {
	rows, err := gs.s.db.QueryContext(ctx,
		`SELECT `+groupColumns+` FROM "groups" g WHERE g.is_enabled = 1 ORDER BY g.name`)
	return queryAll(rows, err, newGroupScan)
}

// List returns the groups matching the query, ordered by name.
func (gs *GroupStore) List(ctx context.Context, q Query) (Page[*model.Group], error) {
	where, args := searchClause(q.Search, "g.name")
	if where != "" {
		where = " WHERE " + where
	}

	var page Page[*model.Group]
	if err := gs.s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM "groups" g`+where, args...).Scan(&page.Total); err != nil {
		return page, translate(err)
	}

	rows, err := gs.s.db.QueryContext(ctx,
		`SELECT `+groupColumns+` FROM "groups" g`+where+` ORDER BY g.name`+q.limitOffset(), args...)

	page.Items, err = queryAll(rows, err, newGroupScan)
	return page, err
}

// GroupCounts is how many members and firewall rules a group has.
type GroupCounts struct {
	Members int
	Rules   int
}

// Counts returns the member and rule counts of every group, by ID.
func (gs *GroupStore) Counts(ctx context.Context) (map[uuid.UUID]GroupCounts, error) {
	rows, err := gs.s.db.QueryContext(ctx,
		`SELECT g.id,
			(SELECT COUNT(*) FROM "users" u WHERE u.group_id = g.id),
			(SELECT COUNT(*) FROM "firewall_rules" r WHERE r.group_id = g.id)
		 FROM "groups" g`)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	counts := map[uuid.UUID]GroupCounts{}
	for rows.Next() {
		var id uuid.UUID
		var c GroupCounts
		if err := rows.Scan(&id, &c.Members, &c.Rules); err != nil {
			return nil, translate(err)
		}
		counts[id] = c
	}
	return counts, translate(rows.Err())
}

// Exists reports whether a group with the given ID exists.
func (gs *GroupStore) Exists(ctx context.Context, id uuid.UUID) bool {
	var one int
	err := gs.s.db.QueryRowContext(ctx, `SELECT 1 FROM "groups" WHERE id = ?`, id).Scan(&one)
	return err == nil
}

// Save inserts or updates the group, assigning its identity and timestamps
// on first write.
func (gs *GroupStore) Save(ctx context.Context, g *model.Group) error {
	now := time.Now().UTC()
	g.UpdatedAt = now

	if g.ID.IsZero() {
		g.ID, g.CreatedAt = uuid.New(), now

		_, err := gs.s.db.ExecContext(ctx,
			`INSERT INTO "groups" (id, created_at, updated_at, description, is_enabled,
				max_devices, mfa_enforced, name, routes, nameservers, device_idle_days)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			g.ID, formatTime(g.CreatedAt), formatTime(g.UpdatedAt), g.Description,
			g.IsEnabled, g.MaxDevices, g.MFAEnforced, g.Name, g.Routes, g.Nameservers,
			g.DeviceIdleDays)
		return translate(err)
	}

	_, err := gs.s.db.ExecContext(ctx,
		`UPDATE "groups" SET updated_at = ?, description = ?, is_enabled = ?,
			max_devices = ?, mfa_enforced = ?, name = ?, routes = ?, nameservers = ?,
			device_idle_days = ? WHERE id = ?`,
		formatTime(g.UpdatedAt), g.Description, g.IsEnabled, g.MaxDevices,
		g.MFAEnforced, g.Name, g.Routes, g.Nameservers, g.DeviceIdleDays, g.ID)
	return translate(err)
}

// Delete removes the group.
//
// Its firewall rules go with it, in the same transaction. Its members must
// already have been deleted: they hold devices and certificates that need
// more than a row removed.
func (gs *GroupStore) Delete(ctx context.Context, id uuid.UUID) error {
	return gs.s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM "firewall_rules" WHERE group_id = ?`, id); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM "groups" WHERE id = ?`, id)
		return err
	})
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }
