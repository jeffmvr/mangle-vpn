package store

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// userColumns lists the user columns, followed by the columns of the group
// the user belongs to, in the order scanUser expects.
const userColumns = `u.id, u.created_at, u.updated_at, u.email, u.group_id, u.is_admin,
	u.is_enabled, u.last_login, u.mfa_enabled, u.mfa_enforced, u.mfa_secret, u.name,
	u.password, u.password_change, u.password_expire, u.mfa_last_step, u.failed_logins,
	u.locked_until, u.session_epoch, u.role, ` + groupColumns

// userJoin attaches the group every user lookup needs to resolve the user's
// effective permissions.
const userJoin = ` FROM "users" u JOIN "groups" g ON g.id = u.group_id`

// newUserScan allocates a scanner for userColumns.
func newUserScan() entityScan[*model.User] { return &userScan{} }

// UserStore reads and writes users. Every user it returns carries its group.
type UserStore struct{ s *Store }

// Get returns the user with the given ID.
func (us *UserStore) Get(ctx context.Context, id uuid.UUID) (*model.User, error) {
	return us.unsealed(queryOne(us.s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+userJoin+` WHERE u.id = ?`, id), &userScan{}))
}

// ByEmail returns the user with the given e-mail address, matched case
// insensitively.
func (us *UserStore) ByEmail(ctx context.Context, email string) (*model.User, error) {
	return us.unsealed(queryOne(us.s.db.QueryRowContext(ctx,
		`SELECT `+userColumns+userJoin+` WHERE u.email = ? COLLATE NOCASE`, email), &userScan{}))
}

// UserFilter narrows a user listing beyond its search term. A nil field
// leaves that property unfiltered.
type UserFilter struct {
	Admin   *bool
	Enabled *bool
}

// List returns the users matching the query and filter, ordered by e-mail
// address.
func (us *UserStore) List(ctx context.Context, q Query, f UserFilter) (Page[*model.User], error) {
	var clauses []string

	search, args := searchClause(q.Search, "u.email", "u.name", "g.name")
	if search != "" {
		clauses = append(clauses, search)
	}
	if f.Admin != nil {
		clauses = append(clauses, "u.is_admin = ?")
		args = append(args, *f.Admin)
	}
	if f.Enabled != nil {
		clauses = append(clauses, "u.is_enabled = ?")
		args = append(args, *f.Enabled)
	}

	where := ""
	if len(clauses) > 0 {
		where = " WHERE " + strings.Join(clauses, " AND ")
	}
	return us.list(ctx, where, " ORDER BY u.email"+q.limitOffset(), args, q)
}

// unsealed decrypts a loaded user's two-factor secret in place. Users loaded
// through joins (a device's owner, an event's user) keep the stored form;
// they are never used to check a code, and saving one writes the stored
// form back unchanged.
func (us *UserStore) unsealed(u *model.User, err error) (*model.User, error) {
	if err != nil {
		return nil, err
	}
	secret, err := us.s.unseal(u.MFASecret)
	if err != nil {
		return nil, err
	}
	u.MFASecret = secret
	return u, nil
}

// ByGroup returns every user in the group, ordered by e-mail address.
func (us *UserStore) ByGroup(ctx context.Context, groupID uuid.UUID) ([]*model.User, error) {
	page, err := us.list(ctx, ` WHERE u.group_id = ?`, ` ORDER BY u.email`,
		[]any{groupID}, Query{})
	return page.Items, err
}

// list runs a user listing with the given WHERE and trailing clauses.
func (us *UserStore) list(ctx context.Context, where, tail string, args []any, q Query) (Page[*model.User], error) {
	var page Page[*model.User]

	if q.Paginated() {
		if err := us.s.db.QueryRowContext(ctx,
			`SELECT COUNT(*)`+userJoin+where, args...).Scan(&page.Total); err != nil {
			return page, translate(err)
		}
	}

	rows, err := us.s.db.QueryContext(ctx, `SELECT `+userColumns+userJoin+where+tail, args...)

	page.Items, err = queryAll(rows, err, newUserScan)
	if err != nil {
		return page, err
	}
	for _, u := range page.Items {
		if _, err := us.unsealed(u, nil); err != nil {
			return page, err
		}
	}
	if !q.Paginated() {
		page.Total = len(page.Items)
	}
	return page, nil
}

// Save inserts or updates the user, assigning its identity, timestamps, and
// an initial two-factor secret on first write.
func (us *UserStore) Save(ctx context.Context, u *model.User) error {
	now := time.Now().UTC()
	u.UpdatedAt = now

	// A user is never stored without a two-factor secret, so that enrolment
	// is available the moment their group starts enforcing it.
	if u.MFASecret == "" {
		u.ResetMFA()
	}

	if u.ID.IsZero() {
		u.ID, u.CreatedAt = uuid.New(), now

		_, err := us.s.db.ExecContext(ctx,
			`INSERT INTO "users" (id, created_at, updated_at, email, group_id, is_admin,
				is_enabled, last_login, mfa_enabled, mfa_enforced, mfa_secret, name,
				password, password_change, password_expire, role)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			u.ID, formatTime(u.CreatedAt), formatTime(u.UpdatedAt), u.Email, u.GroupID,
			u.IsAdmin, u.IsEnabled, nullTime(u.LastLogin), u.MFAEnabled,
			nullBool(u.MFAEnforced), us.s.seal(u.MFASecret), u.Name, u.Password, u.PasswordChange,
			nullTime(u.PasswordExpire), u.Role)
		if err != nil {
			return translate(err)
		}
		return us.reloadGroup(ctx, u)
	}

	// The last sign in is written only by TouchLastLogin, so that a copy of
	// the user loaded before a sign in cannot put back an older time.
	_, err := us.s.db.ExecContext(ctx,
		`UPDATE "users" SET updated_at = ?, email = ?, group_id = ?, is_admin = ?,
			is_enabled = ?, mfa_enabled = ?, mfa_enforced = ?,
			mfa_secret = ?, name = ?, password = ?, password_change = ?,
			password_expire = ?, role = ? WHERE id = ?`,
		formatTime(u.UpdatedAt), u.Email, u.GroupID, u.IsAdmin, u.IsEnabled,
		u.MFAEnabled, nullBool(u.MFAEnforced), us.s.seal(u.MFASecret),
		u.Name, u.Password, u.PasswordChange, nullTime(u.PasswordExpire), u.Role, u.ID)
	if err != nil {
		return translate(err)
	}
	return us.reloadGroup(ctx, u)
}

// The methods below each write a few columns rather than the whole row.
// They serve changes a user makes to their own account mid request, which
// must not undo whatever an administrator changed in the meantime, such as
// disabling them.

// SetPassword stores the user's password hash and whether, and until when,
// it has to be changed.
func (us *UserStore) SetPassword(ctx context.Context, u *model.User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := us.s.db.ExecContext(ctx,
		`UPDATE "users" SET updated_at = ?, password = ?, password_change = ?,
			password_expire = ? WHERE id = ?`,
		formatTime(u.UpdatedAt), u.Password, u.PasswordChange, nullTime(u.PasswordExpire), u.ID)
	return translate(err)
}

// SetMFAEnabled stores whether the user has completed two-factor enrolment.
func (us *UserStore) SetMFAEnabled(ctx context.Context, u *model.User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := us.s.db.ExecContext(ctx,
		`UPDATE "users" SET updated_at = ?, mfa_enabled = ? WHERE id = ?`,
		formatTime(u.UpdatedAt), u.MFAEnabled, u.ID)
	return translate(err)
}

// SetName stores the user's display name.
func (us *UserStore) SetName(ctx context.Context, u *model.User) error {
	u.UpdatedAt = time.Now().UTC()
	_, err := us.s.db.ExecContext(ctx,
		`UPDATE "users" SET updated_at = ?, name = ? WHERE id = ?`,
		formatTime(u.UpdatedAt), u.Name, u.ID)
	return translate(err)
}

// EndSessions raises the user's session epoch, which ends every session
// they have open.
func (us *UserStore) EndSessions(ctx context.Context, u *model.User) error {
	err := us.s.db.QueryRowContext(ctx,
		`UPDATE "users" SET session_epoch = session_epoch + 1 WHERE id = ? RETURNING session_epoch`,
		u.ID).Scan(&u.SessionEpoch)
	return translate(err)
}

// TouchLastLogin records that the user has just signed in.
func (us *UserStore) TouchLastLogin(ctx context.Context, u *model.User) error {
	now := time.Now().UTC()
	u.LastLogin = &now

	_, err := us.s.db.ExecContext(ctx, `UPDATE "users" SET last_login = ? WHERE id = ?`,
		formatTime(now), u.ID)
	return translate(err)
}

// ClaimMFAStep records that a two-factor code from the given time step has
// been used, reporting false when that step or a later one already was. The
// check and the write are one statement, so two sign ins racing with the
// same code cannot both succeed.
func (us *UserStore) ClaimMFAStep(ctx context.Context, u *model.User, step int64) (bool, error) {
	result, err := us.s.db.ExecContext(ctx,
		`UPDATE "users" SET mfa_last_step = ? WHERE id = ? AND mfa_last_step < ?`,
		step, u.ID, step)
	if err != nil {
		return false, translate(err)
	}
	claimed, err := result.RowsAffected()
	if err != nil {
		return false, translate(err)
	}
	if claimed == 1 {
		u.MFALastStep = step
	}
	return claimed == 1, nil
}

// RecordFailedLogin counts a failed password or two-factor code against the
// user. The failure that reaches limit locks the account until lockout has
// passed and starts the count again; locked reports whether this one did.
func (us *UserStore) RecordFailedLogin(ctx context.Context, u *model.User, limit int, lockout time.Duration) (locked bool, err error) {
	until := formatTime(time.Now().UTC().Add(lockout))

	// Every expression on the right reads the row as it was before the
	// update, so both CASEs see the same count. RETURNING sees the row
	// after it.
	err = us.s.db.QueryRowContext(ctx,
		`UPDATE "users" SET
			failed_logins = CASE WHEN failed_logins + 1 >= ? THEN 0 ELSE failed_logins + 1 END,
			locked_until  = CASE WHEN failed_logins + 1 >= ? THEN ? ELSE locked_until END
		 WHERE id = ?
		 RETURNING failed_logins = 0 AND locked_until IS ?`,
		limit, limit, until, u.ID, until).Scan(&locked)
	return locked, translate(err)
}

// ClearFailedLogins resets the failure count after a successful sign in.
func (us *UserStore) ClearFailedLogins(ctx context.Context, u *model.User) error {
	u.FailedLogins, u.LockedUntil = 0, nil
	_, err := us.s.db.ExecContext(ctx,
		`UPDATE "users" SET failed_logins = 0, locked_until = NULL WHERE id = ?`, u.ID)
	return translate(err)
}

// Delete removes the user along with everything that belongs to them: their
// audit events, password links, connections, connection history and
// devices, revoking every device certificate
// that was issued. It happens in one transaction, so a failure leaves the
// user exactly as they were.
//
// The schema, shared with the Django release, has no ON DELETE CASCADE;
// Django cascaded in the ORM, so the rows are removed here explicitly.
func (us *UserStore) Delete(ctx context.Context, id uuid.UUID) error {
	now := formatTime(time.Now().UTC())

	return us.s.inTx(ctx, func(tx *sql.Tx) error {
		statements := []struct {
			query string
			args  []any
		}{
			// Revoked serials get Django-style identifiers: 32 hex digits.
			{`INSERT INTO "revoked_devices" (id, created_at, updated_at, serial)
			  SELECT lower(hex(randomblob(16))), ?, ?, serial FROM "devices"
			  WHERE user_id = ? AND serial != ''`, []any{now, now, id}},
			{`DELETE FROM "clients" WHERE device_id IN (SELECT id FROM "devices" WHERE user_id = ?)`, []any{id}},
			{`DELETE FROM "vpn_sessions" WHERE user_id = ?`, []any{id}},
			{`DELETE FROM "devices" WHERE user_id = ?`, []any{id}},
			{`DELETE FROM "events" WHERE user_id = ?`, []any{id}},
			{`DELETE FROM "password_links" WHERE user_id = ?`, []any{id}},
			{`DELETE FROM "users" WHERE id = ?`, []any{id}},
		}
		for _, st := range statements {
			if _, err := tx.ExecContext(ctx, st.query, st.args...); err != nil {
				return err
			}
		}
		return nil
	})
}

// reloadGroup refreshes the group attached to a saved user so that callers
// always see the group matching the stored group_id.
func (us *UserStore) reloadGroup(ctx context.Context, u *model.User) error {
	if u.Group != nil && u.Group.ID == u.GroupID {
		return nil
	}
	group, err := us.s.Groups.Get(ctx, u.GroupID)
	if err != nil {
		return err
	}
	u.Group = group
	return nil
}
