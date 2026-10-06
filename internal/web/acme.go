package web

import (
	"crypto/tls"
	"log/slog"
	"net/http"

	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
)

// Let's Encrypt. With it turned on, the web server obtains and renews its
// certificate itself, for the configured hostname only. The domain checks
// are answered on port 80, through the HTTP listener, and on port 443, by
// TLS-ALPN, so either port being reachable from the internet is enough.

// NewACMEManager returns the certificate manager for hostname. Certificates
// and the account key are cached in cacheDir, so a restart does not ask Let's
// Encrypt again. email, if set, is where Let's Encrypt sends expiry notices.
func NewACMEManager(cacheDir, hostname, email string) *autocert.Manager {
	return &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		Cache:      autocert.DirCache(cacheDir),
		HostPolicy: autocert.HostWhitelist(hostname),
		Email:      email,
	}
}

// TLSConfigWithACME returns a TLS configuration that serves the Let's
// Encrypt certificate, falling back to the loader's certificate when one
// cannot be had, for instance before the domain checks have passed or for a
// visitor using the server's IP address. A failure to obtain a certificate
// then shows as a browser warning rather than a site that is down.
func (l *CertificateLoader) TLSConfigWithACME(manager *autocert.Manager, log *slog.Logger) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		NextProtos: []string{"h2", "http/1.1", acme.ALPNProto},
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			certificate, err := manager.GetCertificate(hello)
			if err == nil {
				return certificate, nil
			}
			// The TLS-ALPN challenge must not be answered with anything else.
			for _, proto := range hello.SupportedProtos {
				if proto == acme.ALPNProto {
					return nil, err
				}
			}
			log.Warn("serving the fallback certificate", "server_name", hello.ServerName, "err", err)
			return l.GetCertificate(hello)
		},
	}
}

// ACMEChallenges wraps the HTTP listener's handler so that it answers Let's
// Encrypt's HTTP domain checks, and does what it did before for everything
// else.
func ACMEChallenges(manager *autocert.Manager, next http.Handler) http.Handler {
	return manager.HTTPHandler(next)
}
