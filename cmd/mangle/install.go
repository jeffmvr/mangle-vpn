package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/provision"
)

// runInstall performs the first run provisioning: it generates the key
// material and the systemd units, and leaves the application ready for an
// administrator to finish setup in a browser.
func runInstall(ctx context.Context, args []string) error {
	a, err := openApp(ctx, "install", "", args)
	if err != nil {
		return err
	}
	defer a.Close()

	banner()
	header("Running the Mangle VPN installer...")

	installed := a.Config.Bool(config.AppInstalled, false)

	// Key generation dominates the runtime, so each step is reported as it
	// finishes rather than leaving the operator watching a blank screen.
	type step struct {
		describe string
		run      func() error
	}
	var steps []step

	// Before setup, the certificate authority waits for the setup page,
	// which asks what to call it and which kind of key to use. An existing
	// installation keeps the one it has.
	if installed {
		steps = append(steps,
			step{"certificate authority keys created", func() error { return createAuthority(ctx, a) }},
			step{"revocation list created", func() error { return a.WriteCRL(ctx) }},
			step{"openvpn keys created", func() error { return a.CreateVPNKeys(ctx) }},
		)
	}
	steps = append(steps,
		step{"web tls keys created", func() error { return provision.WebKeysIfMissing(a.Paths, a.KeySize()) }},
		step{"systemd units created", func() error { return provision.SystemdUnits(ctx, a.Paths) }},
	)

	for _, step := range steps {
		if err := step.run(); err != nil {
			fail(step.describe)
			return err
		}
		ok(step.describe)
	}

	fmt.Println()
	ok("Installation finished!")

	if installed {
		fmt.Printf("\nSetup was already completed. Sign in at %s.\n",
			color.CyanString("%s", setupURL(a)))
		return nil
	}

	token, err := writeSetupToken(a)
	if err != nil {
		return err
	}
	fmt.Printf("\nPlease visit the web UI at %s to complete the setup process.\n",
		color.CyanString("%s/install?token=%s", setupURL(a), token))
	fmt.Printf("If you open it another way, the setup code is %s\n", color.CyanString("%s", token))
	fmt.Printf("%s\n\n", color.YellowString(
		"The certificate is self-signed, so your browser will warn you once."))

	return nil
}

// writeSetupToken issues the one-time code that the setup page asks for, so
// that only someone who can run this command, and so read the server's own
// files, can create the first administrator. Running install again issues a
// new code and retires the old one.
func writeSetupToken(a *app.App) (string, error) {
	token := rand.Text()
	if err := host.WriteFile(a.Paths.SetupToken, token+"\n", 0o600); err != nil {
		return "", fmt.Errorf("write the setup code: %w", err)
	}
	return token, nil
}

// createAuthority generates the certificate authority, unless one is
// already present: regenerating it would invalidate every device that has
// been issued a certificate.
func createAuthority(ctx context.Context, a *app.App) error {
	if a.Config.Get(config.CACertificate) != "" {
		return nil
	}
	return a.CreateAuthority(ctx)
}

// setupURL returns the address the web UI can be reached on.
func setupURL(a *app.App) string {
	address := "this-machine"
	if addrs := host.IPAddresses(); len(addrs) > 0 {
		address = addrs[0]
	}

	if port := a.Config.Int(config.AppHTTPSPort, 443); port != 443 {
		return fmt.Sprintf("https://%s:%d", address, port)
	}
	return "https://" + address
}

//
// Terminal output
//

// banner prints the application name.
func banner() {
	fmt.Println(color.MagentaString(`
                             _
 _ __ ___   __ _ _ __   __ _| | ___  __   ___ __  _ __
| '_ ` + "`" + ` _ \ / _` + "`" + ` | '_ \ / _` + "`" + ` | |/ _ \ \ \ / / '_ \| '_ \
| | | | | | (_| | | | | (_| | |  __/  \ V /| |_) | | | |
|_| |_| |_|\__,_|_| |_|\__, |_|\___|   \_/ | .__/|_| |_|
                       |___/               |_|
`))
}

// header prints a section title.
func header(title string) {
	fmt.Println(color.New(color.FgCyan, color.Bold).Sprint(title))
}

// ok reports a step that succeeded.
func ok(message string) {
	fmt.Printf("  %s %s\n", color.GreenString("✓"), message)
}

// fail reports a step that did not.
func fail(message string) {
	fmt.Fprintf(os.Stderr, "  %s %s\n", color.RedString("✗"), message)
}
