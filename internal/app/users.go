package app

import (
	"context"
	"errors"
	"fmt"

	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/mailer"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// SaveUser stores a user and brings their VPN access into line with it.
//
// A user who is no longer active loses their connections, and a user who has
// moved group keeps theirs but is re-pointed at the new group's rules.
func (a *App) SaveUser(ctx context.Context, user *model.User) error {
	previousGroupID := uuid.Nil
	if !user.ID.IsZero() {
		if existing, err := a.Store.Users.Get(ctx, user.ID); err == nil {
			previousGroupID = existing.GroupID
		}
	}

	if err := a.Store.Users.Save(ctx, user); err != nil {
		return err
	}

	if !user.IsActive() {
		return a.DisconnectUserClients(ctx, user.ID)
	}

	// A client's firewall rule names its group's chain, so a move has to be
	// rewritten into the live rules or the user keeps the old group's access.
	if !previousGroupID.IsZero() && previousGroupID != user.GroupID {
		return a.remapUserClients(ctx, user, previousGroupID)
	}
	return nil
}

// remapUserClients repoints a moved user's live connections at their new
// group's chain.
func (a *App) remapUserClients(ctx context.Context, user *model.User, previousGroupID uuid.UUID) error {
	clients, err := a.Store.Clients.ByUser(ctx, user.ID)
	if err != nil {
		return err
	}

	previous, err := a.Store.Groups.Get(ctx, previousGroupID)
	if err != nil {
		return err
	}

	for _, client := range clients {
		// Withdraw against the group the rule was written for, not the one
		// the user has now, or the old rule would be left behind.
		stale := *client
		staleUser := *user
		staleUser.Group = previous
		staleDevice := *client.Device
		staleDevice.User = &staleUser
		stale.Device = &staleDevice

		a.RemoveClientRule(ctx, &stale)
		a.AddClientRule(ctx, client)
	}
	return nil
}

// DeleteUser removes a user along with their devices, connections and
// audit events. Their live connections are dropped first; the database rows
// go, and their certificates are revoked, in one transaction.
func (a *App) DeleteUser(ctx context.Context, user *model.User) error {
	if err := a.DisconnectUserClients(ctx, user.ID); err != nil {
		return err
	}
	if err := a.Store.Users.Delete(ctx, user.ID); err != nil {
		return err
	}
	return a.QueueCRLRebuild(ctx)
}

// ResetUserPassword retires a user's password and e-mails them a link to
// choose a new one, returning the link so an administrator can pass it on
// when mail is not set up. The old password stops working at once, and
// whoever is signed in with it is signed out: a reset is often because
// someone else knows it.
func (a *App) ResetUserPassword(ctx context.Context, user *model.User) (string, error) {
	user.ClearPassword()
	if err := a.Store.Users.SetPassword(ctx, user); err != nil {
		return "", err
	}
	if err := a.Store.Users.EndSessions(ctx, user); err != nil {
		return "", err
	}

	link, err := a.IssuePasswordLink(ctx, user, LinkReset)
	if err != nil {
		return "", err
	}
	if !a.MailConfigured() {
		return link, nil
	}
	subject := fmt.Sprintf("Your %s VPN password has been reset", a.Config.Organization())
	return link, a.queueLinkEmail(ctx, mailer.PasswordTemplate, subject, user.Email, link, LinkReset)
}

// Invitation is one account created by [App.InviteUsers], with the link its
// owner chooses a password through.
type Invitation struct {
	Email string `json:"email"`
	Link  string `json:"link"`

	// Emailed says whether the link was e-mailed to them, which needs both
	// asking for it and outgoing mail being set up.
	Emailed bool `json:"emailed"`
}

// InviteUsers creates an account for each e-mail address in a group,
// returning the new accounts with the links their owners choose a password
// through. With notify set, each is e-mailed their link.
//
// An address that already has an account is moved into the group rather
// than duplicated, and keeps its existing password. It is not re-enabled:
// an account another administrator disabled stays disabled until someone
// enables it on purpose.
func (a *App) InviteUsers(ctx context.Context, emails []string, groupID uuid.UUID, notify bool) ([]Invitation, error) {
	var invited []*model.User

	for _, email := range emails {
		user, err := a.Store.Users.ByEmail(ctx, email)

		switch {
		case errors.Is(err, store.ErrNotFound):
			// A new account has no password until its owner chooses one.
			user = &model.User{Email: email, IsEnabled: true}
			user.ClearPassword()
			invited = append(invited, user)
		case err != nil:
			return nil, err
		}

		user.GroupID = groupID
		if err := a.SaveUser(ctx, user); err != nil {
			return nil, err
		}
	}

	subject := fmt.Sprintf("Welcome to the %s VPN!", a.Config.Organization())
	invitations := make([]Invitation, 0, len(invited))
	for _, user := range invited {
		link, err := a.IssuePasswordLink(ctx, user, LinkInvite)
		if err != nil {
			return invitations, err
		}
		emailed := notify && a.MailConfigured()
		invitations = append(invitations, Invitation{Email: user.Email, Link: link, Emailed: emailed})

		if emailed {
			if err := a.queueLinkEmail(ctx, mailer.WelcomeTemplate, subject, user.Email, link, LinkInvite); err != nil {
				return invitations, err
			}
		}
	}
	return invitations, nil
}

// QueueEmail hands a message to the task queue for delivery.
func (a *App) QueueEmail(ctx context.Context, recipient, subject string, content mailer.Content) error {
	return a.Jobs.Enqueue(ctx, jobs.KindSendEmail, mailer.Message{
		Recipient: recipient,
		Subject:   subject,
		Body:      content.HTML,
		Text:      content.Text,
		Sender:    mailer.SenderFromConfig(a.Config),
	})
}

// RecordEvent appends an entry to the audit log, reporting failures to the
// log rather than to the caller: an audit write must never be the reason a
// sign in or a connection fails.
func (a *App) RecordEvent(ctx context.Context, user *model.User, name, detail string) {
	if user == nil {
		return
	}
	if err := a.Store.Events.Record(ctx, user.ID, name, detail); err != nil {
		a.Log.Error("failed to record event", "event", name, "user", user.Email, "err", err)
	}
}

// MailConfigured reports whether the application can send e-mail.
func (a *App) MailConfigured() bool {
	return mailer.AccountFromConfig(a.Config).IsConfigured()
}
