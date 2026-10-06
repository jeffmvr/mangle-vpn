package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
)

// PEM block types.
const (
	blockCertificate = "CERTIFICATE"
	blockPrivateKey  = "RSA PRIVATE KEY"
	blockECKey       = "EC PRIVATE KEY"
	blockCRL         = "X509 CRL"
)

// LoadKeyPair rebuilds a key pair from its PEM encoded parts.
func LoadKeyPair(certificatePEM, privateKeyPEM string) (KeyPair, error) {
	certificate, err := ParseCertificate(certificatePEM)
	if err != nil {
		return KeyPair{}, err
	}

	privateKey, err := ParsePrivateKey(privateKeyPEM)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Certificate: certificate, PrivateKey: privateKey}, nil
}

// ParseCertificate reads a PEM encoded certificate.
//
// The first CERTIFICATE block is read, so a bundle that leads with its
// private key, as Let's Encrypt's cache does, is read too.
func ParseCertificate(data string) (*x509.Certificate, error) {
	rest := []byte(data)
	var block *pem.Block
	for {
		block, rest = pem.Decode(rest)
		if block == nil {
			return nil, errors.New("pki: no PEM certificate found")
		}
		if block.Type == "CERTIFICATE" {
			break
		}
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse certificate: %w", err)
	}
	return certificate, nil
}

// ParsePrivateKey reads a PEM encoded RSA or ECDSA private key, in the
// traditional OpenSSL layouts or PKCS #8.
func ParsePrivateKey(data string) (crypto.Signer, error) {
	block, _ := pem.Decode([]byte(data))
	if block == nil {
		return nil, errors.New("pki: no PEM private key found")
	}

	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("pki: parse private key: %w", err)
	}

	switch key := parsed.(type) {
	case *rsa.PrivateKey:
		return key, nil
	case *ecdsa.PrivateKey:
		return key, nil
	}
	return nil, fmt.Errorf("pki: private key is %T, want RSA or ECDSA", parsed)
}

// EncodeCertificate returns the PEM encoding of a certificate.
func EncodeCertificate(certificate *x509.Certificate) string {
	return encodePEM(blockCertificate, certificate.Raw)
}

// EncodePrivateKey returns the PEM encoding of a private key, in the
// traditional OpenSSL layout OpenVPN has always been given.
func EncodePrivateKey(key crypto.Signer) string {
	switch key := key.(type) {
	case *rsa.PrivateKey:
		return encodePEM(blockPrivateKey, x509.MarshalPKCS1PrivateKey(key))
	case *ecdsa.PrivateKey:
		der, err := x509.MarshalECPrivateKey(key)
		if err != nil {
			// Only a curve Go does not support fails, and every key
			// here is P-256.
			panic(fmt.Sprintf("pki: encode ECDSA key: %v", err))
		}
		return encodePEM(blockECKey, der)
	}
	panic(fmt.Sprintf("pki: cannot encode a %T", key))
}

// encodePEM wraps DER bytes in a PEM block.
func encodePEM(blockType string, der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: blockType, Bytes: der}))
}
