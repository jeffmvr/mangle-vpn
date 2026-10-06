package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/mailer"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
)

// Alerts tell administrators about something that needs them, by e-mail to
// the addresses in the settings and to a webhook such as a Slack channel's.
// They are queued like any other job, so a slow mail server or chat service
// never holds up the sign in or check that raised them.

// webhookTimeout bounds a webhook delivery.
const webhookTimeout = 10 * time.Second

// Alert sends an alert, when the switch named by kind is on. kind is one of
// the config.Alert* switches, or "" for an alert that is always sent.
func (a *App) Alert(ctx context.Context, kind, subject, body string) {
	if kind != "" && !a.Config.Bool(kind, true) {
		return
	}
	if err := a.sendAlert(ctx, subject, body); err != nil {
		a.Log.Error("failed to queue an alert", "subject", subject, "err", err)
	}
}

// SendTestAlert sends an alert through every configured channel, for an
// administrator checking them. It reports whether there was any channel.
func (a *App) SendTestAlert(ctx context.Context) (bool, error) {
	if len(a.alertEmails()) == 0 && a.Config.Get(config.AlertWebhook) == "" {
		return false, nil
	}
	return true, a.sendAlert(ctx, "Test alert",
		"This is a test alert from the "+a.Config.Organization()+" VPN. Alerts are reaching you.")
}

// sendAlert queues an alert for every configured channel.
func (a *App) sendAlert(ctx context.Context, subject, body string) error {
	subject = fmt.Sprintf("[%s VPN] %s", a.Config.Organization(), subject)

	if a.MailConfigured() {
		content := mailer.Content{Text: body + "\n", HTML: "<p>" + htmlEscape(body) + "</p>"}
		for _, address := range a.alertEmails() {
			if err := a.QueueEmail(ctx, address, subject, content); err != nil {
				return err
			}
		}
	}

	if url := a.Config.Get(config.AlertWebhook); url != "" {
		return a.Jobs.Enqueue(ctx, jobs.KindWebhook, jobs.WebhookPayload{URL: url, Text: subject + "\n" + body})
	}
	return nil
}

// alertEmails returns the addresses alerts are e-mailed to.
func (a *App) alertEmails() []string {
	return strings.Fields(strings.NewReplacer(",", " ", ";", " ").Replace(a.Config.Get(config.AlertEmails)))
}

// handleSendWebhook delivers an alert to a webhook. The body carries the
// text as "text", which Slack, Mattermost, Rocket.Chat, Google Chat and
// Microsoft Teams read, and as "content", which Discord reads.
func (a *App) handleSendWebhook(ctx context.Context, payload string) error {
	var hook jobs.WebhookPayload
	if err := jobs.Decode(payload, &hook); err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]string{"text": hook.Text, "content": hook.Text})

	ctx, cancel := context.WithTimeout(ctx, webhookTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("app: webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("app: deliver a webhook: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("app: the webhook answered %s", resp.Status)
	}
	return nil
}

// CheckVPNHealth alerts when OpenVPN stops without anyone asking it to,
// and again when it is back.
func (a *App) CheckVPNHealth(ctx context.Context) error {
	if err := a.Config.Reload(ctx); err != nil {
		return err
	}
	if !a.Config.Bool(config.AppInstalled, false) {
		return nil
	}
	return a.observeVPN(ctx, openvpn.IsRunning(ctx))
}

// observeVPN compares OpenVPN's state with the last one seen, alerting on
// a change that nobody asked for.
func (a *App) observeVPN(ctx context.Context, running bool) error {
	state := "down"
	if running {
		state = "up"
	}
	previous := a.Config.Get(config.AlertVPNState)
	if previous == state {
		return nil
	}
	if err := a.Config.Set(ctx, config.AlertVPNState, state); err != nil {
		return err
	}

	switch {
	case previous == "":
		// The first look: nothing to compare with.
	case !running && !a.Config.Bool(config.VPNStoppedByAdmin, false):
		a.Alert(ctx, config.AlertVPNDown, "OpenVPN has stopped",
			"The OpenVPN server stopped without anyone stopping it, so nobody can connect. "+
				"The server log under Logs › OpenVPN usually says why.")
	case running && previous == "down" && !a.Config.Bool(config.VPNStoppedByAdmin, false):
		a.Alert(ctx, config.AlertVPNDown, "OpenVPN is running again", "The OpenVPN server is back and accepting connections.")
	}
	return nil
}

// CheckCertificates alerts once a day about each certificate the server
// depends on that expires within CertificateWarning.
func (a *App) CheckCertificates(ctx context.Context) error {
	if err := a.Config.Reload(ctx); err != nil {
		return err
	}
	return a.checkCertificates(ctx, time.Now())
}

// checkCertificates is CheckCertificates at a given time.
func (a *App) checkCertificates(ctx context.Context, now time.Time) error {
	today := now.Format(time.DateOnly)
	if a.Config.Get(config.AlertCertificatesDay) == today {
		return nil
	}

	var expiring []string
	for _, certificate := range a.Certificates() {
		if !certificate.ExpiresSoon(now) {
			continue
		}
		when := "expires on " + certificate.NotAfter.Format("January 2, 2006")
		if now.After(certificate.NotAfter) {
			when = "expired on " + certificate.NotAfter.Format("January 2, 2006")
		}
		expiring = append(expiring, fmt.Sprintf("The %s certificate %s.", strings.ToLower(certificate.Label), when))
	}
	if len(expiring) == 0 {
		return nil
	}

	a.Alert(ctx, config.AlertCertificates, "A certificate is about to expire",
		strings.Join(expiring, " ")+" Replace it from Settings › General before then, or connections will fail.")
	return a.Config.Set(ctx, config.AlertCertificatesDay, today)
}

// htmlEscape makes text safe to put in an HTML e-mail.
func htmlEscape(text string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(text)
}

// digestHour is the hour of the day, in the server's time, after which the
// daily summary is sent.
const digestHour = 8

// SendDailyDigest e-mails the daily summary, once a day after digestHour,
// when it is switched on.
func (a *App) SendDailyDigest(ctx context.Context) error {
	if err := a.Config.Reload(ctx); err != nil {
		return err
	}
	return a.sendDailyDigest(ctx, time.Now())
}

// sendDailyDigest is SendDailyDigest at a given time.
func (a *App) sendDailyDigest(ctx context.Context, now time.Time) error {
	today := now.Format(time.DateOnly)
	if !a.Config.Bool(config.AlertDailyDigest, false) || now.Hour() < digestHour ||
		a.Config.Get(config.AlertDigestDay) == today {
		return nil
	}

	since := now.Add(-24 * time.Hour)
	counts, err := a.Store.Events.CountSince(ctx, since)
	if err != nil {
		return err
	}
	connections, traffic, err := a.Store.VPNSessions.TrafficSince(ctx, since)
	if err != nil {
		return err
	}

	adminChanges := 0
	for name, count := range counts {
		if strings.HasPrefix(name, "admin.") {
			adminChanges += count
		}
	}

	lines := []string{
		fmt.Sprintf("In the last 24 hours on the %s VPN:", a.Config.Organization()),
		"",
		fmt.Sprintf("Sign-ins: %d (%d failed)", counts["web.login"], counts["web.error"]),
		fmt.Sprintf("VPN connections finished: %d, moving %s", connections, formatBytes(traffic)),
		fmt.Sprintf("Connections refused: %d", counts["vpn.error"]),
		fmt.Sprintf("Devices added: %d, removed: %d", counts["device.create"]+counts["device.import"],
			counts["device.delete"]+counts["device.retire"]+counts["admin.device.delete"]),
		fmt.Sprintf("Changes by administrators: %d", adminChanges),
	}
	a.Alert(ctx, "", "Daily summary", strings.Join(lines, "\n"))
	return a.Config.Set(ctx, config.AlertDigestDay, today)
}

// formatBytes renders a byte count as "1.2 GB".
func formatBytes(n int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value, unit := float64(n), 0
	for value >= 1000 && unit < len(units)-1 {
		value /= 1000
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", n)
	}
	return fmt.Sprintf("%.1f %s", value, units[unit])
}
