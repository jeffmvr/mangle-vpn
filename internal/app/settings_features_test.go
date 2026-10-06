package app

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
)

func TestDeviceCertificatesLastAsLongAsTheSettingSays(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	if err := f.a.CreateAuthority(ctx); err != nil {
		t.Fatal(err)
	}
	f.a.Config.Set(ctx, config.PKIDeviceDays, "90")

	device, err := f.a.CreateDevice(ctx, f.user, "phone", "")
	if err != nil {
		t.Fatal(err)
	}
	kp, err := f.a.IssueDeviceKeyPair(ctx, device)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode([]byte(kp.CertificatePEM()))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if days := time.Until(cert.NotAfter).Hours() / 24; days < 89 || days > 91 {
		t.Errorf("the certificate lasts %.0f days, want 90", days)
	}
}

func TestServerConfigSharesThePortAndLowersThePacketSize(t *testing.T) {
	tcp, err := openvpn.ServerConfig{Protocol: "tcp", Subnet: "10.8.0.0/24", BindPort: 443, PortShare: 8443, MSSFix: 1300}.Render()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tcp, "port-share 127.0.0.1 8443\n") {
		t.Error("TCP with port sharing lacks the port-share line")
	}
	if strings.Contains(tcp, "mssfix") {
		t.Error("mssfix was written for TCP, which it does nothing for")
	}

	udp, _ := openvpn.ServerConfig{Protocol: "udp", Subnet: "10.8.0.0/24", BindPort: 1194, PortShare: 8443, MSSFix: 1300}.Render()
	if strings.Contains(udp, "port-share") || !strings.Contains(udp, "mssfix 1300\n") {
		t.Errorf("UDP config wrong:\n%s", udp)
	}
}

func TestOneConnectionPerPersonDisconnectsTheOtherDevice(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.VPNSingleSession, "True")

	phone := &model.Device{Name: "phone", UserID: f.user.ID, Fingerprint: "CC:DD", Serial: "2"}
	f.a.Store.Devices.Save(ctx, phone)

	connect := func(name, fingerprint, ip string) {
		t.Helper()
		if err := f.a.ConnectVPNClient(ctx, VPNConnection{
			CommonName: "owner@example.com:" + name, Fingerprint: fingerprint,
			VirtualIP: ip, RemoteIP: "203.0.113.4", RemotePort: "5555",
		}); err != nil {
			t.Fatal(err)
		}
	}
	connect("laptop", "AA:BB", "172.25.0.7")
	connect("phone", "CC:DD", "172.25.0.8")

	clients, _ := f.a.Store.Clients.ByUser(ctx, f.user.ID)
	if len(clients) != 1 || clients[0].Device.Name != "phone" {
		t.Errorf("connected: %d clients, want only the phone", len(clients))
	}
}

func TestNewDeviceAndSignInAlerts(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.AlertWebhook, "https://hooks.example.com/abc")
	f.a.Config.Set(ctx, config.AlertNewDevice, "True")
	f.a.Config.Set(ctx, config.AlertNewAddress, "True")
	f.a.Config.Set(ctx, config.AlertAdminSignIn, "True")

	f.a.CreateDevice(ctx, f.user, "tablet", "")

	// The first sign in is not from a "new" address: there is nothing to
	// compare with. The same address again is not either. Another one is.
	f.a.RecordWebSignIn(ctx, f.user, "203.0.113.4")
	f.a.RecordWebSignIn(ctx, f.user, "203.0.113.4")
	f.a.RecordWebSignIn(ctx, f.user, "198.51.100.9")

	// A member of staff signing in is alerted on whatever the address.
	f.user.IsAdmin = true
	f.a.RecordWebSignIn(ctx, f.user, "198.51.100.9")

	got := queuedWebhooks(t, f.a)
	want := []string{"added a device", "signed in from a new address", "signed in to administration"}
	if len(got) != len(want) {
		t.Fatalf("alerts = %q, want %d", got, len(want))
	}
	for i, w := range want {
		if !strings.Contains(got[i], w) {
			t.Errorf("alert %d = %q, want it to mention %q", i, got[i], w)
		}
	}
}

func TestDailySummaryIsSentOnceAfterEight(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.AlertWebhook, "https://hooks.example.com/abc")
	f.a.Config.Set(ctx, config.AlertDailyDigest, "True")
	f.a.RecordWebSignIn(ctx, f.user, "203.0.113.4")

	day := time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)
	f.a.sendDailyDigest(ctx, day.Add(7*time.Hour))
	f.a.sendDailyDigest(ctx, day.Add(9*time.Hour))
	f.a.sendDailyDigest(ctx, day.Add(15*time.Hour))
	f.a.sendDailyDigest(ctx, day.Add(33*time.Hour))

	got := queuedWebhooks(t, f.a)
	if len(got) != 2 {
		t.Fatalf("summaries = %d, want one a day after 8: %q", len(got), got)
	}
	if !strings.Contains(got[0], "Sign-ins: 1 (0 failed)") {
		t.Errorf("summary = %q", got[0])
	}
}
