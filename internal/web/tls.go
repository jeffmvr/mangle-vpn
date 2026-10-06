package web

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CertificateLoader serves the web server's TLS certificate, reloading it
// from disk when the files change.
//
// An administrator can paste a new certificate into the settings page, and
// the next connection picks it up without the service being restarted.
type CertificateLoader struct {
	certificateFile string
	privateKeyFile  string
	log             *slog.Logger

	mu          sync.RWMutex
	certificate *tls.Certificate
	modified    time.Time
}

// NewCertificateLoader returns a loader for the given key pair and reads it
// once so that a broken certificate is reported at start up rather than on
// the first connection.
func NewCertificateLoader(certificateFile, privateKeyFile string, log *slog.Logger) (*CertificateLoader, error) {
	loader := &CertificateLoader{
		certificateFile: certificateFile,
		privateKeyFile:  privateKeyFile,
		log:             log,
	}

	if err := loader.reload(); err != nil {
		return nil, err
	}
	return loader, nil
}

// TLSConfig returns a TLS configuration that serves the loaded certificate.
func (l *CertificateLoader) TLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion:     tls.VersionTLS12,
		GetCertificate: l.GetCertificate,
		NextProtos:     []string{"h2", "http/1.1"},
	}
}

// GetCertificate implements the hook in [tls.Config].
func (l *CertificateLoader) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if l.changedOnDisk() {
		if err := l.reload(); err != nil {
			// Keep serving the certificate already in hand rather than
			// failing the handshake over a half-written replacement.
			l.log.Error("keeping the previous TLS certificate", "err", err)
		} else {
			l.log.Info("reloaded the TLS certificate", "file", l.certificateFile)
		}
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.certificate, nil
}

// changedOnDisk reports whether the certificate file has been written since
// it was last read.
func (l *CertificateLoader) changedOnDisk() bool {
	info, err := os.Stat(l.certificateFile)
	if err != nil {
		return false
	}

	l.mu.RLock()
	defer l.mu.RUnlock()
	return info.ModTime().After(l.modified)
}

// reload reads the key pair from disk.
func (l *CertificateLoader) reload() error {
	certificate, err := tls.LoadX509KeyPair(l.certificateFile, l.privateKeyFile)
	if err != nil {
		return fmt.Errorf("web: load the TLS certificate: %w", err)
	}

	modified := time.Now()
	if info, err := os.Stat(l.certificateFile); err == nil {
		modified = info.ModTime()
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.certificate, l.modified = &certificate, modified
	return nil
}

// RedirectToHTTPS returns a handler that sends every request to the HTTPS
// port, which is all the plain HTTP listener does.
func RedirectToHTTPS(httpsPort int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if name, _, err := net.SplitHostPort(host); err == nil {
			host = name
		}
		if httpsPort != 443 {
			host = net.JoinHostPort(host, strconv.Itoa(httpsPort))
		}

		target := "https://" + host + r.URL.RequestURI()

		// A redirect cannot preserve a request body, so anything but a
		// read is refused rather than silently losing it.
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Location", target)
			http.Error(w, "Use HTTPS.", http.StatusPermanentRedirect)
			return
		}

		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

// strictTransportSecurity is the HSTS policy served over TLS.
const strictTransportSecurity = "max-age=31536000; includeSubDomains"

// contentSecurityPolicy limits what a page may load and run. Scripts come
// only from this server, never inline, so injected markup cannot run any.
// Styles may be inline, which the server rendered pages use; fonts come from
// Google Fonts, which both the pages and the interface load.
var contentSecurityPolicy = strings.Join([]string{
	"default-src 'self'",
	"script-src 'self'",
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com",
	"font-src https://fonts.gstatic.com",
	"img-src 'self' data:",
	"connect-src 'self'",
	"object-src 'none'",
	"base-uri 'none'",
	"form-action 'self'",
	"frame-ancestors 'none'",
}, "; ")

// secureHeaders adds the response headers that protect every page.
func (s *Server) secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", contentSecurityPolicy)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "same-origin")

		forwardedTLS := s.trustProxy && strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
		if r.TLS != nil || forwardedTLS {
			header.Set("Strict-Transport-Security", strictTransportSecurity)
		}

		next.ServeHTTP(w, r)
	})
}
