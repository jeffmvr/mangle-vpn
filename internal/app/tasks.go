package app

import (
	"context"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/mailer"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
)

// crlInterval is how often the certificate revocation list is rebuilt.
//
// OpenVPN reads the list from disk, so a device deleted between rebuilds can
// still connect until the next one. The list is also rebuilt at start up so
// that a restart picks up anything missed while the worker was down.
const crlInterval = time.Hour

// RegisterTasks attaches the application's job handlers and scheduled work
// to a worker.
func RegisterTasks(a *App, worker *jobs.Worker) {
	worker.Handle(jobs.KindSendEmail, a.handleSendEmail)
	worker.Handle(jobs.KindKillClient, a.handleKillClient)
	worker.Handle(jobs.KindCreateCRL, a.handleCreateCRL)
	worker.Handle(jobs.KindWebhook, a.handleSendWebhook)

	worker.Every(jobs.Periodic{
		Name:    "certificate revocation list",
		Every:   crlInterval,
		OnStart: true,
		Run:     a.handleScheduledCRL,
	})

	worker.Every(jobs.Periodic{
		Name:    "audit log pruning",
		Every:   24 * time.Hour,
		OnStart: true,
		Run:     a.PruneEvents,
	})

	worker.Every(jobs.Periodic{
		Name:    "OpenVPN health check",
		Every:   time.Minute,
		OnStart: true,
		Run:     a.CheckVPNHealth,
	})

	worker.Every(jobs.Periodic{
		Name:    "certificate expiry check",
		Every:   6 * time.Hour,
		OnStart: true,
		Run:     a.CheckCertificates,
	})

	worker.Every(jobs.Periodic{
		Name:    "daily summary",
		Every:   15 * time.Minute,
		OnStart: true,
		Run:     a.SendDailyDigest,
	})

	worker.Every(jobs.Periodic{
		Name:    "nightly backup",
		Every:   24 * time.Hour,
		OnStart: true,
		Run:     a.ScheduledBackup,
	})

	worker.Every(jobs.Periodic{
		Name:    "unused device retirement",
		Every:   24 * time.Hour,
		OnStart: true,
		Run:     a.RetireIdleDevices,
	})

	worker.Every(jobs.Periodic{
		Name:    "expired session and link sweep",
		Every:   time.Hour,
		OnStart: true,
		Run:     a.handleSessionSweep,
	})
}

// handleSendEmail delivers a queued message.
func (a *App) handleSendEmail(ctx context.Context, payload string) error {
	var msg mailer.Message
	if err := jobs.Decode(payload, &msg); err != nil {
		return err
	}

	// Settings may have changed since the message was queued, so read the
	// account fresh rather than trusting this process's cache.
	if err := a.Config.Reload(ctx); err != nil {
		return err
	}

	account := mailer.AccountFromConfig(a.Config)
	if !account.IsConfigured() {
		// Retrying will not help until an administrator fills the settings
		// in, so the message is dropped rather than queued forever.
		a.Log.Error("discarding an e-mail because SMTP is not configured",
			"recipient", msg.Recipient, "subject", msg.Subject)
		return nil
	}

	err := mailer.Send(ctx, account, msg)
	if err != nil {
		return err
	}

	a.Log.Info("e-mail sent", "recipient", msg.Recipient, "subject", msg.Subject)
	return nil
}

// handleKillClient drops a VPN connection through the management socket.
func (a *App) handleKillClient(ctx context.Context, payload string) error {
	var kill jobs.KillClientPayload
	if err := jobs.Decode(payload, &kill); err != nil {
		return err
	}

	if err := openvpn.KillClient(ctx, a.ManagementSocket(), kill.Address); err != nil {
		// A server that is not running has no connection left to drop, so
		// this is already the outcome that was wanted.
		a.Log.Info("could not disconnect a client", "address", kill.Address, "err", err)
		return nil
	}

	a.Log.Info("disconnected an OpenVPN client", "address", kill.Address)
	return nil
}

// handleCreateCRL rebuilds the revocation list on demand.
func (a *App) handleCreateCRL(ctx context.Context, _ string) error {
	return a.WriteCRL(ctx)
}

// handleScheduledCRL rebuilds the revocation list on a schedule, treating a
// missing certificate authority as nothing to do: before setup there is no
// list to publish.
func (a *App) handleScheduledCRL(ctx context.Context) error {
	if err := a.Config.Reload(ctx); err != nil {
		return err
	}
	if a.Config.Get(config.CACertificate) == "" {
		return nil
	}
	return a.WriteCRL(ctx)
}

// PruneEvents removes audit events and connection history older than the
// retention the settings ask for. A retention of 0 keeps everything.
func (a *App) PruneEvents(ctx context.Context) error {
	if err := a.Config.Reload(ctx); err != nil {
		return err
	}
	days := a.Config.Int(config.AppEventRetentionDays, 0)
	if days <= 0 {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	removed, err := a.Store.Events.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}
	if removed > 0 {
		a.Log.Info("pruned the audit log", "removed", removed, "retention_days", days)
	}

	// The connection history is kept as long as the audit log.
	sessions, err := a.Store.VPNSessions.DeleteOlderThan(ctx, cutoff)
	if err != nil {
		return err
	}
	if sessions > 0 {
		a.Log.Info("pruned the connection history", "removed", sessions, "retention_days", days)
	}
	return nil
}

// handleSessionSweep clears out lapsed web sessions and password links.
func (a *App) handleSessionSweep(ctx context.Context) error {
	removed, err := a.Store.Sessions().DeleteExpired(ctx)
	if err != nil {
		return err
	}
	if removed > 0 {
		a.Log.Debug("cleared expired sessions", "count", removed)
	}

	links, err := a.Store.PasswordLinks.DeleteExpired(ctx)
	if err != nil {
		return err
	}
	if links > 0 {
		a.Log.Debug("cleared used and expired password links", "count", links)
	}
	return nil
}
