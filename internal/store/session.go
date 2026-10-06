package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// SessionStore keeps web sessions in the application database, so that they
// survive a restart and are shared by every worker.
//
// It satisfies the session manager's store interface, whose methods take no
// context; queries are therefore bounded by a timeout of their own.
type SessionStore struct {
	db      *sql.DB
	timeout time.Duration
}

// sessionQueryTimeout bounds a session query. A session read sits in front
// of every request, so it must never be the thing that hangs one.
const sessionQueryTimeout = 5 * time.Second

// Sessions returns the session store.
func (s *Store) Sessions() *SessionStore {
	return &SessionStore{db: s.db, timeout: sessionQueryTimeout}
}

// Find returns the data held against a session token, reporting whether a
// live session was found. An expired or unknown token is not an error.
func (ss *SessionStore) Find(token string) ([]byte, bool, error) {
	ctx, cancel := ss.context()
	defer cancel()

	var data []byte
	err := ss.db.QueryRowContext(ctx,
		`SELECT data FROM "sessions" WHERE token = ? AND expires > ?`,
		token, formatTime(time.Now().UTC())).Scan(&data)

	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, false, nil
	case err != nil:
		return nil, false, err
	default:
		return data, true, nil
	}
}

// Commit stores the data for a session token until the given expiry.
func (ss *SessionStore) Commit(token string, data []byte, expiry time.Time) error {
	ctx, cancel := ss.context()
	defer cancel()

	_, err := ss.db.ExecContext(ctx,
		`INSERT INTO "sessions" (token, data, expires) VALUES (?, ?, ?)
		 ON CONFLICT(token) DO UPDATE SET data = excluded.data, expires = excluded.expires`,
		token, data, formatTime(expiry.UTC()))
	return err
}

// Delete removes a session.
func (ss *SessionStore) Delete(token string) error {
	ctx, cancel := ss.context()
	defer cancel()

	_, err := ss.db.ExecContext(ctx, `DELETE FROM "sessions" WHERE token = ?`, token)
	return err
}

// DeleteExpired removes every session that has lapsed.
func (ss *SessionStore) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := ss.db.ExecContext(ctx,
		`DELETE FROM "sessions" WHERE expires <= ?`, formatTime(time.Now().UTC()))
	if err != nil {
		return 0, translate(err)
	}
	return result.RowsAffected()
}

// context returns a context bounded by the store's query timeout.
func (ss *SessionStore) context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), ss.timeout)
}
