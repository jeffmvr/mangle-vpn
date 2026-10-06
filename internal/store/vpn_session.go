package store

import (
	"context"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// VPNSessionStore keeps the history of finished VPN connections.
type VPNSessionStore struct{ s *Store }

// Record stores a finished connection.
func (vs *VPNSessionStore) Record(ctx context.Context, session *model.VPNSession) error {
	session.ID = uuid.New()
	_, err := vs.s.db.ExecContext(ctx,
		`INSERT INTO "vpn_sessions" (id, device_id, user_id, started_at, ended_at,
			remote_ip, virtual_ip, bytes_received, bytes_sent)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.DeviceID, session.UserID, formatTime(session.StartedAt),
		formatTime(session.EndedAt), session.RemoteIP, session.VirtualIP,
		session.BytesReceived, session.BytesSent)
	return translate(err)
}

// ByUser returns a page of the user's finished connections, the latest
// first.
func (vs *VPNSessionStore) ByUser(ctx context.Context, userID uuid.UUID, q Query) (Page[*model.VPNSession], error) {
	var page Page[*model.VPNSession]
	if err := vs.s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM "vpn_sessions" WHERE user_id = ?`, userID).Scan(&page.Total); err != nil {
		return page, translate(err)
	}

	rows, err := vs.s.db.QueryContext(ctx,
		`SELECT s.id, s.device_id, s.user_id, s.started_at, s.ended_at, s.remote_ip,
			s.virtual_ip, s.bytes_received, s.bytes_sent, d.name
		 FROM "vpn_sessions" s JOIN "devices" d ON d.id = s.device_id
		 WHERE s.user_id = ? ORDER BY s.ended_at DESC`+q.limitOffset(), userID)
	if err != nil {
		return page, translate(err)
	}
	defer rows.Close()

	for rows.Next() {
		var session model.VPNSession
		var started, ended timestamp
		if err := rows.Scan(&session.ID, &session.DeviceID, &session.UserID, &started, &ended,
			&session.RemoteIP, &session.VirtualIP, &session.BytesReceived, &session.BytesSent,
			&session.DeviceName); err != nil {
			return page, translate(err)
		}
		session.StartedAt, session.EndedAt = started.t, ended.t
		page.Items = append(page.Items, &session)
	}
	return page, translate(rows.Err())
}

// TrafficByDevice returns the data each of a user's devices has moved over
// its recorded connections, in bytes, by device ID.
func (vs *VPNSessionStore) TrafficByDevice(ctx context.Context, userID uuid.UUID) (map[uuid.UUID]int64, error) {
	rows, err := vs.s.db.QueryContext(ctx,
		`SELECT device_id, SUM(bytes_received + bytes_sent) FROM "vpn_sessions"
		 WHERE user_id = ? GROUP BY device_id`, userID)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	traffic := map[uuid.UUID]int64{}
	for rows.Next() {
		var id uuid.UUID
		var total int64
		if err := rows.Scan(&id, &total); err != nil {
			return nil, translate(err)
		}
		traffic[id] = total
	}
	return traffic, translate(rows.Err())
}

// TrafficSince returns how many connections ended since cutoff and the data
// they moved, in bytes, both ways together.
func (vs *VPNSessionStore) TrafficSince(ctx context.Context, cutoff time.Time) (count int, bytes int64, err error) {
	err = vs.s.db.QueryRowContext(ctx,
		`SELECT COUNT(*), COALESCE(SUM(bytes_received + bytes_sent), 0) FROM "vpn_sessions" WHERE ended_at >= ?`,
		formatTime(cutoff.UTC())).Scan(&count, &bytes)
	return count, bytes, translate(err)
}

// DeleteOlderThan removes the connections that ended before cutoff,
// returning how many went.
func (vs *VPNSessionStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := vs.s.db.ExecContext(ctx,
		`DELETE FROM "vpn_sessions" WHERE ended_at < ?`, formatTime(cutoff.UTC()))
	if err != nil {
		return 0, translate(err)
	}
	removed, err := result.RowsAffected()
	return removed, translate(err)
}
