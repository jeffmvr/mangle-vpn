package model

import (
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/auth"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// unusablePasswordLength is the length of the random tail Django puts after
// the "!" of an unusable password, which keeps each one distinct.
const unusablePasswordLength = 40

// User is a person who may sign in to the web application and connect
// devices to the VPN.
type User struct {
	Base

	Email          string
	GroupID        uuid.UUID
	IsAdmin        bool
	Role           string
	IsEnabled      bool
	LastLogin      *time.Time
	MFAEnabled     bool
	MFAEnforced    *bool
	MFASecret      string
	Name           string
	Password       string
	PasswordChange bool
	PasswordExpire *time.Time

	// Sign-in protection. These are written only through the store's
	// dedicated methods, never by Save, so that concurrent sign ins on the
	// web and the VPN cannot overwrite each other's counts.
	MFALastStep  int64
	FailedLogins int
	LockedUntil  *time.Time

	// SessionEpoch is recorded by each session at sign in; raising it ends
	// every session that recorded an earlier value.
	SessionEpoch int

	// Group is always populated by the store alongside the user, because a
	// user's effective permissions depend on it.
	Group *Group
}

// RoleHelpDesk is the role of someone who helps people back in without
// administering the server. See [User.IsStaff].
const RoleHelpDesk = "helpdesk"

// IsStaff reports whether the user may use the administration pages:
// administrators fully, and the help desk to look and to help people back
// in.
func (u *User) IsStaff() bool {
	return u.IsAdmin || u.Role == RoleHelpDesk
}

// IsActive reports whether the user may sign in, taking both the user's and
// their group's enabled flag into account.
func (u *User) IsActive() bool {
	return u.IsEnabled && u.Group != nil && u.Group.IsEnabled
}

// IsLocked reports whether the account is locked out after too many failed
// sign in attempts.
func (u *User) IsLocked(now time.Time) bool {
	return u.LockedUntil != nil && now.Before(*u.LockedUntil)
}

// PasswordExpired reports whether the user holds a temporary password whose
// window has passed.
func (u *User) PasswordExpired(now time.Time) bool {
	return u.PasswordChange && u.PasswordExpire != nil && now.After(*u.PasswordExpire)
}

// MFARequired reports whether the user must use two-factor authentication.
// A group may enforce it for all of its members, and an individual user may
// opt out.
func (u *User) MFARequired() bool {
	enforcedForUser := u.MFAEnforced == nil || *u.MFAEnforced
	return enforcedForUser && u.Group != nil && u.Group.MFAEnforced
}

// ProvisioningURI returns the otpauth URI the user scans to enrol their
// authenticator app.
func (u *User) ProvisioningURI(organization string) string {
	return auth.TOTPProvisioningURI(u.MFASecret, u.Email, organization+" VPN")
}

// VerifyMFACode reports whether code is the user's current two-factor
// authentication code at the given time, returning the time step it
// belongs to. The caller must still claim the step through the store, which
// is what stops a code from being used twice.
func (u *User) VerifyMFACode(code string, at time.Time) (step int64, ok bool) {
	if !auth.VerifyTOTPAt(u.MFASecret, code, at) {
		return 0, false
	}
	return auth.TOTPStep(at), true
}

// ResetMFA issues a new two-factor secret and marks the user as needing to
// enrol again.
func (u *User) ResetMFA() {
	u.MFASecret = auth.NewTOTPSecret()
	u.MFAEnabled = false
}

// SetPassword replaces the user's password with a hash of the plain text.
func (u *User) SetPassword(password string) {
	u.Password = auth.HashPassword(password)
}

// CheckPassword reports whether password is the user's password. A user
// with no usable password still costs a full check, so that the time taken
// does not tell an attacker which accounts exist.
func (u *User) CheckPassword(password string) bool {
	if !auth.IsPasswordUsable(u.Password) {
		return auth.CheckDecoyPassword(password)
	}
	return auth.CheckPassword(password, u.Password)
}

// PasswordNeedsRehash reports whether the user's password hash, having just
// verified, should be replaced with one made at the current strength.
func (u *User) PasswordNeedsRehash() bool {
	return auth.NeedsRehash(u.Password)
}

// ClearPassword leaves the user with no usable password, as Django marks
// one: they sign in again only after choosing a new one through a link.
func (u *User) ClearPassword() {
	u.Password = "!" + auth.GeneratePassword(unusablePasswordLength)
	u.PasswordChange = false
	u.PasswordExpire = nil
}
