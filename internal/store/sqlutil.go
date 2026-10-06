package store

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// timeLayouts are the timestamp formats found in the database. Django wrote
// naive UTC text and omitted the fractional part when it was zero.
var timeLayouts = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02 15:04:05",
	time.RFC3339Nano,
}

// formatTime renders t the way the database stores timestamps.
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05.999999")
}

// nullTime renders an optional timestamp for storage.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatTime(*t)
}

// scanTime converts a stored timestamp back into a UTC time.
func scanTime(src any) (time.Time, error) {
	switch v := src.(type) {
	case nil:
		return time.Time{}, nil
	case time.Time:
		return v.UTC(), nil
	case []byte:
		return parseTime(string(v))
	case string:
		return parseTime(v)
	default:
		return time.Time{}, fmt.Errorf("store: cannot read timestamp of type %T", src)
	}
}

// parseTime reads a stored timestamp, trying each known layout in turn.
func parseTime(s string) (time.Time, error) {
	if s = strings.TrimSpace(s); s == "" {
		return time.Time{}, nil
	}
	for _, layout := range timeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("store: unrecognised timestamp %q", s)
}

// timestamp is a time.Time that can be scanned straight out of a row.
type timestamp struct{ t time.Time }

// Scan implements [sql.Scanner].
func (ts *timestamp) Scan(src any) error {
	t, err := scanTime(src)
	ts.t = t
	return err
}

// optionalTimestamp is a nullable time.Time that can be scanned from a row.
type optionalTimestamp struct{ t *time.Time }

// Scan implements [sql.Scanner].
func (ts *optionalTimestamp) Scan(src any) error {
	if src == nil {
		ts.t = nil
		return nil
	}
	t, err := scanTime(src)
	if err != nil || t.IsZero() {
		ts.t = nil
		return err
	}
	ts.t = &t
	return nil
}

// optionalBool is a nullable bool that can be scanned from a row.
type optionalBool struct{ b *bool }

// Scan implements [sql.Scanner].
func (ob *optionalBool) Scan(src any) error {
	if src == nil {
		ob.b = nil
		return nil
	}
	var n sql.NullBool
	if err := n.Scan(src); err != nil {
		return err
	}
	ob.b = &n.Bool
	return nil
}

// nullBool renders an optional bool for storage.
func nullBool(b *bool) any {
	if b == nil {
		return nil
	}
	return *b
}

// translate maps driver level errors onto the package's sentinel errors.
func translate(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	case strings.Contains(err.Error(), "UNIQUE constraint failed"):
		return fmt.Errorf("%w: %s", ErrConflict, err)
	default:
		return err
	}
}

// likeTerm wraps a search term for a case insensitive LIKE comparison.
func likeTerm(search string) string {
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(search)
	return "%" + escaped + "%"
}

// searchClause builds an OR'ed LIKE filter across the given columns along
// with the arguments it needs. Both are empty when search is empty.
func searchClause(search string, columns ...string) (string, []any) {
	if search == "" {
		return "", nil
	}

	term := likeTerm(search)
	parts := make([]string, len(columns))
	args := make([]any, len(columns))

	for i, column := range columns {
		parts[i] = column + ` LIKE ? ESCAPE '\'`
		args[i] = term
	}
	return "(" + strings.Join(parts, " OR ") + ")", args
}
