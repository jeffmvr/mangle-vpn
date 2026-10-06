// Package provision writes the files the application needs outside its own
// database: the systemd units, the web server's TLS material, and the
// firewall rule sets for the web and VPN services.
package provision

import (
	"cmp"
	"context"
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
)

//go:embed templates
var templateFS embed.FS

// templates holds the systemd unit and firewall rule templates.
var templates = template.Must(template.ParseFS(templateFS,
	"templates/systemd/*.service", "templates/firewall/*.rules"))

// File permissions.
const (
	// keyPerm keeps private key material readable only by its owner.
	keyPerm = 0o600

	// unitPerm is applied to the generated systemd units.
	unitPerm = 0o644
)

// render executes a template by name.
func render(name string, data any) (string, error) {
	var b strings.Builder
	if err := templates.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("provision: render %s: %w", name, err)
	}
	return b.String(), nil
}

// WebKeys writes a self-signed TLS certificate and private key for the web
// server to use until an administrator supplies their own.
func WebKeys(p paths.Paths, keySize int) error {
	kp, err := pki.SelfSigned(webCertificateName, keySize, true)
	if err != nil {
		return err
	}

	certificate, privateKey := kp.PEM()
	return WriteTLSMaterial(p, certificate, privateKey)
}

// WebKeysIfMissing generates the self-signed web certificate when there is
// none yet, leaving one already in place, whether generated, uploaded or
// pasted, alone.
func WebKeysIfMissing(p paths.Paths, keySize int) error {
	_, certErr := os.Stat(p.WebCertificate)
	_, keyErr := os.Stat(p.WebPrivateKey)
	if certErr == nil && keyErr == nil {
		return nil
	}
	return WebKeys(p, keySize)
}

// webCertificateName is the common name of the generated self-signed web
// certificate.
const webCertificateName = "Mangle VPN"

// WriteTLSMaterial stores the web server's certificate and private key.
//
// The running server notices the change and serves the new certificate on
// the next connection, so there is nothing to restart.
func WriteTLSMaterial(p paths.Paths, certificate, privateKey string) error {
	if err := host.WriteFile(p.WebCertificate, certificate, keyPerm); err != nil {
		return fmt.Errorf("provision: write web certificate: %w", err)
	}
	if err := host.WriteFile(p.WebPrivateKey, privateKey, keyPerm); err != nil {
		return fmt.Errorf("provision: write web private key: %w", err)
	}
	return nil
}

// unitData fills in the systemd unit templates.
type unitData struct {
	RootDir       string
	Executable    string
	OpenVPN       string
	OpenVPNConfig string
	Modprobe      string
}

// SystemdUnits writes the systemd units and reloads the daemon so that
// systemd picks them up.
func SystemdUnits(ctx context.Context, p paths.Paths) error {
	data := unitData{
		RootDir:       p.Root,
		Executable:    p.Executable,
		OpenVPN:       host.Which("openvpn"),
		OpenVPNConfig: p.OpenVPNConfig,
		Modprobe:      cmp.Or(host.Which("modprobe"), "/sbin/modprobe"),
	}

	units := map[string]string{
		"mangle-web.service": p.WebUnit,
		"mangle-vpn.service": p.VPNUnit,
	}

	// The background tasks used to run as a service of their own, and now
	// run inside the web application. An installation that still has the
	// old unit has it stopped and removed, so the work is not done twice.
	retired := filepath.Join(p.Systemd, "mangle-tasks.service")
	if _, err := os.Stat(retired); err == nil {
		host.Run(ctx, "systemctl", "disable", "--now", "mangle-tasks.service")
		if err := os.Remove(retired); err != nil {
			return fmt.Errorf("provision: remove the old task worker unit: %w", err)
		}
	}

	for name, path := range units {
		content, err := render(name, data)
		if err != nil {
			return err
		}
		if err := host.WriteFile(path, content, unitPerm); err != nil {
			return fmt.Errorf("provision: write %s: %w", name, err)
		}
		host.Run(ctx, "systemctl", "enable", path)
	}

	host.Run(ctx, "systemctl", "daemon-reload")
	return nil
}
