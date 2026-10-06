package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/auth"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// This file is the one place sign in is decided. The web pages, OpenVPN
// Connect's profile import and the OpenVPN authentication hook all come
// through it, so a rule changed here changes for all three.

// Account lockout: after so many wrong passwords or codes in a row, an
// account refuses every attempt for a while. Both are settings, with these
// defaults. The count is kept in the database, so the web application and
// the OpenVPN hooks, which run as separate processes, share it.
const (
	LoginFailureLimit = 10
	LoginLockout      = 15 * time.Minute
)

// lockoutPolicy returns how many failures lock an account and for how long.
func (a *App) lockoutPolicy() (int, time.Duration) {
	return a.Config.Int(config.AuthLockoutAttempts, LoginFailureLimit),
		time.Duration(a.Config.Int(config.AuthLockoutMinutes, int(LoginLockout.Minutes()))) * time.Minute
}

// ErrBadCredentials is returned for every refused password, whatever the
// reason: an unknown address, a wrong password, a locked or a disabled
// account. Callers show one message for all of them, so that the answer
// does not tell an attacker which addresses have accounts.
var ErrBadCredentials = errors.New("app: invalid email address or password")

// Authenticate checks an email address and password and returns the user
// they belong to.
//
// A full password check runs whether or not the account exists or is
// locked, so the time taken gives nothing away. A wrong password counts
// towards the account's lockout. A right one stored at an older strength is
// rehashed at the current one. Two-factor authentication is not part of
// this: see VerifyMFA.
func (a *App) Authenticate(ctx context.Context, email, password string) (*model.User, error) {
	user, err := a.Store.Users.ByEmail(ctx, strings.TrimSpace(email))
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.Log.Error("failed to look up a user signing in", "err", err)
		}
		auth.CheckDecoyPassword(password)
		return nil, ErrBadCredentials
	}

	if user.IsLocked(time.Now()) {
		auth.CheckDecoyPassword(password)
		return nil, ErrBadCredentials
	}

	if !user.CheckPassword(password) {
		a.RecordFailedSignIn(ctx, user)
		return nil, ErrBadCredentials
	}
	if !user.IsActive() {
		return nil, ErrBadCredentials
	}

	if user.PasswordNeedsRehash() {
		user.SetPassword(password)
		if err := a.Store.Users.SetPassword(ctx, user); err != nil {
			a.Log.Error("failed to upgrade a password hash", "user", user.Email, "err", err)
		}
	}
	return user, nil
}

// VerifyMFA checks a two-factor code for the user and claims its time step,
// so that the same code cannot be used again, on the web or on the VPN. A
// locked or inactive account is refused whatever the code. A wrong or
// reused code counts towards the lockout.
func (a *App) VerifyMFA(ctx context.Context, user *model.User, code string) (bool, error) {
	if user.IsLocked(time.Now()) || !user.IsActive() {
		return false, nil
	}

	step, ok := user.VerifyMFACode(code, time.Now())
	if ok {
		claimed, err := a.Store.Users.ClaimMFAStep(ctx, user, step)
		if err != nil {
			return false, err
		}
		ok = claimed
	}

	if !ok {
		a.RecordFailedSignIn(ctx, user)
	}
	return ok, nil
}

// RecordFailedSignIn counts a wrong password or code against the account.
// A failure to record it is logged rather than returned: the attempt is
// refused either way.
func (a *App) RecordFailedSignIn(ctx context.Context, user *model.User) {
	limit, lockout := a.lockoutPolicy()
	locked, err := a.Store.Users.RecordFailedLogin(ctx, user, limit, lockout)
	if err != nil {
		a.Log.Error("failed to record a failed sign in", "user", user.Email, "err", err)
		return
	}
	if locked {
		a.Alert(ctx, config.AlertLockouts, fmt.Sprintf("%s has been locked out", user.Email),
			fmt.Sprintf("%s was locked out of the %s VPN for %d minutes after %d wrong passwords or codes in a row. "+
				"If that wasn't them, someone may be guessing. An administrator can unlock the account sooner from its page.",
				user.Email, a.Config.Organization(), int(lockout.Minutes()), limit))
	}
}

// SignInSucceeded resets the account's failure count once a sign in is
// complete: password and, where required, two-factor code.
func (a *App) SignInSucceeded(ctx context.Context, user *model.User) {
	if user.FailedLogins == 0 && user.LockedUntil == nil {
		return
	}
	if err := a.Store.Users.ClearFailedLogins(ctx, user); err != nil {
		a.Log.Error("failed to clear failed sign ins", "user", user.Email, "err", err)
	}
}

// RecordWebSignIn records a completed sign in to the web application, from
// the given address, and raises the alerts that asks for: a member of staff
// signing in, or anyone signing in from an address they have not used
// before.
func (a *App) RecordWebSignIn(ctx context.Context, user *model.User, address string) {
	// Checked before this sign in is recorded, which would otherwise be the
	// address seen before.
	before, fromHere, err := a.Store.Events.SignInAddresses(ctx, user.ID, address)
	if err != nil {
		a.Log.Error("failed to look up earlier sign ins", "user", user.Email, "err", err)
	}

	a.RecordEvent(ctx, user, model.EventWebLogin, fmt.Sprintf("Logged in to web application from %s.", address))

	if user.IsStaff() {
		a.Alert(ctx, config.AlertAdminSignIn, fmt.Sprintf("%s signed in to administration", user.Email),
			fmt.Sprintf("%s, who can use the administration pages, signed in from %s.", user.Email, address))
	}
	if err == nil && before && !fromHere {
		a.Alert(ctx, config.AlertNewAddress, fmt.Sprintf("%s signed in from a new address", user.Email),
			fmt.Sprintf("%s signed in from %s, an address they haven't signed in from before. "+
				"If that wasn't them, reset their password from their page.", user.Email, address))
	}
}
