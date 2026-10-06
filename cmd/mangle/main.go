// Command mangle is the Mangle VPN server: an OpenVPN management
// application with a web interface.
//
// It runs as three systemd services, all from this one binary:
//
//	mangle web      the web application and its JSON API
//	mangle tasks    the background worker
//	mangle vpn      the hooks OpenVPN calls as clients come and go
//
// Plus commands run by hand:
//
//	mangle install  first run provisioning
//	mangle backup   write a backup of the data
//	mangle restore  replace the data with a backup
//	mangle version  the version number
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/jeffmvr/mangle-vpn/internal/version"
)

// errUsage asks main to print the usage message and exit non-zero.
var errUsage = errors.New("usage")

func main() {
	// A command stops at the first interrupt, which is also how systemd
	// asks a service to shut down.
	ctx, stop := signal.NotifyContext(context.Background(),
		os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		if errors.Is(err, errUsage) {
			usage()
			os.Exit(2)
		}
		// A cancelled context means the operator asked us to stop, which
		// is a clean exit rather than a failure, except for the OpenVPN
		// hooks: there OpenVPN reads a zero exit as "admit the client", so
		// an interrupted check must fail closed.
		if errors.Is(err, context.Canceled) && os.Args[1] != "vpn" {
			return
		}
		fmt.Fprintln(os.Stderr, filepath.Base(os.Args[0])+": "+err.Error())
		os.Exit(1)
	}
}

// run dispatches to the named command.
func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errUsage
	}

	command, rest := args[0], args[1:]

	switch command {
	case "web":
		return runWeb(ctx, rest)
	case "vpn":
		return runVPN(ctx, rest)
	case "run":
		return runContainer(ctx, rest)
	case "health":
		return runHealth(ctx, rest)
	case "install":
		return runInstall(ctx, rest)
	case "secrets":
		return runSecrets(ctx, rest)
	case "backup":
		return runBackup(ctx, rest)
	case "restore":
		return runRestore(ctx, rest)
	case "version":
		fmt.Printf("Mangle VPN v%s\n", version.String())
		return nil
	case "help", "-h", "--help":
		usage()
		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

// usage prints the command summary.
func usage() {
	name := filepath.Base(os.Args[0])

	fmt.Fprint(os.Stderr, `Mangle VPN `+version.String()+`

Usage:
  `+name+` <command> [flags]

Commands:
  web                      run the web application and its background tasks
  web post-start           install the web firewall rules
  web post-stop            remove the web firewall rules
  vpn pre-start            write the OpenVPN server configuration
  vpn post-start           install the OpenVPN firewall rules
  vpn post-stop            remove the OpenVPN firewall rules
  vpn client-authenticate  authenticate a connecting OpenVPN client
  vpn client-connect       record a connected OpenVPN client
  vpn client-disconnect    forget a disconnected OpenVPN client
  run                      run everything in one process, for a container
  health                   check that the web application answers
  install                  perform the first run setup
  backup [-o file]         write a backup of the database, keys and certificate
  restore <file>           replace the data with a backup, with the services stopped
  secrets decrypt          store secrets as plaintext, before rolling back to Django
  version                  print the version number

Run "`+name+` <command> -h" for the flags a command takes.
`)
}
