// Package app wires the application together and holds the operations that
// span more than one subsystem.
//
// Saving a user, a group, or a firewall rule is never only a database write:
// it also reshapes the iptables chains that enforce the change and may
// disconnect clients. Those effects were implicit in the Django release,
// carried by model signals. Here they are plain method calls, so the order
// and the failure handling are visible at the call site.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strconv"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// secretKeyBytes is the size of the application secret, from which session
// and CSRF keys are derived.
const secretKeyBytes = 50

// App holds the application's shared dependencies.
type App struct {
	Paths  paths.Paths
	Store  *store.Store
	Config *config.Config
	Jobs   *jobs.Queue
	Log    *slog.Logger

	// SecretKey seeds the session and CSRF token keys.
	SecretKey []byte
}

// Open prepares the application rooted at the given directory, creating the
// data directories, database, and secret key when they are missing.
func Open(ctx context.Context, root string, log *slog.Logger) (*App, error) {
	p, err := paths.New(root)
	if err != nil {
		return nil, err
	}
	if err := p.EnsureDirs(); err != nil {
		return nil, err
	}

	secret, err := loadOrCreateSecretKey(p.SecretKey)
	if err != nil {
		return nil, err
	}

	st, err := store.Open(p.Database)
	if err != nil {
		return nil, err
	}

	// Secrets are encrypted at rest with a key derived from the secret key,
	// which lives outside the database. Anything still in plaintext, from
	// the Django release or from before encryption, is encrypted now.
	if err := st.UseSecretKey(secret, config.SecretSettings); err != nil {
		st.Close()
		return nil, err
	}
	if err := st.SealExisting(ctx); err != nil {
		st.Close()
		return nil, fmt.Errorf("app: encrypt stored secrets: %w", err)
	}

	cfg := config.New(st.Settings)
	if err := cfg.Reload(ctx); err != nil {
		st.Close()
		return nil, fmt.Errorf("app: load settings: %w", err)
	}

	a := &App{
		Paths:     p,
		Store:     st,
		Config:    cfg,
		Jobs:      jobs.NewQueue(st.Jobs),
		Log:       log,
		SecretKey: secret,
	}

	if err := a.applyDefaultSettings(ctx); err != nil {
		st.Close()
		return nil, err
	}
	return a, nil
}

// Close releases the application's resources.
func (a *App) Close() error { return a.Store.Close() }

// applyDefaultSettings fills in any setting that has never been given a
// value, so that a fresh installation starts out usable.
func (a *App) applyDefaultSettings(ctx context.Context) error {
	// The first interface with an address is the best guess for which one
	// the VPN should bind to and masquerade behind.
	iface := ""
	if names := host.InterfaceNames("lo", "tun"); len(names) > 0 {
		iface = names[0]
	}

	defaults := map[string]string{
		config.AppInstalled:           "False",
		config.AppOrganization:        "Mangle",
		config.AppEventRetentionDays:  "365",
		config.AppLetsEncrypt:         "False",
		config.AppHTTPPort:            "80",
		config.AppHTTPSPort:           "443",
		config.OAuth2Provider:         config.NoOAuth2Provider,
		config.PKIKeySize:             "2048",
		config.SMTPReplyAddress:       "",
		config.VPNInterface:           iface,
		config.VPNNATInterface:        iface,
		config.VPNPort:                "1194",
		config.VPNProtocol:            "udp",
		config.VPNRedirectGW:          "False",
		config.VPNDeviceToDevice:      "False",
		config.VPNRequirePassword:     "False",
		config.VPNIdleMinutes:         "60",
		config.VPNLogLevel:            "3",
		config.VPNSessionHours:        "0",
		config.VPNSubnet:              "172.25.0.0/16",
		config.AlertVPNDown:           "True",
		config.AlertCertificates:      "True",
		config.AlertLockouts:          "True",
		config.AlertNewDevice:         "False",
		config.AlertAdminSignIn:       "False",
		config.AlertNewAddress:        "False",
		config.AlertDailyDigest:       "False",
		config.AuthLockoutAttempts:    "10",
		config.AuthLockoutMinutes:     "15",
		config.AuthSessionIdleMinutes: "15",
		config.AuthSessionHours:       "12",
		config.AuthPasswordMinLength:  "8",
		config.AuthPasswordComplexity: "True",
		config.OAuth2Only:             "False",
		config.PKIDeviceDays:          "0",
		config.VPNPortShare:           "False",
		config.VPNMSSFix:              "0",
		config.VPNSingleSession:       "False",
		config.BackupKeep:             "7",
	}

	for name, value := range defaults {
		if err := a.Config.SetDefault(ctx, name, value); err != nil {
			return fmt.Errorf("app: set default %s: %w", name, err)
		}
	}
	return nil
}

// KeySize returns the configured RSA key size for issued certificates.
func (a *App) KeySize() int {
	return a.Config.Int(config.PKIKeySize, pki.DefaultKeySize)
}

// Authority returns the application's certificate authority.
func (a *App) Authority() (pki.Authority, error) {
	return pki.LoadAuthority(
		a.Config.Get(config.CACertificate),
		a.Config.Get(config.CAPrivateKey),
		a.KeySize(),
	)
}

// CreateAuthority generates the certificate authority and stores it.
func (a *App) CreateAuthority(ctx context.Context) error {
	return a.CreateAuthorityWith(ctx, pki.Options{KeySize: a.KeySize()})
}

// CreateAuthorityWith generates a certificate authority of the given name
// and key type and stores it. An RSA key size given is kept as the size of
// every certificate it goes on to issue.
func (a *App) CreateAuthorityWith(ctx context.Context, opts pki.Options) error {
	if opts.KeySize <= 0 {
		opts.KeySize = a.KeySize()
	} else if err := a.Config.Set(ctx, config.PKIKeySize, strconv.Itoa(opts.KeySize)); err != nil {
		return err
	}

	ca, err := pki.NewAuthority(opts)
	if err != nil {
		return err
	}
	certificate, privateKey := ca.PEM()
	if err := a.Config.Set(ctx, config.CACertificate, certificate); err != nil {
		return err
	}
	return a.Config.Set(ctx, config.CAPrivateKey, privateKey)
}

// WriteCRL regenerates the certificate revocation list and writes it to disk
// for OpenVPN to read.
func (a *App) WriteCRL(ctx context.Context) error {
	ca, err := a.Authority()
	if err != nil {
		return err
	}

	serials, err := a.Store.RevokedDevices.Serials(ctx)
	if err != nil {
		return err
	}

	crl, err := ca.CreateCRL(serials)
	if err != nil {
		return err
	}
	if err := host.WriteFile(a.Paths.CRL, crl, 0o644); err != nil {
		return fmt.Errorf("app: write revocation list: %w", err)
	}

	a.Log.Info("revocation list written", "revoked", len(serials))
	return nil
}

// loadOrCreateSecretKey reads the application secret, generating one on
// first run.
func loadOrCreateSecretKey(path string) ([]byte, error) {
	secret, err := os.ReadFile(path)
	switch {
	case err == nil && len(secret) > 0:
		return secret, nil
	case err != nil && !errors.Is(err, fs.ErrNotExist):
		return nil, fmt.Errorf("app: read secret key: %w", err)
	}

	secret = make([]byte, secretKeyBytes)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("app: generate secret key: %w", err)
	}

	// The secret signs sessions, so it must not be world readable.
	if err := host.WriteFile(path, string(secret), 0o600); err != nil {
		return nil, fmt.Errorf("app: write secret key: %w", err)
	}
	return secret, nil
}
