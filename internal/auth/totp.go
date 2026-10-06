package auth

import (
	"encoding/base32"
	"log/slog"
	"strings"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// TOTP parameters. These match the defaults every authenticator app assumes,
// and the ones the previous Django release enrolled users against.
var totpOpts = totp.ValidateOpts{
	Period:    30,
	Digits:    otp.DigitsSix,
	Algorithm: otp.AlgorithmSHA1,

	// Only the current time step is accepted, matching the behaviour users
	// are already enrolled against. Raising this to 1 would forgive up to
	// thirty seconds of clock drift on the client.
	Skew: 0,
}

// secretBytes is the size of a generated two-factor secret: 160 bits, the
// length RFC 4226 recommends for HMAC-SHA1. It encodes to 32 base32
// characters. Secrets issued before, at 80 bits, keep working.
const secretBytes = 20

// base32NoPadding is the encoding authenticator apps expect for secrets.
var base32NoPadding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret returns a random base32 encoded two-factor secret.
func NewTOTPSecret() string {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Mangle",
		AccountName: "setup",
		SecretSize:  secretBytes,
		Period:      totpOpts.Period,
		Digits:      totpOpts.Digits,
		Algorithm:   totpOpts.Algorithm,
	})
	if err != nil {
		// Generate only fails when the options are inconsistent or the
		// system random source is unavailable; neither is recoverable here.
		panic("auth: generate two-factor secret: " + err.Error())
	}
	return key.Secret()
}

// TOTPCode returns the authentication code for the secret at the given time.
func TOTPCode(secret string, at time.Time) string {
	code, err := totp.GenerateCodeCustom(normalizeSecret(secret), at, totpOpts)
	if err != nil {
		slog.Debug("failed to generate two-factor code", "err", err)
		return ""
	}
	return code
}

// VerifyTOTPAt reports whether code is the authentication code for the
// secret at the given time.
func VerifyTOTPAt(secret, code string, at time.Time) bool {
	if secret == "" || code == "" {
		return false
	}
	ok, err := totp.ValidateCustom(strings.TrimSpace(code), normalizeSecret(secret), at, totpOpts)
	return ok && err == nil
}

// TOTPStep returns the time step a code generated at the given time belongs
// to. With no skew allowed, it is the only step VerifyTOTPAt accepts then.
func TOTPStep(at time.Time) int64 {
	return at.Unix() / int64(totpOpts.Period)
}

// TOTPProvisioningURI returns the otpauth URI an authenticator app scans to
// enrol the account.
func TOTPProvisioningURI(secret, account, issuer string) string {
	raw, err := base32NoPadding.DecodeString(normalizeSecret(secret))
	if err != nil {
		slog.Error("failed to decode two-factor secret", "account", account, "err", err)
		return ""
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: account,
		Secret:      raw,
		Period:      totpOpts.Period,
		Digits:      totpOpts.Digits,
		Algorithm:   totpOpts.Algorithm,
	})
	if err != nil {
		slog.Error("failed to build provisioning URI", "account", account, "err", err)
		return ""
	}
	return key.URL()
}

// normalizeSecret puts a stored secret into the unpadded uppercase base32
// form the library expects, tolerating the spacing and casing a user might
// type in by hand.
func normalizeSecret(secret string) string {
	s := strings.ToUpper(strings.ReplaceAll(secret, " ", ""))
	return strings.TrimRight(s, "=")
}
