package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
)

func TestHealthCheck(t *testing.T) {
	ts := newTestServer(t)

	// Works before setup too: an uptime check should not depend on it.
	resp, err := http.Get(ts.http.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"ok"`) {
		t.Errorf("healthz = %d %s", resp.StatusCode, body)
	}
}

func TestMetrics(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	if err := ts.app.CreateAuthority(t.Context()); err != nil {
		t.Fatal(err)
	}
	ts.addUser("person@example.com", false)

	metrics := httptest.NewServer(MetricsHandler(ts.app))
	defer metrics.Close()

	resp, err := http.Get(metrics.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	for _, want := range []string{
		"mangle_users 1",
		"mangle_clients_connected 0",
		"mangle_users_locked 0",
		`mangle_certificate_expiry_timestamp_seconds{certificate="ca"}`,
		"mangle_openvpn_up",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("metrics lack %q", want)
		}
	}
}

func TestCertificatesAPI(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	if err := ts.app.CreateAuthority(t.Context()); err != nil {
		t.Fatal(err)
	}
	ts.addUser("admin@example.com", false)

	b := ts.browser()
	b.login("admin@example.com", "Password1")
	resp, err := b.client.Get(ts.http.URL + "/api/admin/certificates")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), `"name":"ca"`) ||
		!strings.Contains(string(body), `"expires_soon":false`) {
		t.Errorf("certificates = %d %s", resp.StatusCode, body)
	}
}

func TestRenewVPNCertificate(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	if err := ts.app.CreateAuthority(t.Context()); err != nil {
		t.Fatal(err)
	}
	ts.addUser("admin@example.com", false)
	b := ts.browser()
	b.login("admin@example.com", "Password1")

	before := ts.app.Config.Get(config.VPNCertificate)
	status, body := b.do(http.MethodPost, "/api/admin/certificates/openvpn/renew", "")
	if status != http.StatusOK || !strings.Contains(string(body), `"name":"openvpn"`) {
		t.Fatalf("renew = %d: %s", status, body)
	}

	after := ts.app.Config.Get(config.VPNCertificate)
	if after == "" || after == before {
		t.Error("the server certificate was not replaced")
	}
	kp, err := pki.LoadKeyPair(after, ts.app.Config.Get(config.VPNPrivateKey))
	if err != nil {
		t.Errorf("the new certificate and key do not match: %v", err)
	}
	ca, _ := pki.ParseCertificate(ts.app.Config.Get(config.CACertificate))
	if err := kp.Certificate.CheckSignatureFrom(ca); err != nil {
		t.Errorf("the new certificate is not signed by the authority: %v", err)
	}
	if !ts.app.Config.Bool(config.VPNRestartPending, false) {
		t.Error("renewing did not flag OpenVPN for a restart")
	}
}
