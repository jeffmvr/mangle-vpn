package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

func TestAssignStaticIP(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.VPNSubnet, "10.8.0.0/24")

	other := &model.Device{Name: "phone", UserID: f.user.ID, Fingerprint: "CC:DD", Serial: "2"}
	f.a.Store.Devices.Save(ctx, other)

	// The first free address is the bottom of the upper half.
	if err := f.a.AssignStaticIP(ctx, f.device, AutomaticAddress); err != nil || f.device.StaticIP != "10.8.0.128" {
		t.Fatalf("auto = %q, %v; want 10.8.0.128", f.device.StaticIP, err)
	}
	if err := f.a.AssignStaticIP(ctx, other, AutomaticAddress); err != nil || other.StaticIP != "10.8.0.129" {
		t.Fatalf("second auto = %q, %v; want 10.8.0.129", other.StaticIP, err)
	}

	for requested, want := range map[string]error{
		"10.8.0.128": ErrAddressTaken,      // the laptop's
		"10.8.0.20":  ErrAddressOutOfRange, // in the pool
		"10.8.0.255": ErrAddressOutOfRange, // broadcast
		"10.9.0.200": ErrAddressOutOfRange, // another network
		"bogus":      ErrAddressOutOfRange,
	} {
		if err := f.a.AssignStaticIP(ctx, other, requested); !errors.Is(err, want) {
			t.Errorf("assign %s = %v, want %v", requested, err, want)
		}
	}

	if err := f.a.AssignStaticIP(ctx, other, "10.8.0.200"); err != nil {
		t.Errorf("a free address in range: %v", err)
	}
	if err := f.a.AssignStaticIP(ctx, other, ""); err != nil || other.StaticIP != "" {
		t.Errorf("clearing = %q, %v", other.StaticIP, err)
	}

	// In a larger subnet the fixed range starts on a .0, which is skipped.
	f.a.Config.Set(ctx, config.VPNSubnet, "172.25.0.0/16")
	f.a.AssignStaticIP(ctx, f.device, "")
	if err := f.a.AssignStaticIP(ctx, f.device, AutomaticAddress); err != nil || f.device.StaticIP != "172.25.128.1" {
		t.Errorf("auto in a /16 = %q, %v; want 172.25.128.1", f.device.StaticIP, err)
	}
}

func TestConnectWritesTheClientsOwnSettings(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.VPNSubnet, "10.8.0.0/24")

	group := f.user.Group
	group.Routes = "10.0.10.0/24\n192.168.5.0/24"
	group.Nameservers = "10.0.0.53"
	f.a.Store.Groups.Save(ctx, group)
	f.a.AssignStaticIP(ctx, f.device, "10.8.0.150")

	configFile := filepath.Join(t.TempDir(), "client-connect.tmp")
	err := f.a.ConnectVPNClient(ctx, VPNConnection{
		CommonName: "owner@example.com:laptop", Fingerprint: "AA:BB",
		VirtualIP: "10.8.0.7", RemoteIP: "203.0.113.4", RemotePort: "5555",
		ConfigFile: configFile,
	})
	if err != nil {
		t.Fatal(err)
	}

	written, _ := os.ReadFile(configFile)
	for _, line := range []string{
		"ifconfig-push 10.8.0.150 255.255.255.0",
		`push "route 10.0.10.0 255.255.255.0"`,
		`push "route 192.168.5.0 255.255.255.0"`,
		`push "dhcp-option DNS 10.0.0.53"`,
	} {
		if !strings.Contains(string(written), line+"\n") {
			t.Errorf("the client's settings lack %q:\n%s", line, written)
		}
	}

	// The connection is recorded at the fixed address, which is the one the
	// firewall rule must name, not the pool address OpenVPN offered.
	client, err := f.a.Store.Clients.ByCommonName(ctx, "owner@example.com:laptop")
	if err != nil || client.VirtualIP != "10.8.0.150" {
		t.Errorf("recorded at %q, %v; want 10.8.0.150", client.VirtualIP, err)
	}
}

func TestFixedAddressOutsideANewSubnetFallsBackToThePool(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.VPNSubnet, "10.8.0.0/24")
	f.a.AssignStaticIP(ctx, f.device, "10.8.0.150")
	f.a.Config.Set(ctx, config.VPNSubnet, "172.25.0.0/16")

	configFile := filepath.Join(t.TempDir(), "client-connect.tmp")
	if err := f.a.ConnectVPNClient(ctx, VPNConnection{
		CommonName: "owner@example.com:laptop", Fingerprint: "AA:BB",
		VirtualIP: "172.25.0.9", RemoteIP: "203.0.113.4", RemotePort: "5555",
		ConfigFile: configFile,
	}); err != nil {
		t.Fatal(err)
	}
	if written, _ := os.ReadFile(configFile); strings.Contains(string(written), "ifconfig-push") {
		t.Errorf("pushed an address outside the subnet:\n%s", written)
	}
}

func TestDisconnectKeepsTheConnectionsHistory(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()

	if err := f.a.ConnectVPNClient(ctx, VPNConnection{
		CommonName: "owner@example.com:laptop", Fingerprint: "AA:BB",
		VirtualIP: "172.25.0.7", RemoteIP: "203.0.113.4", RemotePort: "5555",
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.a.DisconnectVPNClient(ctx, VPNDisconnection{
		CommonName: "owner@example.com:laptop", RemoteIP: "203.0.113.4",
		BytesReceived: 1000, BytesSent: 250000,
	}); err != nil {
		t.Fatal(err)
	}

	page, err := f.a.Store.VPNSessions.ByUser(ctx, f.user.ID, store.Query{Size: 10, Page: 1})
	if err != nil || page.Total != 1 {
		t.Fatalf("history = %d sessions, %v; want 1", page.Total, err)
	}
	got := page.Items[0]
	if got.DeviceName != "laptop" || got.VirtualIP != "172.25.0.7" || got.BytesSent != 250000 || got.BytesReceived != 1000 {
		t.Errorf("recorded %+v", got)
	}

	// Deleting the user takes their history with them.
	if err := f.a.DeleteUser(ctx, f.user); err != nil {
		t.Fatal(err)
	}
	var left int
	f.a.Store.DB().QueryRow(`SELECT COUNT(*) FROM "vpn_sessions"`).Scan(&left)
	if left != 0 {
		t.Errorf("%d sessions outlived their user", left)
	}
}

func TestRetireIdleDevices(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	db := f.a.Store.DB()

	stale := &model.Device{Name: "old-laptop", UserID: f.user.ID, Fingerprint: "EE:FF", Serial: "3"}
	f.a.Store.Devices.Save(ctx, stale)
	db.Exec(`UPDATE "devices" SET last_login = '2026-01-01 00:00:00' WHERE id = ?`, stale.ID)

	neverUsed := &model.Device{Name: "unused", UserID: f.user.ID}
	f.a.Store.Devices.Save(ctx, neverUsed)
	db.Exec(`UPDATE "devices" SET created_at = '2026-01-01 00:00:00' WHERE id = ?`, neverUsed.ID)

	// The fixture's laptop was made just now, so it stays either way.

	// With the group keeping devices forever, nothing goes.
	if err := f.a.RetireIdleDevices(ctx); err != nil {
		t.Fatal(err)
	}
	if devices, _ := f.a.Store.Devices.ByUser(ctx, f.user.ID); len(devices) != 3 {
		t.Fatalf("%d devices left with retirement off, want 3", len(devices))
	}

	group := f.user.Group
	group.DeviceIdleDays = 30
	f.a.Store.Groups.Save(ctx, group)
	if err := f.a.RetireIdleDevices(ctx); err != nil {
		t.Fatal(err)
	}

	devices, _ := f.a.Store.Devices.ByUser(ctx, f.user.ID)
	if len(devices) != 1 || devices[0].Name != "laptop" {
		t.Fatalf("left %v, want only the recent laptop", devices)
	}
	serials, _ := f.a.Store.RevokedDevices.Serials(ctx)
	if len(serials) != 1 || serials[0] != "3" {
		t.Errorf("revoked %v, want the stale laptop's certificate", serials)
	}
}
