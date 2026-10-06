// Package pki issues and encodes the X.509 material the application needs:
// its own certificate authority, the OpenVPN server certificate, a
// certificate per client device, and the revocation list that retires them.
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// DefaultKeySize is the RSA key size used when the application settings do
// not say otherwise.
const DefaultKeySize = 2048

// Key types an authority can issue. Every certificate it signs uses the
// same type as its own key.
const (
	KeyRSA   = "rsa"
	KeyECDSA = "ecdsa"
)

// DefaultAuthorityName is the authority's name when none is given.
const DefaultAuthorityName = "OpenVPN CA"

// Validity is how long issued certificates last.
const Validity = 3650 * 24 * time.Hour

// backdate is how far before the present a certificate becomes valid. It
// absorbs modest clock skew between this machine and a connecting client.
const backdate = 24 * time.Hour

// role describes what an issued certificate is for, which decides its key
// usage and basic constraints.
type role int

const (
	roleClient role = iota
	roleServer
	roleCA
)

// KeyPair is a certificate together with its private key.
type KeyPair struct {
	Certificate *x509.Certificate
	PrivateKey  crypto.Signer
}

// Fingerprint returns the SHA-1 digest of the certificate as colon separated
// lowercase hex. It is the form OpenVPN hands to the connect hook, and how a
// device is recognised when it dials in.
func (kp KeyPair) Fingerprint() string {
	sum := sha1.Sum(kp.Certificate.Raw)

	var b strings.Builder
	for i, octet := range sum {
		if i > 0 {
			b.WriteByte(':')
		}
		b.WriteString(hex.EncodeToString([]byte{octet}))
	}
	return b.String()
}

// SerialNumber returns the certificate serial in decimal, the form the
// revocation list is built from.
func (kp KeyPair) SerialNumber() string {
	return kp.Certificate.SerialNumber.String()
}

// CertificatePEM returns the PEM encoded certificate.
func (kp KeyPair) CertificatePEM() string {
	return EncodeCertificate(kp.Certificate)
}

// PrivateKeyPEM returns the PEM encoded private key.
func (kp KeyPair) PrivateKeyPEM() string {
	return EncodePrivateKey(kp.PrivateKey)
}

// PEM returns the PEM encoded certificate and private key together.
func (kp KeyPair) PEM() (certificate, privateKey string) {
	return kp.CertificatePEM(), kp.PrivateKeyPEM()
}

// Authority is the application's certificate authority. It signs the server
// certificate, every client device certificate, and the revocation list.
type Authority struct {
	KeyPair

	// KeyType is KeyRSA or KeyECDSA, and KeySize the RSA key size of the
	// certificates it issues. ECDSA keys are always P-256.
	KeyType string
	KeySize int
}

// Options describe a new certificate authority.
type Options struct {
	// Name is its common name, DefaultAuthorityName when empty.
	Name string

	// KeyType is KeyRSA or KeyECDSA, KeyRSA when empty, and KeySize the
	// RSA key size.
	KeyType string
	KeySize int
}

// NewAuthority creates a self-signed certificate authority.
func NewAuthority(opts Options) (Authority, error) {
	keyType := opts.KeyType
	if keyType == "" {
		keyType = KeyRSA
	}
	if keyType != KeyRSA && keyType != KeyECDSA {
		return Authority{}, fmt.Errorf("pki: unknown key type %q", keyType)
	}
	name := opts.Name
	if name == "" {
		name = DefaultAuthorityName
	}

	ca := Authority{KeyType: keyType, KeySize: orDefault(opts.KeySize)}
	kp, err := ca.issue(name, Validity, roleCA)
	if err != nil {
		return Authority{}, fmt.Errorf("pki: create certificate authority: %w", err)
	}

	ca.KeyPair = kp
	return ca, nil
}

// LoadAuthority rebuilds the certificate authority from its stored PEM.
func LoadAuthority(certificatePEM, privateKeyPEM string, keySize int) (Authority, error) {
	kp, err := LoadKeyPair(certificatePEM, privateKeyPEM)
	if err != nil {
		return Authority{}, fmt.Errorf("pki: load certificate authority: %w", err)
	}
	// What the authority issues follows its own key.
	keyType := KeyRSA
	if _, ok := kp.PrivateKey.(*ecdsa.PrivateKey); ok {
		keyType = KeyECDSA
	}
	return Authority{KeyPair: kp, KeyType: keyType, KeySize: orDefault(keySize)}, nil
}

// IssueServer returns a new certificate for the OpenVPN server.
func (ca Authority) IssueServer(name string, validity time.Duration) (KeyPair, error) {
	return ca.issue(name, validity, roleServer)
}

// IssueClient returns a new certificate for a client device.
func (ca Authority) IssueClient(name string, validity time.Duration) (KeyPair, error) {
	return ca.issue(name, validity, roleClient)
}

// CreateCRL returns a PEM encoded certificate revocation list naming every
// given certificate serial. Serials that cannot be read are skipped, so that
// one bad row cannot stop the list being published.
func (ca Authority) CreateCRL(serials []string) (string, error) {
	now := time.Now()

	revoked := make([]x509.RevocationListEntry, 0, len(serials))
	for _, serial := range serials {
		number, ok := new(big.Int).SetString(strings.TrimSpace(serial), 10)
		if !ok {
			continue
		}
		revoked = append(revoked, x509.RevocationListEntry{
			SerialNumber:   number,
			RevocationTime: now.Add(-backdate),
		})
	}

	template := &x509.RevocationList{
		RevokedCertificateEntries: revoked,
		Number:                    big.NewInt(now.Unix()),
		ThisUpdate:                now.Add(-backdate),
		NextUpdate:                now.Add(Validity),
		SignatureAlgorithm:        ca.signatureAlgorithm(),
	}

	der, err := x509.CreateRevocationList(rand.Reader, template, ca.Certificate, ca.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("pki: create revocation list: %w", err)
	}
	return encodePEM(blockCRL, der), nil
}

// SelfSigned returns a self-signed certificate, used for the web server's
// default TLS certificate before an administrator supplies their own.
func SelfSigned(name string, keySize int, serverAuth bool) (KeyPair, error) {
	ca := Authority{KeyType: KeyRSA, KeySize: orDefault(keySize)}

	kind := roleCA
	if serverAuth {
		// The web certificate vouches for itself and terminates TLS, so it
		// needs both the authority and the server roles.
		kind = roleServer
	}

	kp, err := ca.issueSelfSigned(name, Validity, kind)
	if err != nil {
		return KeyPair{}, fmt.Errorf("pki: create self-signed certificate: %w", err)
	}
	return kp, nil
}

// issue returns a new key pair signed by the authority. A certificate
// authority signs its own.
func (ca Authority) issue(name string, validity time.Duration, kind role) (KeyPair, error) {
	if kind == roleCA {
		return ca.issueSelfSigned(name, validity, kind)
	}
	if ca.Certificate == nil || ca.PrivateKey == nil {
		return KeyPair{}, fmt.Errorf("pki: issue %q: the certificate authority is not loaded", name)
	}

	key, template, err := ca.prepare(name, validity, kind)
	if err != nil {
		return KeyPair{}, err
	}

	return sign(template, ca.Certificate, ca.PrivateKey, key)
}

// issueSelfSigned returns a new key pair that signs itself.
func (ca Authority) issueSelfSigned(name string, validity time.Duration, kind role) (KeyPair, error) {
	key, template, err := ca.prepare(name, validity, kind)
	if err != nil {
		return KeyPair{}, err
	}

	// A self-signed certificate is its own issuer, and when it is also an
	// authority it must be allowed to sign certificates and revocation lists.
	template.Issuer = template.Subject
	template.IsCA = true
	template.MaxPathLen = 0
	template.MaxPathLenZero = true
	template.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign

	return sign(template, template, key, key)
}

// prepare generates a private key and the certificate template for a role.
func (ca Authority) prepare(name string, validity time.Duration, kind role) (crypto.Signer, *x509.Certificate, error) {
	key, err := ca.generateKey()
	if err != nil {
		return nil, nil, fmt.Errorf("pki: generate private key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return nil, nil, err
	}

	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: name},
		NotBefore:             now.Add(-backdate),
		NotAfter:              now.Add(validity),
		SignatureAlgorithm:    ca.signatureAlgorithm(),
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyAgreement,
	}

	switch kind {
	case roleCA:
		template.IsCA = true
		template.MaxPathLen = 0
		template.MaxPathLenZero = true
		template.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	case roleServer:
		// Only an RSA key encrypts the key exchange itself.
		if ca.KeyType != KeyECDSA {
			template.KeyUsage |= x509.KeyUsageKeyEncipherment
		}
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	case roleClient:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	return key, template, nil
}

// sign creates the certificate and pairs it with its private key.
func sign(template, parent *x509.Certificate, signer, key crypto.Signer) (KeyPair, error) {
	der, err := x509.CreateCertificate(rand.Reader, template, parent, key.Public(), signer)
	if err != nil {
		return KeyPair{}, fmt.Errorf("pki: sign certificate: %w", err)
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return KeyPair{}, fmt.Errorf("pki: parse signed certificate: %w", err)
	}
	return KeyPair{Certificate: certificate, PrivateKey: key}, nil
}

// generateKey returns a new private key of the authority's type.
func (ca Authority) generateKey() (crypto.Signer, error) {
	if ca.KeyType == KeyECDSA {
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	return rsa.GenerateKey(rand.Reader, orDefault(ca.KeySize))
}

// signatureAlgorithm is how the authority signs, which follows its key.
func (ca Authority) signatureAlgorithm() x509.SignatureAlgorithm {
	if ca.KeyType == KeyECDSA {
		return x509.ECDSAWithSHA256
	}
	return x509.SHA512WithRSA
}

// randomSerial returns a positive 159 bit certificate serial number.
func randomSerial() (*big.Int, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	if err != nil {
		return nil, fmt.Errorf("pki: generate serial number: %w", err)
	}
	return serial.Add(serial, big.NewInt(1)), nil
}

// orDefault substitutes the default key size for an unset one.
func orDefault(keySize int) int {
	if keySize <= 0 {
		return DefaultKeySize
	}
	return keySize
}
