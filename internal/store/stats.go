package store

import (
	"context"
	"time"
)

// Stats are the counts monitoring reports.
type Stats struct {
	Users       int
	LockedUsers int
	Devices     int
	Clients     int
}

// Stats counts what monitoring reports, in one query.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats
	err := s.db.QueryRowContext(ctx, `SELECT
		(SELECT COUNT(*) FROM "users"),
		(SELECT COUNT(*) FROM "users" WHERE locked_until > ?),
		(SELECT COUNT(*) FROM "devices"),
		(SELECT COUNT(*) FROM "clients")`,
		formatTime(time.Now().UTC())).Scan(&st.Users, &st.LockedUsers, &st.Devices, &st.Clients)
	return st, translate(err)
}

// Ping reports whether the database answers.
func (s *Store) Ping(ctx context.Context) error {
	return translate(s.db.PingContext(ctx))
}
