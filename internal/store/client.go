package store

import (
	"context"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// clientColumns lists the client columns followed by those of the connected
// device, its owner, and their group, in the order clientScan expects.
const clientColumns = `c.id, c.created_at, c.updated_at, c.common_name, c.device_id,
	c.platform, c.remote_ip, c.virtual_ip, ` + deviceColumns

// clientJoin attaches the device, user, and group behind every connection.
const clientJoin = ` FROM "clients" c
	JOIN "devices" d ON d.id = c.device_id
	JOIN "users" u ON u.id = d.user_id
	JOIN "groups" g ON g.id = u.group_id`

// clientSearchColumns are the columns an administrator's search looks in.
var clientSearchColumns = []string{
	"c.remote_ip", "c.virtual_ip", "d.name", "u.email", "u.name", "g.name",
}

// newClientScan allocates a scanner for clientColumns.
func newClientScan() entityScan[*model.Client] { return &clientScan{} }

// ClientStore reads and writes the record of currently connected clients.
type ClientStore struct{ s *Store }

// Get returns the client with the given ID.
func (cs *ClientStore) Get(ctx context.Context, id uuid.UUID) (*model.Client, error) {
	return queryOne(cs.s.db.QueryRowContext(ctx,
		`SELECT `+clientColumns+clientJoin+` WHERE c.id = ?`, id), &clientScan{})
}

// ByCommonName returns the client with the given certificate common name.
func (cs *ClientStore) ByCommonName(ctx context.Context, name string) (*model.Client, error) {
	return queryOne(cs.s.db.QueryRowContext(ctx,
		`SELECT `+clientColumns+clientJoin+` WHERE c.common_name = ?`, name), &clientScan{})
}

// ByDevice returns the connection of the given device.
func (cs *ClientStore) ByDevice(ctx context.Context, deviceID uuid.UUID) (*model.Client, error) {
	return queryOne(cs.s.db.QueryRowContext(ctx,
		`SELECT `+clientColumns+clientJoin+` WHERE c.device_id = ?`, deviceID), &clientScan{})
}

// ByUser returns every client belonging to the user.
func (cs *ClientStore) ByUser(ctx context.Context, userID uuid.UUID) ([]*model.Client, error) {
	rows, err := cs.s.db.QueryContext(ctx,
		`SELECT `+clientColumns+clientJoin+` WHERE u.id = ? ORDER BY c.virtual_ip`, userID)
	return queryAll(rows, err, newClientScan)
}

// ByGroup returns every client whose user belongs to the group.
func (cs *ClientStore) ByGroup(ctx context.Context, groupID uuid.UUID) ([]*model.Client, error) {
	rows, err := cs.s.db.QueryContext(ctx,
		`SELECT `+clientColumns+clientJoin+` WHERE u.group_id = ? ORDER BY c.virtual_ip`, groupID)
	return queryAll(rows, err, newClientScan)
}

// All returns every connected client.
func (cs *ClientStore) All(ctx context.Context) ([]*model.Client, error) {
	rows, err := cs.s.db.QueryContext(ctx,
		`SELECT `+clientColumns+clientJoin+` ORDER BY c.virtual_ip`)
	return queryAll(rows, err, newClientScan)
}

// List returns the clients matching the query, ordered by virtual address.
func (cs *ClientStore) List(ctx context.Context, q Query) (Page[*model.Client], error) {
	where, args := searchClause(q.Search, clientSearchColumns...)
	if where != "" {
		where = " WHERE " + where
	}

	var page Page[*model.Client]
	if err := cs.s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)`+clientJoin+where, args...).Scan(&page.Total); err != nil {
		return page, translate(err)
	}

	rows, err := cs.s.db.QueryContext(ctx,
		`SELECT `+clientColumns+clientJoin+where+` ORDER BY c.virtual_ip`+q.limitOffset(), args...)

	page.Items, err = queryAll(rows, err, newClientScan)
	return page, err
}

// Create records a newly connected client.
func (cs *ClientStore) Create(ctx context.Context, c *model.Client) error {
	now := time.Now().UTC()
	c.ID, c.CreatedAt, c.UpdatedAt = uuid.New(), now, now

	_, err := cs.s.db.ExecContext(ctx,
		`INSERT INTO "clients" (id, created_at, updated_at, common_name, device_id,
			platform, remote_ip, virtual_ip) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, formatTime(c.CreatedAt), formatTime(c.UpdatedAt), c.CommonName,
		c.DeviceID, c.Platform, c.RemoteIP, c.VirtualIP)
	return translate(err)
}

// Delete removes the client.
func (cs *ClientStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := cs.s.db.ExecContext(ctx, `DELETE FROM "clients" WHERE id = ?`, id)
	return translate(err)
}

// DeleteAll removes every client. It runs when the VPN server stops, so that
// a dirty restart cannot leave stale connections behind.
func (cs *ClientStore) DeleteAll(ctx context.Context) error {
	_, err := cs.s.db.ExecContext(ctx, `DELETE FROM "clients"`)
	return translate(err)
}
