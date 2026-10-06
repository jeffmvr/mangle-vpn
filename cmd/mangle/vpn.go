package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/jeffmvr/mangle-vpn/internal/app"
)

// vpnActions are the OpenVPN service hooks. systemd runs the first three
// around the server's lifetime; OpenVPN runs the client ones as clients come
// and go, and refuses a client whose hook exits non-zero. What each does is
// decided in internal/app; these only read what OpenVPN passes in its
// environment.
//
// OpenVPN passes the client-connect hook one argument: the file to write the
// client's own settings to. The others take none.
var vpnActions = map[string]func(*app.App, context.Context, []string) error{
	"pre-start":  noArgs((*app.App).WriteVPNServerConfig),
	"post-start": noArgs((*app.App).InstallVPNFirewall),
	"post-stop":  noArgs((*app.App).RemoveVPNFirewall),

	"client-authenticate": func(a *app.App, ctx context.Context, _ []string) error {
		return a.AuthenticateVPNClient(ctx, app.VPNLogin{
			Username:    os.Getenv("username"),
			Password:    os.Getenv("password"),
			Fingerprint: os.Getenv("tls_digest_0"),
		})
	},

	"client-connect": func(a *app.App, ctx context.Context, args []string) error {
		configFile := ""
		if len(args) > 0 {
			configFile = args[len(args)-1]
		}
		return a.ConnectVPNClient(ctx, app.VPNConnection{
			ConfigFile:  configFile,
			CommonName:  os.Getenv("common_name"),
			Fingerprint: os.Getenv("tls_digest_0"),
			Platform:    os.Getenv("IV_PLAT"),
			VirtualIP:   os.Getenv("ifconfig_pool_remote_ip"),
			RemoteIP:    os.Getenv("trusted_ip"),
			RemotePort:  os.Getenv("trusted_port"),
		})
	},

	"client-disconnect": func(a *app.App, ctx context.Context, _ []string) error {
		return a.DisconnectVPNClient(ctx, app.VPNDisconnection{
			CommonName:    os.Getenv("common_name"),
			RemoteIP:      os.Getenv("trusted_ip"),
			BytesReceived: envInt("bytes_received"),
			BytesSent:     envInt("bytes_sent"),
		})
	},
}

// runVPN dispatches an OpenVPN service hook.
func runVPN(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errUsage
	}

	action, ok := vpnActions[args[0]]
	if !ok {
		return fmt.Errorf("unknown vpn action %q", args[0])
	}

	a, rest, err := openAppArgs(ctx, "vpn", "mangle.log", args[1:])
	if err != nil {
		return err
	}
	defer a.Close()

	return action(a, ctx, rest)
}

// envInt reads a whole number OpenVPN set in the environment, 0 when it is
// missing or not a number.
func envInt(name string) int64 {
	n, _ := strconv.ParseInt(os.Getenv(name), 10, 64)
	return n
}

// noArgs adapts an action that takes no arguments.
func noArgs(action func(*app.App, context.Context) error) func(*app.App, context.Context, []string) error {
	return func(a *app.App, ctx context.Context, _ []string) error { return action(a, ctx) }
}
