package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// migrationDir is the directory holding the migrations inside migrationFS.
const migrationDir = "migrations"

// migrate brings the database schema up to date.
//
// The first migration is written to be idempotent, because it may be
// applied to a database the Django release already created the tables in.
// Everything after it is an ordinary migration.
func migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrationFS)

	// goose writes progress to standard output by default, which would
	// mix into the output of whichever command happens to open the
	// database.
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("store: select the migration dialect: %w", err)
	}

	// Up reads the version table and returns without writing when there is
	// nothing to apply, which is every process start after the first.
	if err := goose.UpContext(ctx, db, migrationDir); err != nil {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}
