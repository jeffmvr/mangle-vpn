package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
)

// CertificateWarning is how long before a certificate expires that
// administrators are warned.
const CertificateWarning = 30 * 24 * time.Hour

// Certificate is one of the certificates the application depends on, and
// when it expires.
type Certificate struct {
	Name     string    `json:"name"`
	Label    string    `json:"label"`
	NotAfter time.Time `json:"not_after"`
}

// ExpiresSoon reports whether the certificate is within CertificateWarning
// of expiring, or already has.
func (c Certificate) ExpiresSoon(now time.Time) bool {
	return now.Add(CertificateWarning).After(c.NotAfter)
}

// Certificates returns the certificate authority's, the OpenVPN server's and
// the web server's certificates, skipping any not yet created. Device
// certificates are not included: the CA's expiry bounds theirs, and a device
// is replaced by adding it again.
func (a *App) Certificates() []Certificate {
	sources := []struct {
		name, label string
		pem         func() string
	}{
		{"ca", "Certificate authority", func() string { return a.Config.Get(config.CACertificate) }},
		{"openvpn", "OpenVPN server", func() string { return a.Config.Get(config.VPNCertificate) }},
		{"web", "Web server", func() string {
			path := a.Paths.WebCertificate
			if a.Config.Bool(config.AppLetsEncrypt, false) {
				// The cache file holds the key and then the chain; the
				// certificate is the first CERTIFICATE block, which
				// ParseCertificate finds below.
				path = filepath.Join(a.Paths.ACMECache, a.Config.Get(config.AppHostname))
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return ""
			}
			return string(data)
		}},
	}

	var out []Certificate
	for _, source := range sources {
		data := source.pem()
		if data == "" {
			continue
		}
		certificate, err := pki.ParseCertificate(data)
		if err != nil {
			a.Log.Error("failed to read a certificate", "certificate", source.name, "err", err)
			continue
		}
		out = append(out, Certificate{Name: source.name, Label: source.label, NotAfter: certificate.NotAfter.UTC()})
	}
	return out
}

// Healthy reports whether the application can serve: today, whether the
// database answers.
func (a *App) Healthy(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return a.Store.Ping(ctx)
}
