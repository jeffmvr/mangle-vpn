// Package auth implements the application's credential primitives: password
// hashing compatible with the previous Django deployment, and TOTP two-factor
// authentication codes.
package auth

import (
	"crypto/hmac"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"sync"
)

// Iterations is the PBKDF2 work factor applied to newly hashed passwords,
// the OWASP recommendation for PBKDF2-SHA256. Django verifies whatever count
// a hash records, so raising it keeps the database usable by that release;
// older hashes are upgraded as their owners sign in (see NeedsRehash).
const Iterations = 600000

// saltLength is the number of random characters in a generated salt.
const saltLength = 12

// alphanumeric is the character set used for salts and generated passwords.
const alphanumeric = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// HashPassword returns an encoded PBKDF2-SHA256 hash of the password in the
// "algorithm$iterations$salt$hash" layout shared with the Django release, so
// that an existing database keeps working unchanged.
func HashPassword(password string) string {
	return encodePBKDF2(password, randomString(saltLength), Iterations, sha256.New, "pbkdf2_sha256")
}

// CheckPassword reports whether password matches the encoded hash. Hashes
// that cannot be parsed, and the "!" marker Django writes for accounts with
// no usable password, never match.
func CheckPassword(password, encoded string) bool {
	algorithm, rest, ok := strings.Cut(encoded, "$")
	if !ok || !IsPasswordUsable(encoded) {
		return false
	}

	var newHash func() hash.Hash
	switch algorithm {
	case "pbkdf2_sha256":
		newHash = sha256.New
	case "pbkdf2_sha1":
		newHash = sha1.New
	default:
		return false
	}

	parts := strings.Split(rest, "$")
	if len(parts) != 3 {
		return false
	}
	iterations, err := strconv.Atoi(parts[0])
	if err != nil || iterations <= 0 {
		return false
	}

	candidate := encodePBKDF2(password, parts[1], iterations, newHash, algorithm)
	return hmac.Equal([]byte(candidate), []byte(encoded))
}

// NeedsRehash reports whether a hash that just verified should be replaced
// with one made at the current strength.
func NeedsRehash(encoded string) bool {
	algorithm, rest, ok := strings.Cut(encoded, "$")
	if !ok || algorithm != "pbkdf2_sha256" {
		return true
	}
	count, _, _ := strings.Cut(rest, "$")
	iterations, err := strconv.Atoi(count)
	return err != nil || iterations < Iterations
}

// decoyHash is a real hash of a random password, made on first use.
var decoyHash = sync.OnceValue(func() string { return HashPassword(randomString(saltLength)) })

// CheckDecoyPassword does the work of checking a password and always fails.
// It stands in for a check against an account that does not exist, or has
// no usable password, so the response takes as long as a real check and
// does not reveal which addresses have accounts.
func CheckDecoyPassword(password string) bool {
	CheckPassword(password, decoyHash())
	return false
}

// IsPasswordUsable reports whether the encoded hash can ever match a
// password. Django marks unusable passwords with a leading "!".
func IsPasswordUsable(encoded string) bool {
	return encoded != "" && !strings.HasPrefix(encoded, "!")
}

// GeneratePassword returns a random alphanumeric password of the given
// length, suitable for an invitation or a password reset.
func GeneratePassword(length int) string {
	return randomString(length)
}

// encodePBKDF2 derives the key and renders it in Django's encoded form.
func encodePBKDF2(password, salt string, iterations int, newHash func() hash.Hash, algorithm string) string {
	key, err := pbkdf2.Key(newHash, password, []byte(salt), iterations, newHash().Size())
	if err != nil {
		// Key only fails on nonsensical parameters, which the callers above
		// have already ruled out.
		panic("auth: pbkdf2: " + err.Error())
	}
	return fmt.Sprintf("%s$%d$%s$%s", algorithm, iterations, salt, base64.StdEncoding.EncodeToString(key))
}

// randomString returns a cryptographically random alphanumeric string.
func randomString(length int) string {
	buf := make([]byte, length)
	rand.Read(buf)
	for i, b := range buf {
		buf[i] = alphanumeric[int(b)%len(alphanumeric)]
	}
	return string(buf)
}
