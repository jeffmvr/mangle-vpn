package store

import (
	"context"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// PasswordLinkStore keeps the single-use links people choose a password
// through. It holds hashes of the links' codes, never the codes.
type PasswordLinkStore struct{ s *Store }

// Create records a link for the user, with the hash of its code, what it
// was issued for, and when it lapses.
func (ps *PasswordLinkStore) Create(ctx context.Context, userID uuid.UUID, hash, purpose string, expires time.Time) error {
	_, err := ps.s.db.ExecContext(ctx,
		`INSERT INTO "password_links" (id, created_at, user_id, code_hash, purpose, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New(), formatTime(time.Now().UTC()), userID, hash, purpose, formatTime(expires.UTC()))
	return translate(err)
}

// Find returns the user and purpose of the unused, unexpired link with the
// given hash, or ErrNotFound. It leaves the link usable.
func (ps *PasswordLinkStore) Find(ctx context.Context, hash string) (uuid.UUID, string, error) {
	var userID uuid.UUID
	var purpose string
	err := ps.s.db.QueryRowContext(ctx,
		`SELECT user_id, purpose FROM "password_links"
		 WHERE code_hash = ? AND used_at IS NULL AND expires_at > ?`,
		hash, formatTime(time.Now().UTC())).Scan(&userID, &purpose)
	return userID, purpose, translate(err)
}

// Take spends the unused, unexpired link with the given hash, returning its
// user and purpose, or ErrNotFound. Finding and spending it are one
// statement, so a link cannot be used twice even by requests racing.
func (ps *PasswordLinkStore) Take(ctx context.Context, hash string) (uuid.UUID, string, error) {
	now := formatTime(time.Now().UTC())

	var userID uuid.UUID
	var purpose string
	err := ps.s.db.QueryRowContext(ctx,
		`UPDATE "password_links" SET used_at = ?
		 WHERE code_hash = ? AND used_at IS NULL AND expires_at > ?
		 RETURNING user_id, purpose`,
		now, hash, now).Scan(&userID, &purpose)
	return userID, purpose, translate(err)
}

// DeleteForUser removes every link issued to the user, so that issuing a
// new one, or choosing a password, retires the rest.
func (ps *PasswordLinkStore) DeleteForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := ps.s.db.ExecContext(ctx, `DELETE FROM "password_links" WHERE user_id = ?`, userID)
	return translate(err)
}

// DeleteExpired removes links that have lapsed or been used, returning how
// many went.
func (ps *PasswordLinkStore) DeleteExpired(ctx context.Context) (int64, error) {
	result, err := ps.s.db.ExecContext(ctx,
		`DELETE FROM "password_links" WHERE used_at IS NOT NULL OR expires_at <= ?`,
		formatTime(time.Now().UTC()))
	if err != nil {
		return 0, translate(err)
	}
	removed, err := result.RowsAffected()
	return removed, translate(err)
}
