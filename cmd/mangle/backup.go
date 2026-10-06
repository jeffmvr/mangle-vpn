package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
)

// serviceUnits are the systemd units that must be stopped for a restore.
var serviceUnits = []string{"mangle-web", "mangle-vpn"}

// runBackup writes a backup of the installation: the database, the secret
// key and the web certificate. It is safe while the services run.
func runBackup(ctx context.Context, args []string) error {
	var flags commonFlags
	var output string

	fs := flag.NewFlagSet("backup", flag.ContinueOnError)
	flags.bind(fs, "")
	fs.StringVar(&output, "o", "", "file to write (defaults to data/backups/mangle-<time>.tar.gz)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	a, err := flags.open(ctx)
	if err != nil {
		return err
	}
	defer a.Close()

	if output == "" {
		output = filepath.Join(a.Paths.Backups, "mangle-"+time.Now().UTC().Format("20060102-150405")+".tar.gz")
	}
	if err := a.WriteBackupFile(ctx, output); err != nil {
		return err
	}

	fmt.Printf("Backup written to %s\n", output)
	fmt.Println("It holds the secret key that decrypts the database's secrets: keep it as safe as the server.")
	return nil
}

// runRestore replaces the installation's data with a backup. Whatever it
// replaces is moved aside rather than deleted.
func runRestore(ctx context.Context, args []string) error {
	var flags commonFlags

	fs := flag.NewFlagSet("restore", flag.ContinueOnError)
	flags.bind(fs, "")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("usage: mangle-vpn restore [-root dir] <backup.tar.gz>")
	}

	// The running services hold the database open and would carry on with
	// what is in memory, so they have to be stopped first.
	for _, unit := range serviceUnits {
		if host.Run(ctx, "systemctl", "is-active", "--quiet", unit) {
			return fmt.Errorf("%s is running; stop the services first: systemctl stop %s",
				unit, strings.Join(serviceUnits, " "))
		}
	}

	p, err := paths.New(flags.root)
	if err != nil {
		return err
	}

	f, err := os.Open(fs.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	aside, err := app.Restore(ctx, p, f)
	if err != nil {
		return err
	}

	fmt.Printf("Restored from %s.\n", fs.Arg(0))
	fmt.Printf("What it replaced is in %s.\n", aside)
	fmt.Printf("Start Mangle VPN again: systemctl start %s, or start its container.\n", strings.Join(serviceUnits, " "))
	return nil
}
