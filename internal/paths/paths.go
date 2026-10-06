// Package paths names every file and directory the application owns.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

// VPNManagementSocket is OpenVPN's management interface, which lives
// outside the installation directory.
const VPNManagementSocket = "/run/mangle-vpn.sock"

// Paths names the files and directories under an installation root.
type Paths struct {
	// Root is the installation directory.
	Root string

	// Executable is the absolute path of the running binary, which the
	// systemd units and the OpenVPN hooks are generated against.
	Executable string

	// Directories.
	Data    string
	Keys    string
	Logs    string
	Systemd string
	Backups string

	// Data files.
	Database      string
	OpenVPNConfig string

	// Key material.
	CRL            string
	SecretKey      string
	SetupToken     string
	ACMECache      string
	WebCertificate string
	WebPrivateKey  string

	// Logs.
	AppLog           string
	OpenVPNLog       string
	OpenVPNStatusLog string

	// Generated systemd units.
	WebUnit string
	VPNUnit string
}

// New returns the paths under the given installation root. An empty root
// falls back to the directory holding the running binary.
func New(root string) (Paths, error) {
	executable, err := os.Executable()
	if err != nil {
		return Paths{}, fmt.Errorf("paths: locate the running binary: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return Paths{}, fmt.Errorf("paths: resolve %s: %w", executable, err)
	}

	if root == "" {
		root = filepath.Dir(executable)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return Paths{}, fmt.Errorf("paths: resolve installation root: %w", err)
	}

	data := filepath.Join(root, "data")
	keys := filepath.Join(data, "keys")
	logs := filepath.Join(data, "logs")
	systemd := filepath.Join(data, "systemd")
	backups := filepath.Join(data, "backups")
	return Paths{
		Root:       root,
		Executable: executable,

		Data:          data,
		Keys:          keys,
		Logs:          logs,
		Systemd:       systemd,
		Backups:       backups,
		Database:      filepath.Join(data, "mangle.db"),
		OpenVPNConfig: filepath.Join(data, "openvpn.conf"),

		CRL:            filepath.Join(keys, "crl.pem"),
		SecretKey:      filepath.Join(keys, "secret.key"),
		SetupToken:     filepath.Join(keys, "setup.token"),
		ACMECache:      filepath.Join(keys, "acme"),
		WebCertificate: filepath.Join(keys, "ssl.crt"),
		WebPrivateKey:  filepath.Join(keys, "ssl.key"),

		AppLog:           filepath.Join(logs, "mangle.log"),
		OpenVPNLog:       filepath.Join(logs, "openvpn.log"),
		OpenVPNStatusLog: filepath.Join(logs, "openvpn-status.log"),

		WebUnit: filepath.Join(systemd, "mangle-web.service"),
		VPNUnit: filepath.Join(systemd, "mangle-vpn.service"),
	}, nil
}

// EnsureDirs creates the application directories if they are missing.
//
// The data directory holds the database and private keys, so it is readable
// only by the user the services run as.
func (p Paths) EnsureDirs() error {
	for _, dir := range []string{p.Data, p.Keys, p.Logs, p.Systemd, p.Backups} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("paths: create %s: %w", dir, err)
		}
	}
	return nil
}
