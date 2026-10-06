// Package uuid adapts [github.com/google/uuid] to the storage format this
// application inherited from its Django release.
//
// UUIDs live in the database as 32 unseparated hex characters, which is what
// Django's UUIDField writes on SQLite, while JSON and logs use the canonical
// hyphenated form. Only the database encoding differs from the upstream
// package, so that is all this type overrides.
package uuid

import (
	"database/sql/driver"
	"encoding/hex"
	"fmt"

	"github.com/google/uuid"
)

// UUID is a 128 bit universally unique identifier.
type UUID struct{ uuid.UUID }

// Nil is the zero UUID.
var Nil UUID

// New returns a randomly generated version 4 UUID.
func New() UUID { return UUID{uuid.New()} }

// Parse reads a UUID in either the hyphenated or the bare hex form.
func Parse(s string) (UUID, error) {
	parsed, err := uuid.Parse(s)
	if err != nil {
		return Nil, fmt.Errorf("uuid: %w", err)
	}
	return UUID{parsed}, nil
}

// IsZero reports whether the UUID is the zero value.
func (u UUID) IsZero() bool { return u == Nil }

// Hex returns the 32 character unseparated form used for storage.
func (u UUID) Hex() string { return hex.EncodeToString(u.UUID[:]) }

// Value implements [driver.Valuer], storing the bare hex form.
func (u UUID) Value() (driver.Value, error) { return u.Hex(), nil }

// Scan implements [sql.Scanner], accepting either stored form.
func (u *UUID) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*u = Nil
		return nil
	case string:
		parsed, err := Parse(v)
		*u = parsed
		return err
	case []byte:
		parsed, err := Parse(string(v))
		*u = parsed
		return err
	default:
		return fmt.Errorf("uuid: cannot scan %T", src)
	}
}
