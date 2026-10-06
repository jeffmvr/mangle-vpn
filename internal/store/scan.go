package store

import (
	"database/sql"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// The scanners below let a query compose its SELECT list and its scan
// destinations from the same building blocks. Each one exposes the columns it
// reads, the destinations to scan them into, and the entity that results.
//
// Joined queries nest them, so a client reads
// clientColumns + deviceColumns + userColumns + groupColumns and scans into
// the matching chain of destinations.

// groupScan reads the columns listed in groupColumns.
type groupScan struct {
	g       model.Group
	created timestamp
	updated timestamp
}

func (s *groupScan) dest() []any {
	return []any{&s.g.ID, &s.created, &s.updated, &s.g.Description, &s.g.IsEnabled,
		&s.g.MaxDevices, &s.g.MFAEnforced, &s.g.Name, &s.g.Routes, &s.g.Nameservers,
		&s.g.DeviceIdleDays}
}

func (s *groupScan) value() *model.Group {
	s.g.CreatedAt, s.g.UpdatedAt = s.created.t, s.updated.t
	return &s.g
}

// userScan reads the columns listed in userColumns, including the nested
// group every user lookup joins in.
type userScan struct {
	u              model.User
	created        timestamp
	updated        timestamp
	lastLogin      optionalTimestamp
	passwordExpire optionalTimestamp
	lockedUntil    optionalTimestamp
	mfaEnforced    optionalBool
	group          groupScan
}

func (s *userScan) dest() []any {
	own := []any{&s.u.ID, &s.created, &s.updated, &s.u.Email, &s.u.GroupID,
		&s.u.IsAdmin, &s.u.IsEnabled, &s.lastLogin, &s.u.MFAEnabled, &s.mfaEnforced,
		&s.u.MFASecret, &s.u.Name, &s.u.Password, &s.u.PasswordChange,
		&s.passwordExpire, &s.u.MFALastStep, &s.u.FailedLogins, &s.lockedUntil,
		&s.u.SessionEpoch, &s.u.Role}
	return append(own, s.group.dest()...)
}

func (s *userScan) value() *model.User {
	s.u.CreatedAt, s.u.UpdatedAt = s.created.t, s.updated.t
	s.u.LastLogin, s.u.PasswordExpire = s.lastLogin.t, s.passwordExpire.t
	s.u.MFAEnforced = s.mfaEnforced.b
	s.u.LockedUntil = s.lockedUntil.t
	s.u.Group = s.group.value()
	return &s.u
}

// deviceScan reads the columns listed in deviceColumns, including the nested
// owner and their group.
type deviceScan struct {
	d         model.Device
	created   timestamp
	updated   timestamp
	lastLogin optionalTimestamp
	user      userScan
}

func (s *deviceScan) dest() []any {
	own := []any{&s.d.ID, &s.created, &s.updated, &s.d.Fingerprint, &s.lastLogin,
		&s.d.Name, &s.d.OS, &s.d.Serial, &s.d.UserID, &s.d.StaticIP}
	return append(own, s.user.dest()...)
}

func (s *deviceScan) value() *model.Device {
	s.d.CreatedAt, s.d.UpdatedAt, s.d.LastLogin = s.created.t, s.updated.t, s.lastLogin.t
	s.d.User = s.user.value()
	return &s.d
}

// clientScan reads the columns listed in clientColumns, including the nested
// device, its owner, and their group.
type clientScan struct {
	c       model.Client
	created timestamp
	updated timestamp
	device  deviceScan
}

func (s *clientScan) dest() []any {
	own := []any{&s.c.ID, &s.created, &s.updated, &s.c.CommonName, &s.c.DeviceID,
		&s.c.Platform, &s.c.RemoteIP, &s.c.VirtualIP}
	return append(own, s.device.dest()...)
}

func (s *clientScan) value() *model.Client {
	s.c.CreatedAt, s.c.UpdatedAt = s.created.t, s.updated.t
	s.c.Device = s.device.value()
	return &s.c
}

// eventScan reads the columns listed in eventColumns, including the nested
// user and their group.
type eventScan struct {
	e       model.Event
	created timestamp
	updated timestamp
	user    userScan
}

func (s *eventScan) dest() []any {
	own := []any{&s.e.ID, &s.created, &s.updated, &s.e.Detail, &s.e.Name, &s.e.UserID}
	return append(own, s.user.dest()...)
}

func (s *eventScan) value() *model.Event {
	s.e.CreatedAt, s.e.UpdatedAt = s.created.t, s.updated.t
	s.e.User = s.user.value()
	return &s.e
}

// firewallRuleScan reads the columns listed in firewallRuleColumns.
type firewallRuleScan struct {
	r       model.FirewallRule
	created timestamp
	updated timestamp
}

func (s *firewallRuleScan) dest() []any {
	return []any{&s.r.ID, &s.created, &s.updated, &s.r.Action, &s.r.Destination,
		&s.r.GroupID, &s.r.IsEnabled, &s.r.Port, &s.r.Protocol}
}

func (s *firewallRuleScan) value() *model.FirewallRule {
	s.r.CreatedAt, s.r.UpdatedAt = s.created.t, s.updated.t
	return &s.r
}

// entityScan is a scanner that reads one entity of type T out of a row.
type entityScan[T any] interface {
	dest() []any
	value() T
}

// queryOne reads a single row through the given scanner.
func queryOne[T any](row scanner, s entityScan[T]) (T, error) {
	if err := row.Scan(s.dest()...); err != nil {
		var zero T
		return zero, translate(err)
	}
	return s.value(), nil
}

// queryAll reads every row of a result set through a fresh scanner each time,
// closing the set when it is finished. It takes the error from the query that
// produced rows so that call sites read as a single expression.
func queryAll[T any](rows *sql.Rows, err error, newScan func() entityScan[T]) ([]T, error) {
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()

	var items []T
	for rows.Next() {
		s := newScan()
		if err := rows.Scan(s.dest()...); err != nil {
			return nil, translate(err)
		}
		items = append(items, s.value())
	}
	return items, translate(rows.Err())
}
