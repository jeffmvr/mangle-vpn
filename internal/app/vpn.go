package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/iptables"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/provision"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// This file holds what the OpenVPN server's hooks do: write its
// configuration before it starts, build and tear down its firewall around
// it, and decide on each client that connects. The hooks themselves, in
// cmd/mangle, only read what OpenVPN passes them and call these.

//
// Server lifecycle
//

// WriteVPNServerConfig writes the OpenVPN server configuration from the
// settings, so that the server always starts with what is in the database.
func (a *App) WriteVPNServerConfig(ctx context.Context) error {
	conf, err := openvpn.ServerConfig{
		BindAddress:      host.InterfaceIP(a.Config.Get(config.VPNInterface)),
		BindPort:         a.Config.Int(config.VPNPort, 1194),
		CACertificate:    a.Config.Get(config.CACertificate),
		Certificate:      a.Config.Get(config.VPNCertificate),
		CRLFile:          a.Paths.CRL,
		Domain:           a.Config.Get(config.VPNDomain),
		Executable:       a.Paths.Executable,
		LogFile:          a.Paths.OpenVPNLog,
		LogLevel:         a.Config.Int(config.VPNLogLevel, 3),
		ManagementSocket: a.ManagementSocket(),
		Nameservers:      a.Config.Lines(config.VPNNameservers),
		PrivateKey:       a.Config.Get(config.VPNPrivateKey),
		Protocol:         a.Config.GetOr(config.VPNProtocol, "udp"),
		RedirectGateway:  a.Config.Bool(config.VPNRedirectGW, false),
		Routes:           a.Config.Lines(config.VPNRoutes),
		SessionLifetime:  a.Config.Int(config.VPNSessionHours, 0) * 3600,
		StatusFile:       a.Paths.OpenVPNStatusLog,
		Subnet:           a.Config.Get(config.VPNSubnet),
		TLSAuthKey:       a.Config.Get(config.VPNTLSAuthKey),
		PortShare:        a.portShare(),
		MSSFix:           a.Config.Int(config.VPNMSSFix, 0),
	}.Render()
	if err != nil {
		return err
	}

	// The configuration holds the server's private key.
	if err := host.WriteFile(a.Paths.OpenVPNConfig, conf, 0o600); err != nil {
		return fmt.Errorf("app: write the OpenVPN configuration: %w", err)
	}

	// OpenVPN refuses to start without the revocation list it is pointed
	// at, so an empty one stands in until the first is published.
	if err := touch(a.Paths.CRL); err != nil {
		return fmt.Errorf("app: create the revocation list: %w", err)
	}

	a.Log.Info("OpenVPN configuration written", "file", a.Paths.OpenVPNConfig)
	return nil
}

// portShare returns the web server's HTTPS port when OpenVPN is to share
// its port with it, or 0.
func (a *App) portShare() int {
	if !a.Config.Bool(config.VPNPortShare, false) {
		return 0
	}
	return a.Config.Int(config.AppHTTPSPort, 0)
}

// InstallVPNFirewall builds the firewall around a freshly started server:
// its base chains and rules, and a chain for each group.
func (a *App) InstallVPNFirewall(ctx context.Context) error {
	// Clear anything a previous run left behind before building again.
	if err := a.RemoveVPNFirewall(ctx); err != nil {
		return err
	}

	iptables.CreateChain(ctx, iptables.Filter, provision.VPNChain)
	iptables.CreateChain(ctx, iptables.Filter, provision.ClientsChain)

	rules, err := provision.VPNRules(a.Config)
	if err != nil {
		return err
	}
	provision.ApplyRules(ctx, rules)

	if err := a.RebuildAllGroupChains(ctx); err != nil {
		return err
	}

	// The rules are recorded so that exactly these are withdrawn at stop,
	// even if the settings they were built from change in between.
	if err := a.Config.Set(ctx, config.VPNFirewallRules, rules); err != nil {
		return err
	}

	a.Log.Info("OpenVPN firewall rules installed")
	return a.Config.SetBool(ctx, config.VPNRestartPending, false)
}

// RemoveVPNFirewall tears the server's firewall down again and forgets its
// connections, which do not survive the server.
func (a *App) RemoveVPNFirewall(ctx context.Context) error {
	if rules := a.Config.Get(config.VPNFirewallRules); rules != "" {
		provision.WithdrawRules(ctx, rules)
	}

	if err := a.DeleteAllGroupChains(ctx); err != nil {
		return err
	}

	iptables.DeleteChain(ctx, iptables.Filter, provision.VPNChain)
	iptables.DeleteChain(ctx, iptables.Filter, provision.ClientsChain)

	if err := a.Config.Delete(ctx, config.VPNFirewallRules); err != nil {
		return err
	}

	// A stale connection record would otherwise be shown as live and keep
	// a firewall rule alive.
	if err := a.Store.Clients.DeleteAll(ctx); err != nil {
		return err
	}

	a.Log.Info("OpenVPN firewall rules removed")
	return nil
}

// InstallWebFirewall opens the web application's ports.
func (a *App) InstallWebFirewall(ctx context.Context) error {
	// Withdraw whatever was installed last time first, so that a port change
	// does not leave the old one open.
	a.withdrawWebRules(ctx)

	rules, err := provision.WebRules(a.Config)
	if err != nil {
		return err
	}
	provision.ApplyRules(ctx, rules)

	a.Log.Info("web firewall rules installed")
	return a.Config.Set(ctx, config.WebFirewallRules, rules)
}

// RemoveWebFirewall closes the web application's ports again.
func (a *App) RemoveWebFirewall(ctx context.Context) error {
	a.withdrawWebRules(ctx)
	a.Log.Info("web firewall rules removed")
	return a.Config.Delete(ctx, config.WebFirewallRules)
}

// withdrawWebRules removes the web rules recorded at the last start.
func (a *App) withdrawWebRules(ctx context.Context) {
	if rules := a.Config.Get(config.WebFirewallRules); rules != "" {
		provision.WithdrawRules(ctx, rules)
	}
}

//
// Clients
//

// VPNLogin is what a connecting client presents to be authenticated: the
// username and password typed into the client, and the fingerprint of the
// certificate its TLS session was made with.
type VPNLogin struct {
	Username    string
	Password    string
	Fingerprint string
}

// AuthenticateVPNClient decides whether a client may connect. By the time it
// is asked, the client has presented a certificate the CA signed, so the
// checks are about whether that certificate's owner, now, may connect:
//
//   - the certificate must belong to a device of the user named, or a valid
//     profile could be used under anyone's name, including to get past
//     two-factor or a disabled account;
//   - the account must be active and not locked out;
//   - where two-factor is required, the password carries a code that has
//     not been used before (see VerifyMFA).
//
// Any error refuses the connection.
func (a *App) AuthenticateVPNClient(ctx context.Context, login VPNLogin) error {
	user, err := a.Store.Users.ByEmail(ctx, login.Username)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			a.Log.Error("failed to look up a connecting user", "username", login.Username, "err", err)
		}
		return fmt.Errorf("app: no user with the username %s", login.Username)
	}

	device, err := a.Store.Devices.ByFingerprint(ctx, login.Fingerprint)
	if err != nil || device.UserID != user.ID {
		a.RecordEvent(ctx, user, model.EventVPNError,
			"Connection refused: the certificate does not belong to this user.")
		return fmt.Errorf("app: the certificate %q does not belong to %s", login.Fingerprint, login.Username)
	}

	if !user.IsActive() {
		a.RecordEvent(ctx, user, model.EventVPNError, "User is not currently active.")
		return fmt.Errorf("app: the account for %s is not active", login.Username)
	}
	if user.IsLocked(time.Now()) {
		a.RecordEvent(ctx, user, model.EventVPNError,
			"Connection refused: the account is locked after too many failed attempts.")
		return fmt.Errorf("app: the account for %s is locked", login.Username)
	}

	// A profile issued while the account password is required asks for
	// both and sends them together; an older one sends the code alone.
	password, code, both := openvpn.ParseStaticChallenge(login.Password)
	if !both {
		code = login.Password
		if a.VPNRequiresPassword() {
			a.RecordEvent(ctx, user, model.EventVPNError,
				"Connection refused: the profile does not ask for the account password. Add the device again.")
			return fmt.Errorf("app: %s connected with a profile that does not send the password", login.Username)
		}
	}
	if both && !user.CheckPassword(password) {
		a.RecordFailedSignIn(ctx, user)
		a.RecordEvent(ctx, user, model.EventVPNError, "Incorrect password.")
		return fmt.Errorf("app: wrong password for %s", login.Username)
	}

	if user.MFARequired() {
		ok, err := a.VerifyMFA(ctx, user, code)
		if err != nil {
			return fmt.Errorf("app: check the two-factor code for %s: %w", login.Username, err)
		}
		if !ok {
			a.RecordEvent(ctx, user, model.EventVPNError, "User two-factor authentication code invalid.")
			return fmt.Errorf("app: invalid two-factor authentication code for %s", login.Username)
		}
	}
	if both || user.MFARequired() {
		a.SignInSucceeded(ctx, user)
	}

	a.Log.Info("authenticated an OpenVPN client", "username", login.Username, "device", device.Name)
	return nil
}

// VPNRequiresPassword reports whether connecting asks for the account
// password as well as the authenticator code.
func (a *App) VPNRequiresPassword() bool {
	return a.Config.Bool(config.VPNRequirePassword, false)
}

// VPNChallengePrompt is the prompt profiles carry for the code, when the
// account password is required too, or "" when it is not and the password
// field carries the code alone.
func (a *App) VPNChallengePrompt() string {
	if !a.VPNRequiresPassword() {
		return ""
	}
	return "Code from your authenticator app"
}

// VPNConnection describes a client OpenVPN has just admitted.
type VPNConnection struct {
	CommonName  string
	Fingerprint string
	Platform    string
	VirtualIP   string
	RemoteIP    string
	RemotePort  string

	// ConfigFile is where OpenVPN reads back the client's own settings:
	// its fixed address and its group's routes and DNS servers. Empty
	// writes nothing.
	ConfigFile string
}

// ConnectVPNClient records a client that has just connected and opens its
// group's firewall rules to it. Any error makes OpenVPN drop the client.
func (a *App) ConnectVPNClient(ctx context.Context, conn VPNConnection) error {
	// The certificate fingerprint, not the common name, is what ties the
	// connection to a device: it cannot be chosen by the client.
	device, err := a.Store.Devices.ByFingerprint(ctx, conn.Fingerprint)
	if err != nil {
		return fmt.Errorf("app: no device with the certificate fingerprint %s", conn.Fingerprint)
	}

	// Checked again here, since a certificate outlives its owner's access: an
	// account disabled since this device was issued must not connect.
	if device.User == nil || !device.User.IsActive() {
		a.RecordEvent(ctx, device.User, model.EventVPNError,
			"Connection refused: the device's owner is not currently active.")
		return fmt.Errorf("app: the owner of device %s is not active", device.Name)
	}

	// A fixed address replaces the one OpenVPN picked from its pool, and
	// is the one the firewall rule has to name.
	conf, virtualIP := a.clientConnectConfig(device, conn.VirtualIP)
	if conn.ConfigFile != "" {
		if err := writeClientConnectConfig(conn.ConfigFile, conf); err != nil {
			return err
		}
	}

	client := &model.Client{
		CommonName: conn.CommonName,
		DeviceID:   device.ID,
		Platform:   conn.Platform,
		RemoteIP:   conn.RemoteIP + ":" + conn.RemotePort,
		VirtualIP:  virtualIP,
	}
	// With one connection per person, a new one replaces whatever other
	// device they had connected.
	if a.Config.Bool(config.VPNSingleSession, false) {
		a.disconnectOtherDevices(ctx, device)
	}

	if err := a.ConnectClient(ctx, client); err != nil {
		return err
	}

	a.RecordEvent(ctx, device.User, model.EventVPNConnect,
		fmt.Sprintf("Device %s connected from %s", device.Name, conn.RemoteIP))

	if err := a.Store.Devices.TouchLastLogin(ctx, device); err != nil {
		return err
	}

	a.Log.Info("OpenVPN client connected",
		"device", device.Name, "user", device.User.Email, "remote", conn.RemoteIP)
	return nil
}

// VPNDisconnection describes a client that has gone away, with the data
// it moved as OpenVPN counted it from the server's side.
type VPNDisconnection struct {
	CommonName    string
	RemoteIP      string
	BytesReceived int64
	BytesSent     int64
}

// disconnectOtherDevices drops the connections of the device owner's other
// devices. A failure is logged rather than refusing the new connection.
func (a *App) disconnectOtherDevices(ctx context.Context, device *model.Device) {
	clients, err := a.Store.Clients.ByUser(ctx, device.UserID)
	if err != nil {
		a.Log.Error("failed to look up a user's other connections", "user", device.User.Email, "err", err)
		return
	}
	for _, other := range clients {
		if other.DeviceID == device.ID {
			continue
		}
		if err := a.DisconnectClient(ctx, other); err != nil {
			a.Log.Error("failed to disconnect a user's other device", "device", other.Device.Name, "err", err)
			continue
		}
		a.RecordEvent(ctx, device.User, model.EventVPNDisconnect,
			fmt.Sprintf("Device %s disconnected because %s connected.", other.Device.Name, device.Name))
	}
}

// DisconnectVPNClient forgets a client that has gone away, closes the
// firewall to it, and adds the connection to its device's history. A client
// with nothing recorded is not an error.
func (a *App) DisconnectVPNClient(ctx context.Context, gone VPNDisconnection) error {
	commonName, remoteIP := gone.CommonName, gone.RemoteIP
	client, err := a.Store.Clients.ByCommonName(ctx, commonName)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	a.RecordEvent(ctx, client.User(), model.EventVPNDisconnect,
		fmt.Sprintf("Device %s disconnected from %s after %s",
			client.Device.Name, remoteIP, formatDuration(client.Duration())))

	// The connection has already gone, so withdraw the firewall rule and
	// forget the client without asking OpenVPN to drop it again.
	a.RemoveClientRule(ctx, client)

	if err := a.Store.Clients.Delete(ctx, client.ID); err != nil {
		return err
	}

	// The history is worth having but not worth failing the hook over.
	if err := a.Store.VPNSessions.Record(ctx, &model.VPNSession{
		DeviceID:      client.DeviceID,
		UserID:        client.Device.UserID,
		StartedAt:     client.CreatedAt,
		EndedAt:       time.Now().UTC(),
		RemoteIP:      remoteIP,
		VirtualIP:     client.VirtualIP,
		BytesReceived: gone.BytesReceived,
		BytesSent:     gone.BytesSent,
	}); err != nil {
		a.Log.Error("failed to record a connection's history", "device", client.Device.Name, "err", err)
	}

	a.Log.Info("OpenVPN client disconnected", "device", client.Device.Name, "remote", remoteIP)
	return nil
}

// formatDuration renders a number of seconds as "1h 05m 30s", or as seconds
// alone under a minute.
func formatDuration(secs int) string {
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	hours, mins, rem := secs/3600, secs%3600/60, secs%60
	if hours > 0 {
		return fmt.Sprintf("%dh %02dm %02ds", hours, mins, rem)
	}
	return fmt.Sprintf("%dm %02ds", mins, rem)
}

// touch creates an empty file at path when it does not already exist.
func touch(path string) error {
	f, err := os.OpenFile(path, os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	return f.Close()
}
