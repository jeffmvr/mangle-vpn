package store

import (
	"context"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// eventColumns lists the event columns followed by those of the user the
// event concerns, in the order eventScan expects.
const eventColumns = `e.id, e.created_at, e.updated_at, e.detail, e.name, e.user_id, ` + userColumns

// eventJoin attaches the user and group behind every event.
const eventJoin = ` FROM "events" e
	JOIN "users" u ON u.id = e.user_id
	JOIN "groups" g ON g.id = u.group_id`

// eventSearchColumns are the columns an administrator's search looks in. The
// detail is among them because an administrator's change names the account
// or group it was made to, which is what someone searching for it types.
var eventSearchColumns = []string{"e.name", "e.detail", "u.email", "u.name", "g.name"}

// newEventScan allocates a scanner for eventColumns.
func newEventScan() entityScan[*model.Event] { return &eventScan{} }

// EventStore reads and writes the audit log.
type EventStore struct{ s *Store }

// Record appends an entry to the audit log.
func (es *EventStore) Record(ctx context.Context, userID uuid.UUID, name, detail string) error {
	now := time.Now().UTC()

	_, err := es.s.db.ExecContext(ctx,
		`INSERT INTO "events" (id, created_at, updated_at, detail, name, user_id)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New(), formatTime(now), formatTime(now), detail, name, userID)
	return translate(err)
}

// SignInAddresses reports whether the user has signed in to the web
// application before, and whether from the given address. Sign ins are
// found by their recorded detail, which names the address.
func (es *EventStore) SignInAddresses(ctx context.Context, userID uuid.UUID, address string) (before, fromHere bool, err error) {
	err = es.s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) > 0, COALESCE(SUM(detail LIKE ? ESCAPE '\'), 0) > 0
		 FROM "events" WHERE user_id = ? AND name = ?`,
		"% from "+strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(address)+".", userID, model.EventWebLogin).Scan(&before, &fromHere)
	return before, fromHere, translate(err)
}

// CountSince returns how many events of each name were recorded since
// cutoff.
func (es *EventStore) CountSince(ctx context.Context, cutoff time.Time) (map[string]int, error) {
	rows, err := es.s.db.QueryContext(ctx,
		`SELECT name, COUNT(*) FROM "events" WHERE created_at >= ? GROUP BY name`, formatTime(cutoff.UTC()))
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return nil, translate(err)
		}
		counts[name] = count
	}
	return counts, translate(rows.Err())
}

// DeleteOlderThan removes the events recorded before cutoff, returning how
// many went.
func (es *EventStore) DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	result, err := es.s.db.ExecContext(ctx,
		`DELETE FROM "events" WHERE created_at < ?`, formatTime(cutoff.UTC()))
	if err != nil {
		return 0, translate(err)
	}
	removed, err := result.RowsAffected()
	return removed, translate(err)
}

// EventFilter narrows an audit log listing beyond its search term.
type EventFilter struct {
	// Kind keeps the events whose name starts with this kind, one of
	// model.EventKinds; empty keeps every event.
	Kind string

	// UserID keeps the events recorded against one user.
	UserID uuid.UUID
}

// List returns the events matching the query and filter, newest first.
func (es *EventStore) List(ctx context.Context, q Query, f EventFilter) (Page[*model.Event], error) {
	var clauses []string

	search, args := searchClause(q.Search, eventSearchColumns...)
	if search != "" {
		clauses = append(clauses, search)
	}
	if f.Kind != "" {
		clauses = append(clauses, `e.name LIKE ? ESCAPE '\'`)
		args = append(args, strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(f.Kind)+".%")
	}
	if !f.UserID.IsZero() {
		clauses = append(clauses, "e.user_id = ?")
		args = append(args, f.UserID)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}

	var page Page[*model.Event]
	if err := es.s.db.QueryRowContext(ctx,
		`SELECT COUNT(*)`+eventJoin+where, args...).Scan(&page.Total); err != nil {
		return page, translate(err)
	}

	rows, err := es.s.db.QueryContext(ctx,
		`SELECT `+eventColumns+eventJoin+where+
			` ORDER BY e.created_at DESC, e.name`+q.limitOffset(), args...)

	page.Items, err = queryAll(rows, err, newEventScan)
	return page, err
}
