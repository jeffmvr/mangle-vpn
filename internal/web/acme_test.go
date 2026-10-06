package web

import (
	"crypto/tls"
	"io"
	"log/slog"
	"os"
	"slices"
	"testing"

	"golang.org/x/crypto/acme"

	"github.com/jeffmvr/mangle-vpn/internal/pki"
)

func TestACMEFallsBackToTheCertificateOnDisk(t *testing.T) {
	dir := t.TempDir()
	ca, err := pki.NewAuthority(pki.Options{KeySize: 2048})
	if err != nil {
		t.Fatal(err)
	}
	kp, err := ca.IssueServer("vpn.example.com", pki.Validity)
	if err != nil {
		t.Fatal(err)
	}
	certificate, key := kp.PEM()
	writeFile(t, dir+"/web.crt", certificate)
	writeFile(t, dir+"/web.key", key)

	loader, err := NewCertificateLoader(dir+"/web.crt", dir+"/web.key", slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	manager := NewACMEManager(t.TempDir(), "vpn.example.com", "")
	config := loader.TLSConfigWithACME(manager, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if !slices.Contains(config.NextProtos, acme.ALPNProto) {
		t.Error("the TLS-ALPN challenge protocol is not offered")
	}

	// Another name is refused by the host policy without asking Let's
	// Encrypt, so the certificate on disk is served instead.
	got, err := config.GetCertificate(&tls.ClientHelloInfo{ServerName: "other.example.com"})
	if err != nil || got == nil {
		t.Fatalf("no fallback certificate: %v", err)
	}

	// A TLS-ALPN challenge is never answered with the fallback.
	if _, err := config.GetCertificate(&tls.ClientHelloInfo{
		ServerName: "other.example.com", SupportedProtos: []string{acme.ALPNProto},
	}); err == nil {
		t.Error("a TLS-ALPN challenge for another name was answered")
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
