package web

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
)

func TestVPNSettingsOnlyAcceptTheOfferedChoices(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")

	put := func(body string) int {
		req, _ := http.NewRequest(http.MethodPut, ts.http.URL+"/api/admin/settings/vpn", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrfHeader, b.csrf())
		resp, err := b.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	for _, body := range []string{
		`{"vpn_session_hours": "5"}`,
		`{"vpn_idle_minutes": "60\nscript-security 3"}`,
		`{"vpn_log_level": "11"}`,
	} {
		if got := put(body); got != http.StatusBadRequest {
			t.Errorf("PUT %s = %d, want 400", body, got)
		}
	}

	if got := put(`{"vpn_session_hours": "12", "vpn_idle_minutes": "0", "vpn_log_level": "4", "vpn_device_to_device": true}`); got != http.StatusNoContent {
		t.Fatalf("valid settings = %d, want 204", got)
	}
	ts.app.Config.Reload(t.Context())
	if ts.app.Config.Int(config.VPNSessionHours, 0) != 12 || !ts.app.Config.Bool(config.VPNDeviceToDevice, false) ||
		ts.app.Config.Get(config.VPNIdleMinutes) != "0" {
		t.Error("the settings were not stored")
	}
	if !ts.app.Config.Bool(config.VPNRestartPending, false) {
		t.Error("saving OpenVPN settings did not flag a restart")
	}
}

func TestGeneralSettingsValidation(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("admin@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")

	put := func(body string) int {
		req, _ := http.NewRequest(http.MethodPut, ts.http.URL+"/api/admin/settings/app", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrfHeader, b.csrf())
		resp, err := b.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	for _, body := range []string{
		`{"app_letsencrypt": "True", "app_hostname": "10.0.0.5"}`,
		`{"app_acme_email": "not an address"}`,
		`{"app_event_retention_days": "7"}`,
	} {
		if got := put(body); got != http.StatusBadRequest {
			t.Errorf("PUT %s = %d, want 400", body, got)
		}
	}
	if got := put(`{"app_event_retention_days": "90", "app_letsencrypt": "False"}`); got != http.StatusNoContent {
		t.Errorf("valid settings = %d, want 204", got)
	}

	// The Google domain restriction is stored lower case and must be a domain.
	putAuth := func(body string) int {
		req, _ := http.NewRequest(http.MethodPut, ts.http.URL+"/api/admin/settings/auth", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(csrfHeader, b.csrf())
		resp, _ := b.client.Do(req)
		resp.Body.Close()
		return resp.StatusCode
	}
	base := `"oauth2_provider": "google", "oauth2_client_id": "id", "oauth2_client_secret": "secret"`
	if got := putAuth(`{` + base + `, "oauth2_allowed_domain": "not a domain"}`); got != http.StatusBadRequest {
		t.Errorf("bad domain = %d, want 400", got)
	}
	if got := putAuth(`{` + base + `, "oauth2_allowed_domain": "Example.COM"}`); got != http.StatusNoContent {
		t.Errorf("good domain = %d, want 204", got)
	}
	ts.app.Config.Reload(t.Context())
	if got := ts.app.Config.Get(config.OAuth2AllowedDomain); got != "example.com" {
		t.Errorf("domain stored as %q", got)
	}
}
