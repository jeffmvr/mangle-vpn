package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/mailer"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// People choose their own passwords through single-use links, rather than
// being sent one: an invitation, an administrator's reset, and a forgotten
// password all e-mail a link to a page where the person picks a password.
// Nothing in the e-mail can be used to sign in once the link has been used
// or has lapsed.

// What a password link was issued for, which decides how long it lasts.
const (
	LinkInvite = "invite"
	LinkReset  = "reset"
	LinkForgot = "forgot"
)

// linkLifetimes is how long each kind of link stays usable. A forgotten
// password link is asked for by whoever typed the address, so it is short.
var linkLifetimes = map[string]time.Duration{
	LinkInvite: 7 * 24 * time.Hour,
	LinkReset:  3 * 24 * time.Hour,
	LinkForgot: time.Hour,
}

// linkCodeBytes is the size of a link's random code.
const linkCodeBytes = 32

// ErrLinkInvalid is returned for a password link that does not exist, has
// lapsed, has been used, or belongs to an account that can no longer sign
// in.
var ErrLinkInvalid = errors.New("app: the password link has expired or has already been used")

// IssuePasswordLink creates a link for the user to choose a password
// through, retiring any link issued before, and returns its address.
func (a *App) IssuePasswordLink(ctx context.Context, user *model.User, purpose string) (string, error) {
	lifetime, ok := linkLifetimes[purpose]
	if !ok {
		return "", fmt.Errorf("app: unknown password link purpose %q", purpose)
	}

	raw := make([]byte, linkCodeBytes)
	rand.Read(raw)
	code := hex.EncodeToString(raw)

	if err := a.Store.PasswordLinks.DeleteForUser(ctx, user.ID); err != nil {
		return "", err
	}
	if err := a.Store.PasswordLinks.Create(ctx, user.ID, hashLinkCode(code), purpose,
		time.Now().Add(lifetime)); err != nil {
		return "", err
	}
	return a.Config.URL("password/set") + "?code=" + code, nil
}

// PasswordLinkUser returns the account a password link is for and what the
// link was issued for, without using the link up, or ErrLinkInvalid.
func (a *App) PasswordLinkUser(ctx context.Context, code string) (*model.User, string, error) {
	userID, purpose, err := a.Store.PasswordLinks.Find(ctx, hashLinkCode(code))
	if err != nil {
		return nil, "", linkError(err)
	}
	user, err := a.activeLinkUser(ctx, userID)
	return user, purpose, err
}

// RedeemPasswordLink sets the password of the account a link is for, using
// the link up, and its full name when one is given, as it is when an
// invitation is accepted. Every session the account had is ended and every
// other link retired, since whoever asked for the link may have done so
// because the old password was known to someone else.
func (a *App) RedeemPasswordLink(ctx context.Context, code, password, name string) (*model.User, error) {
	userID, purpose, err := a.Store.PasswordLinks.Take(ctx, hashLinkCode(code))
	if err != nil {
		return nil, linkError(err)
	}
	user, err := a.activeLinkUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	user.SetPassword(password)
	user.PasswordChange = false
	user.PasswordExpire = nil

	if err := a.Store.Users.SetPassword(ctx, user); err != nil {
		return nil, err
	}
	if name != "" {
		user.Name = name
		if err := a.Store.Users.SetName(ctx, user); err != nil {
			return nil, err
		}
	}
	// Proving the mailbox is as good as the password, so a lockout ends.
	if err := a.Store.Users.ClearFailedLogins(ctx, user); err != nil {
		return nil, err
	}
	if err := a.Store.Users.EndSessions(ctx, user); err != nil {
		return nil, err
	}
	if err := a.Store.PasswordLinks.DeleteForUser(ctx, user.ID); err != nil {
		return nil, err
	}

	detail := "Chose a new password through an e-mailed link."
	if purpose == LinkInvite {
		detail = "Chose a password through their invitation."
	}
	a.RecordEvent(ctx, user, model.EventAccountPassword, detail)
	return user, nil
}

// activeLinkUser loads the account a link is for, refusing one that can no
// longer sign in.
func (a *App) activeLinkUser(ctx context.Context, id uuid.UUID) (*model.User, error) {
	user, err := a.Store.Users.Get(ctx, id)
	if err != nil {
		return nil, linkError(err)
	}
	if !user.IsActive() {
		return nil, ErrLinkInvalid
	}
	return user, nil
}

// linkError reports a missing link as ErrLinkInvalid and passes anything
// else through.
func linkError(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return ErrLinkInvalid
	}
	return err
}

// hashLinkCode returns the form a link's code is stored in.
func hashLinkCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// ForgotPassword e-mails the account with the given address a link to
// choose a new password. It says nothing about whether the account exists:
// an address with no active account, or no outgoing mail, is quietly
// passed over, so the sign in page cannot be used to find out who has one.
// The current password keeps working until the link is used. With
// adminsOnly, only administrators are sent one.
func (a *App) ForgotPassword(ctx context.Context, email string, adminsOnly bool) error {
	if !a.MailConfigured() {
		return nil
	}

	user, err := a.Store.Users.ByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !user.IsActive() || adminsOnly && !user.IsAdmin {
		return nil
	}

	link, err := a.IssuePasswordLink(ctx, user, LinkForgot)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("Reset your %s VPN password", a.Config.Organization())
	return a.queueLinkEmail(ctx, mailer.ForgotTemplate, subject, user.Email, link, LinkForgot)
}

// queueLinkEmail e-mails someone a password link, using one of the link
// templates.
func (a *App) queueLinkEmail(ctx context.Context, template, subject, email, link, purpose string) error {
	content, err := mailer.Render(template, mailer.LinkData{
		Email:        email,
		Organization: a.Config.Organization(),
		Link:         link,
		Lasts:        describeLifetime(linkLifetimes[purpose]),
		URL:          a.Config.URL(),
	})
	if err != nil {
		return err
	}
	return a.QueueEmail(ctx, email, subject, content)
}

// describeLifetime renders a link's lifetime for an e-mail: "1 hour",
// "3 days".
func describeLifetime(d time.Duration) string {
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}
