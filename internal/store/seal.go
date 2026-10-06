package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Secrets are kept in the database encrypted: users' two-factor secrets, and
// the settings that hold keys and passwords. The key is derived from the
// application's secret key, which is a file outside the database, so a copy
// of the database alone, a leaked backup say, gives none of them away.
//
// An encrypted value is "enc:v1:" followed by the base64 of a nonce and the
// AES-GCM ciphertext. A value without the prefix is plaintext: what the
// Django release wrote, or anything written before encryption was turned
// on. Reading accepts both, and SealExisting converts the second into the
// first.

// sealedPrefix marks an encrypted value.
const sealedPrefix = "enc:v1:"

// sealInfo binds the derived key to this use, so the same secret key used
// elsewhere yields an unrelated key.
const sealInfo = "mangle-vpn stored secrets v1"

// sealer encrypts and decrypts stored secrets.
type sealer struct {
	aead     cipher.AEAD
	settings []string
}

// UseSecretKey turns on encryption of stored secrets, with a key derived
// from secretKey. secretSettings names the settings whose values are
// secrets.
func (s *Store) UseSecretKey(secretKey []byte, secretSettings []string) error {
	key, err := hkdf.Key(sha256.New, secretKey, nil, sealInfo, 32)
	if err != nil {
		return fmt.Errorf("store: derive the secrets key: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("store: secrets cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("store: secrets cipher: %w", err)
	}
	s.sealer = &sealer{aead: aead, settings: secretSettings}
	return nil
}

// seal encrypts a value for storage. An empty value, one already encrypted,
// or any value when encryption is off, is returned as it is.
func (s *Store) seal(value string) string {
	if s.sealer == nil || value == "" || strings.HasPrefix(value, sealedPrefix) {
		return value
	}
	nonce := make([]byte, s.sealer.aead.NonceSize())
	rand.Read(nonce)
	sealed := s.sealer.aead.Seal(nonce, nonce, []byte(value), nil)
	return sealedPrefix + base64.StdEncoding.EncodeToString(sealed)
}

// unseal decrypts a stored value. Plaintext is returned as it is.
func (s *Store) unseal(value string) (string, error) {
	encoded, sealed := strings.CutPrefix(value, sealedPrefix)
	if !sealed {
		return value, nil
	}
	if s.sealer == nil {
		return "", errors.New("store: an encrypted value was read without the secret key")
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	size := s.sealer.aead.NonceSize()
	if err != nil || len(data) < size {
		return "", errors.New("store: an encrypted value is malformed")
	}
	plain, err := s.sealer.aead.Open(nil, data[:size], data[size:], nil)
	if err != nil {
		// The secret key file was replaced, or the value was tampered with.
		return "", errors.New("store: an encrypted value does not match the secret key")
	}
	return string(plain), nil
}

// isSecretSetting reports whether a setting's value is a secret.
func (s *Store) isSecretSetting(name string) bool {
	return s.sealer != nil && slices.Contains(s.sealer.settings, name)
}

// SealExisting encrypts every secret still stored as plaintext. It runs at
// start up and does nothing once everything is encrypted.
func (s *Store) SealExisting(ctx context.Context) error {
	if s.sealer == nil {
		return nil
	}
	return s.rewriteSecrets(ctx, func(value string) (string, error) {
		return s.seal(value), nil
	})
}

// UnsealAll writes every secret back as plaintext, which is what the Django
// release can read. It is the step before rolling back to it.
func (s *Store) UnsealAll(ctx context.Context) error {
	return s.rewriteSecrets(ctx, s.unseal)
}

// rewriteSecrets passes every stored secret through convert, writing back
// those it changes, in one transaction.
func (s *Store) rewriteSecrets(ctx context.Context, convert func(string) (string, error)) error {
	type row struct{ key, value string }

	read := func(query string, args ...any) ([]row, error) {
		rows, err := s.db.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.key, &r.value); err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, rows.Err()
	}

	users, err := read(`SELECT id, mfa_secret FROM "users" WHERE mfa_secret != ''`)
	if err != nil {
		return translate(err)
	}
	settings, err := read(`SELECT name, value FROM "setting" WHERE value != ''`)
	if err != nil {
		return translate(err)
	}

	return s.inTx(ctx, func(tx *sql.Tx) error {
		for _, u := range users {
			value, err := convert(u.value)
			if err != nil {
				return err
			}
			if value != u.value {
				if _, err := tx.ExecContext(ctx, `UPDATE "users" SET mfa_secret = ? WHERE id = ?`, value, u.key); err != nil {
					return err
				}
			}
		}
		for _, setting := range settings {
			if s.sealer != nil && !s.isSecretSetting(setting.key) && !strings.HasPrefix(setting.value, sealedPrefix) {
				continue
			}
			value, err := convert(setting.value)
			if err != nil {
				return err
			}
			if value != setting.value {
				if _, err := tx.ExecContext(ctx, `UPDATE "setting" SET value = ? WHERE name = ?`, value, setting.key); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
