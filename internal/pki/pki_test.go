package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

// firstLine returns the first non-empty line of s, for use in a message.
func firstLine(s string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return "no output"
}

// testKeySize keeps the tests quick. Key size does not affect the structure
// of what is issued, which is what these tests check.
const testKeySize = 1024

// newTestAuthority returns a certificate authority for the tests.
func newTestAuthority(t *testing.T) Authority {
	t.Helper()

	ca, err := NewAuthority(Options{KeySize: testKeySize})
	if err != nil {
		t.Fatalf("NewAuthority: %v", err)
	}
	return ca
}

func TestAuthorityIsSelfSignedAndCanSign(t *testing.T) {
	ca := newTestAuthority(t)
	cert := ca.Certificate

	if cert.Subject.CommonName != "OpenVPN CA" {
		t.Errorf("common name = %q, want OpenVPN CA", cert.Subject.CommonName)
	}
	if !cert.IsCA || !cert.BasicConstraintsValid {
		t.Error("the authority is not marked as a certificate authority")
	}
	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Error("the authority cannot sign certificates")
	}
	// Publishing a revocation list depends on this.
	if cert.KeyUsage&x509.KeyUsageCRLSign == 0 {
		t.Error("the authority cannot sign revocation lists")
	}
	if err := cert.CheckSignatureFrom(cert); err != nil {
		t.Errorf("the authority is not self-signed: %v", err)
	}
	if cert.SignatureAlgorithm != x509.SHA512WithRSA {
		t.Errorf("signature algorithm = %v, want SHA512WithRSA", cert.SignatureAlgorithm)
	}
}

func TestIssuedCertificatesChainToTheAuthority(t *testing.T) {
	ca := newTestAuthority(t)

	roots := x509.NewCertPool()
	roots.AddCert(ca.Certificate)

	tests := []struct {
		name  string
		issue func(string, time.Duration) (KeyPair, error)
		usage x509.ExtKeyUsage
	}{
		{"OpenVPN Server", ca.IssueServer, x509.ExtKeyUsageServerAuth},
		{"person@example.com:laptop", ca.IssueClient, x509.ExtKeyUsageClientAuth},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kp, err := tt.issue(tt.name, Validity)
			if err != nil {
				t.Fatalf("issue: %v", err)
			}

			if kp.Certificate.Subject.CommonName != tt.name {
				t.Errorf("common name = %q, want %q", kp.Certificate.Subject.CommonName, tt.name)
			}
			if kp.Certificate.IsCA {
				t.Error("an issued leaf must not be a certificate authority")
			}

			// OpenVPN's remote-cert-tls check depends on the right usage.
			_, err = kp.Certificate.Verify(x509.VerifyOptions{
				Roots:     roots,
				KeyUsages: []x509.ExtKeyUsage{tt.usage},
			})
			if err != nil {
				t.Errorf("the certificate does not chain to the authority: %v", err)
			}
		})
	}
}

func TestIssuedCertificateIsValidAcrossClockSkew(t *testing.T) {
	ca := newTestAuthority(t)

	kp, err := ca.IssueClient("person@example.com:laptop", Validity)
	if err != nil {
		t.Fatalf("IssueClient: %v", err)
	}

	// Backdating absorbs modest skew between this machine and a client.
	if !kp.Certificate.NotBefore.Before(time.Now().Add(-time.Hour)) {
		t.Errorf("NotBefore = %v, which leaves no room for clock skew", kp.Certificate.NotBefore)
	}
	if !kp.Certificate.NotAfter.After(time.Now().Add(Validity - 48*time.Hour)) {
		t.Errorf("NotAfter = %v, which is sooner than the requested validity", kp.Certificate.NotAfter)
	}
}

func TestKeyPairPEMRoundTrip(t *testing.T) {
	ca := newTestAuthority(t)

	kp, err := ca.IssueClient("person@example.com:laptop", Validity)
	if err != nil {
		t.Fatalf("IssueClient: %v", err)
	}

	certPEM, keyPEM := kp.PEM()
	if !strings.HasPrefix(certPEM, "-----BEGIN CERTIFICATE-----") {
		t.Errorf("certificate PEM starts %q", firstLine(certPEM))
	}
	// OpenVPN has always been handed the traditional OpenSSL layout.
	if !strings.HasPrefix(keyPEM, "-----BEGIN RSA PRIVATE KEY-----") {
		t.Errorf("private key PEM starts %q", firstLine(keyPEM))
	}

	back, err := LoadKeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("LoadKeyPair: %v", err)
	}
	if back.SerialNumber() != kp.SerialNumber() {
		t.Errorf("serial = %s, want %s", back.SerialNumber(), kp.SerialNumber())
	}
	if !back.PrivateKey.(interface{ Equal(crypto.PrivateKey) bool }).Equal(kp.PrivateKey) {
		t.Error("the private key did not survive the round trip")
	}

	// An authority reloaded from storage must still issue usable leaves.
	caCertPEM, caKeyPEM := ca.PEM()
	reloaded, err := LoadAuthority(caCertPEM, caKeyPEM, testKeySize)
	if err != nil {
		t.Fatalf("LoadAuthority: %v", err)
	}
	if _, err := reloaded.IssueClient("person@example.com:phone", Validity); err != nil {
		t.Errorf("a reloaded authority could not issue: %v", err)
	}
}

func TestFingerprintMatchesOpenVPNFormat(t *testing.T) {
	ca := newTestAuthority(t)

	kp, err := ca.IssueClient("person@example.com:laptop", Validity)
	if err != nil {
		t.Fatalf("IssueClient: %v", err)
	}

	// OpenVPN hands the connect hook a SHA-1 digest as colon separated
	// lowercase hex, and a device is looked up by exactly that string.
	fingerprint := kp.Fingerprint()
	parts := strings.Split(fingerprint, ":")

	if len(parts) != 20 {
		t.Fatalf("fingerprint %q has %d octets, want 20", fingerprint, len(parts))
	}
	for _, part := range parts {
		if len(part) != 2 || strings.ToLower(part) != part {
			t.Fatalf("fingerprint %q is not lowercase two-digit hex", fingerprint)
		}
	}
}

func TestCreateCRL(t *testing.T) {
	ca := newTestAuthority(t)

	first, err := ca.IssueClient("person@example.com:laptop", Validity)
	if err != nil {
		t.Fatalf("IssueClient: %v", err)
	}
	second, err := ca.IssueClient("person@example.com:phone", Validity)
	if err != nil {
		t.Fatalf("IssueClient: %v", err)
	}

	// A serial that cannot be read must not stop the list being published.
	crlPEM, err := ca.CreateCRL([]string{first.SerialNumber(), "not-a-number", second.SerialNumber()})
	if err != nil {
		t.Fatalf("CreateCRL: %v", err)
	}
	if !strings.HasPrefix(crlPEM, "-----BEGIN X509 CRL-----") {
		t.Fatalf("CRL PEM starts %q", firstLine(crlPEM))
	}

	block, _ := decodePEMBlock(crlPEM)
	crl, err := x509.ParseRevocationList(block)
	if err != nil {
		t.Fatalf("ParseRevocationList: %v", err)
	}
	if err := crl.CheckSignatureFrom(ca.Certificate); err != nil {
		t.Errorf("the CRL is not signed by the authority: %v", err)
	}
	if len(crl.RevokedCertificateEntries) != 2 {
		t.Fatalf("CRL names %d certificates, want 2", len(crl.RevokedCertificateEntries))
	}
	if !crl.NextUpdate.After(time.Now()) {
		t.Error("the CRL is already stale, which would stop OpenVPN accepting clients")
	}

	// An empty revocation list is still a valid one; OpenVPN needs the file
	// to exist even when nothing has been revoked.
	empty, err := ca.CreateCRL(nil)
	if err != nil {
		t.Fatalf("CreateCRL with no serials: %v", err)
	}
	if empty == "" {
		t.Error("an empty revocation list produced no output")
	}
}

func TestSelfSignedWebCertificate(t *testing.T) {
	kp, err := SelfSigned("vpn", testKeySize, true)
	if err != nil {
		t.Fatalf("SelfSigned: %v", err)
	}

	cert := kp.Certificate
	if err := cert.CheckSignatureFrom(cert); err != nil {
		t.Errorf("the web certificate is not self-signed: %v", err)
	}
	// It terminates TLS for the web UI, so it needs server authentication...
	if len(cert.ExtKeyUsage) != 1 || cert.ExtKeyUsage[0] != x509.ExtKeyUsageServerAuth {
		t.Errorf("extended key usage = %v, want server authentication", cert.ExtKeyUsage)
	}
	// ...and it vouches for itself, so it must also be an authority.
	if !cert.IsCA {
		t.Error("a self-signed certificate must be its own authority")
	}
}

// decodePEMBlock returns the DER bytes of the first PEM block in data along
// with its type.
func decodePEMBlock(data string) ([]byte, string) {
	block, _ := pem.Decode([]byte(data))
	if block == nil {
		return nil, ""
	}
	return block.Bytes, block.Type
}

func TestParseCertificateSkipsALeadingKey(t *testing.T) {
	ca, err := NewAuthority(Options{KeySize: 2048})
	if err != nil {
		t.Fatal(err)
	}
	certificate, key := ca.KeyPair.PEM()
	// Let's Encrypt's cache keeps the key first and the chain after it.
	parsed, err := ParseCertificate(key + certificate)
	if err != nil || parsed == nil {
		t.Fatalf("ParseCertificate = %v", err)
	}
}

func TestECDSAAuthority(t *testing.T) {
	ca, err := NewAuthority(Options{Name: "Acme VPN CA", KeyType: KeyECDSA})
	if err != nil {
		t.Fatalf("NewAuthority: %v", err)
	}
	if ca.Certificate.Subject.CommonName != "Acme VPN CA" {
		t.Errorf("name = %q", ca.Certificate.Subject.CommonName)
	}

	// Stored and loaded again, it still issues ECDSA certificates.
	certificatePEM, keyPEM := ca.PEM()
	reloaded, err := LoadAuthority(certificatePEM, keyPEM, 0)
	if err != nil {
		t.Fatalf("LoadAuthority: %v", err)
	}
	if reloaded.KeyType != KeyECDSA {
		t.Fatalf("reloaded key type = %q", reloaded.KeyType)
	}

	for _, issue := range []func(string, time.Duration) (KeyPair, error){reloaded.IssueServer, reloaded.IssueClient} {
		kp, err := issue("leaf", Validity)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := kp.PrivateKey.(*ecdsa.PrivateKey); !ok {
			t.Errorf("leaf key is %T", kp.PrivateKey)
		}
		if err := kp.Certificate.CheckSignatureFrom(ca.Certificate); err != nil {
			t.Errorf("leaf not signed by the authority: %v", err)
		}
		if _, err := LoadKeyPair(kp.PEM()); err != nil {
			t.Errorf("leaf does not round trip: %v", err)
		}
	}

	crl, err := reloaded.CreateCRL([]string{"12345"})
	if err != nil {
		t.Fatalf("CreateCRL: %v", err)
	}
	der, _ := decodePEMBlock(crl)
	list, err := x509.ParseRevocationList(der)
	if err != nil || list.CheckSignatureFrom(ca.Certificate) != nil {
		t.Errorf("revocation list does not verify: %v", err)
	}
}
