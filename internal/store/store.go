// Package store is the application's persistence layer. It owns the SQLite
// database and exposes one repository per entity.
//
// The schema matches the one Django created, so an existing mangle.db is
// adopted in place. Changes to it are versioned migrations under
// migrations/, applied by goose when the database is opened.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("store: not found")

// ErrConflict is returned when a write violates a uniqueness constraint.
var ErrConflict = errors.New("store: conflict")

// Store owns the database handle and the repositories built on top of it.
type Store struct {
	db *sql.DB

	// sealer encrypts stored secrets once UseSecretKey has been called.
	sealer *sealer

	Clients        *ClientStore
	Devices        *DeviceStore
	Events         *EventStore
	FirewallRules  *FirewallRuleStore
	Groups         *GroupStore
	Jobs           *JobStore
	PasswordLinks  *PasswordLinkStore
	RevokedDevices *RevokedDeviceStore
	Settings       *SettingStore
	Users          *UserStore
	VPNSessions    *VPNSessionStore
}

// Open opens the database at path, creating it when it does not yet exist,
// and brings the schema up to date.
func Open(path string) (*Store, error) {
	return OpenContext(context.Background(), path)
}

// OpenContext behaves like [Open] with a caller supplied context, which
// bounds the schema migration.
func OpenContext(ctx context.Context, path string) (*Store, error) {
	// A single writer with the write-ahead log and a generous busy timeout
	// is what keeps the web, task, and VPN hook processes from tripping over
	// each other on the shared file.
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)", path)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}

	// SQLite serialises writes anyway, and holding one connection avoids
	// spurious "database is locked" errors under concurrent requests.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}

	s := &Store{db: db}
	s.Clients = &ClientStore{s}
	s.Devices = &DeviceStore{s}
	s.Events = &EventStore{s}
	s.FirewallRules = &FirewallRuleStore{s}
	s.Groups = &GroupStore{s}
	s.Jobs = &JobStore{s}
	s.PasswordLinks = &PasswordLinkStore{s}
	s.RevokedDevices = &RevokedDeviceStore{s}
	s.Settings = &SettingStore{s}
	s.Users = &UserStore{s}
	s.VPNSessions = &VPNSessionStore{s}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// inTx runs fn inside a transaction, committing if it returns nil and
// rolling back otherwise.
func (s *Store) inTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return translate(err)
	}
	if err := fn(tx); err != nil {
		tx.Rollback()
		return translate(err)
	}
	return translate(tx.Commit())
}

// SnapshotTo writes a consistent copy of the database to path, which must
// not exist yet. It is safe while other processes are using the database.
func (s *Store) SnapshotTo(ctx context.Context, path string) error {
	_, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path)
	if err != nil {
		return fmt.Errorf("store: snapshot the database: %w", err)
	}
	return nil
}

// DB exposes the underlying handle for the rare caller that needs it.
func (s *Store) DB() *sql.DB { return s.db }
