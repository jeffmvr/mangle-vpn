package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/jobs"
)

// queuedWebhooks returns the texts of the webhook alerts waiting in the
// queue, oldest first.
func queuedWebhooks(t *testing.T, a *App) []string {
	t.Helper()
	rows, err := a.Store.DB().Query(`SELECT payload FROM "jobs" WHERE kind = ? ORDER BY created_at`, jobs.KindWebhook)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var texts []string
	for rows.Next() {
		var payload string
		rows.Scan(&payload)
		var hook jobs.WebhookPayload
		json.Unmarshal([]byte(payload), &hook)
		texts = append(texts, hook.Text)
	}
	return texts
}

func TestOpenVPNStoppingIsAlertedUnlessAskedFor(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.AlertWebhook, "https://hooks.example.com/abc")

	// The first look only records the state.
	f.a.observeVPN(ctx, true)
	// Then it falls over, and comes back.
	f.a.observeVPN(ctx, false)
	f.a.observeVPN(ctx, false)
	f.a.observeVPN(ctx, true)

	// An administrator stops it on purpose.
	f.a.Config.SetBool(ctx, config.VPNStoppedByAdmin, true)
	f.a.observeVPN(ctx, false)

	got := queuedWebhooks(t, f.a)
	if len(got) != 2 || !strings.Contains(got[0], "OpenVPN has stopped") || !strings.Contains(got[1], "running again") {
		t.Errorf("alerts = %q, want one for stopping and one for coming back", got)
	}

	// With the switch off there are none.
	f.a.Config.SetBool(ctx, config.VPNStoppedByAdmin, false)
	f.a.Config.SetBool(ctx, config.AlertVPNDown, false)
	f.a.observeVPN(ctx, true)
	f.a.observeVPN(ctx, false)
	if len(queuedWebhooks(t, f.a)) != 2 {
		t.Error("alerted with the switch off")
	}
}

func TestExpiringCertificatesAreAlertedOnceADay(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.AlertWebhook, "https://hooks.example.com/abc")
	if err := f.a.CreateAuthority(ctx); err != nil {
		t.Fatal(err)
	}

	// Nothing is near expiry now.
	f.a.checkCertificates(ctx, time.Now())
	if got := queuedWebhooks(t, f.a); len(got) != 0 {
		t.Fatalf("alerted about fresh certificates: %q", got)
	}

	// Seen from much later, the authority is expiring: one alert a day.
	later := f.a.Certificates()[0].NotAfter.Add(-24 * time.Hour)
	f.a.checkCertificates(ctx, later)
	f.a.checkCertificates(ctx, later.Add(time.Hour))
	f.a.checkCertificates(ctx, later.Add(25*time.Hour))
	got := queuedWebhooks(t, f.a)
	if len(got) != 2 || !strings.Contains(got[0], "certificate authority certificate expires") {
		t.Errorf("alerts = %q, want two, a day apart", got)
	}
}

func TestLockoutIsAlerted(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.AlertWebhook, "https://hooks.example.com/abc")

	for range LoginFailureLimit {
		f.a.RecordFailedSignIn(ctx, f.user)
	}
	got := queuedWebhooks(t, f.a)
	if len(got) != 1 || !strings.Contains(got[0], "owner@example.com has been locked out") {
		t.Errorf("alerts = %q, want one lockout", got)
	}
}

func TestWebhookDelivery(t *testing.T) {
	f := newVPNFixture(t, false)

	var received map[string]string
	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)
	}))
	defer hook.Close()

	payload, _ := json.Marshal(jobs.WebhookPayload{URL: hook.URL, Text: "hello"})
	if err := f.a.handleSendWebhook(t.Context(), string(payload)); err != nil {
		t.Fatal(err)
	}
	if received["text"] != "hello" || received["content"] != "hello" {
		t.Errorf("the webhook received %v", received)
	}

	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer failing.Close()
	payload, _ = json.Marshal(jobs.WebhookPayload{URL: failing.URL, Text: "hello"})
	if err := f.a.handleSendWebhook(t.Context(), string(payload)); err == nil {
		t.Error("a refused delivery reported success, so it would not be retried")
	}
}
