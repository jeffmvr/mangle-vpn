package main

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/provision"
)

// runContainer runs all of Mangle VPN in one process, for a container,
// where there is no systemd: it prepares the installation on first start,
// serves the web application and its background work, and runs OpenVPN as
// a child process. It takes the same flags as "web".
func runContainer(ctx context.Context, args []string) error {
	opts, err := parseWebFlags("run", args)
	if err != nil {
		return err
	}

	a, err := opts.common.open(ctx)
	if err != nil {
		return err
	}
	defer a.Close()

	if err := prepareContainer(ctx, a); err != nil {
		return err
	}
	warnWithoutForwarding(a)

	// OpenVPN runs as a child of this process, and whatever starts,
	// stops or restarts it, from the sidebar to the end of setup, does so
	// through it.
	vpn := a.NewVPNProcess()
	openvpn.Use(vpn)
	defer vpn.Stop(context.WithoutCancel(ctx))

	stopTasks := startTasks(ctx, a)
	defer stopTasks()

	// Started as the systemd unit would be at boot, unless an
	// administrator stopped it.
	if a.Config.Bool(config.AppInstalled, false) && !a.Config.Bool(config.VPNStoppedByAdmin, false) {
		vpn.Start(ctx)
	}

	return serveRestartable(ctx, a, opts)
}

// serveRestartable serves the web application, bringing it back with fresh
// settings whenever a change to its ports or certificate source asks it
// to, as systemd would restart the service. OpenVPN, a child of this
// process, is not interrupted.
func serveRestartable(ctx context.Context, a *app.App, opts webOptions) error {
	restart := make(chan struct{}, 1)
	opts.restart = func() {
		select {
		case restart <- struct{}{}:
		default:
		}
	}

	for {
		if err := a.Config.Reload(ctx); err != nil {
			a.Log.Error("failed to reload settings", "err", err)
		}
		if err := a.InstallWebFirewall(ctx); err != nil {
			a.Log.Error("failed to install the web firewall rules", "err", err)
		}

		webCtx, stop := context.WithCancel(ctx)
		go func() {
			select {
			case <-restart:
				a.Log.Info("restarting the web server")
				stop()
			case <-webCtx.Done():
			}
		}()

		err := serveWeb(webCtx, a, opts)
		stop()

		if err := a.RemoveWebFirewall(context.WithoutCancel(ctx)); err != nil {
			a.Log.Error("failed to remove the web firewall rules", "err", err)
		}
		if err != nil || ctx.Err() != nil {
			return err
		}
	}
}

// prepareContainer does what "mangle-vpn install" does on a server, on
// every start, leaving anything already there alone: the web certificate,
// and either the setup code for a new installation or the keys of one that
// has been set up.
func prepareContainer(ctx context.Context, a *app.App) error {
	if err := provision.WebKeysIfMissing(a.Paths, a.KeySize()); err != nil {
		return err
	}

	if a.Config.Bool(config.AppInstalled, false) {
		if err := createAuthority(ctx, a); err != nil {
			return err
		}
		if err := a.WriteCRL(ctx); err != nil {
			return err
		}
		return a.CreateVPNKeys(ctx)
	}

	token, err := existingSetupToken(a)
	if err != nil {
		return err
	}
	a.Log.Info("waiting for setup: open /install?token="+token+" on this server's address",
		"setup_code", token)
	return nil
}

// existingSetupToken returns the setup code already issued, so a restart
// before setup is finished does not retire the link printed before, and
// issues one when there is none.
func existingSetupToken(a *app.App) (string, error) {
	data, err := os.ReadFile(a.Paths.SetupToken)
	if err == nil && strings.TrimSpace(string(data)) != "" {
		return strings.TrimSpace(string(data)), nil
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	return writeSetupToken(a)
}

// ipForward is where the kernel says whether this network namespace
// forwards IPv4.
const ipForward = "/proc/sys/net/ipv4/ip_forward"

// warnWithoutForwarding warns when IPv4 forwarding is off, without which
// devices connect but reach nothing. A container cannot turn it on for
// itself; it is set when the container is created.
func warnWithoutForwarding(a *app.App) {
	data, err := os.ReadFile(ipForward)
	if err == nil && strings.TrimSpace(string(data)) != "1" {
		a.Log.Warn("IPv4 forwarding is off, so devices will reach nothing through the VPN; " +
			"start the container with --sysctl net.ipv4.ip_forward=1")
	}
}
