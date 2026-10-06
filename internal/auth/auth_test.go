package auth

import (
	"testing"
	"time"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

func TestCheckPasswordAgainstDjangoHashes(t *testing.T) {
	// Hashes produced by Django's PBKDF2PasswordHasher, which the previous
	// release of this application wrote into the database.
	tests := []struct {
		password string
		encoded  string
		want     bool
	}{
		{"Password1", "pbkdf2_sha256$120000$NSVqbRsd9Rhl$uRk5YCo0ctJ9lLHb8VEGa9bnh4ISxE6CSgx4g3LC3U8=", true},
		{"Password2", "pbkdf2_sha256$120000$NSVqbRsd9Rhl$uRk5YCo0ctJ9lLHb8VEGa9bnh4ISxE6CSgx4g3LC3U8=", false},
		{"Password1", "!unusable", false},
		{"Password1", "", false},
		{"Password1", "md5$salt$deadbeef", false},
	}

	for _, tt := range tests {
		if got := CheckPassword(tt.password, tt.encoded); got != tt.want {
			t.Errorf("CheckPassword(%q, %q) = %v, want %v", tt.password, tt.encoded, got, tt.want)
		}
	}
}

func TestHashPasswordRoundTrip(t *testing.T) {
	encoded := HashPassword("Correct Horse 1")

	if !CheckPassword("Correct Horse 1", encoded) {
		t.Fatalf("freshly hashed password did not verify: %s", encoded)
	}
	if CheckPassword("wrong", encoded) {
		t.Fatal("an incorrect password verified")
	}
	if HashPassword("Correct Horse 1") == encoded {
		t.Fatal("two hashes of the same password shared a salt")
	}
}

func TestTOTPCodeRFC6238(t *testing.T) {
	// RFC 6238 appendix B, SHA-1 column, truncated to six digits. The shared
	// secret is the ASCII string "12345678901234567890".
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	tests := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}

	for _, tt := range tests {
		if got := TOTPCode(secret, time.Unix(tt.unix, 0)); got != tt.want {
			t.Errorf("TOTPCode(at %d) = %q, want %q", tt.unix, got, tt.want)
		}
	}
}

func TestVerifyTOTP(t *testing.T) {
	secret := NewTOTPSecret()

	// Twenty random bytes, 160 bits, encode to 32 base32 characters.
	if len(secret) != 32 {
		t.Fatalf("NewTOTPSecret() = %q, want 32 characters", secret)
	}
	if !VerifyTOTPAt(secret, TOTPCode(secret, time.Now()), time.Now()) {
		t.Error("the current code did not verify")
	}
	if VerifyTOTPAt(secret, "000000", time.Now()) && TOTPCode(secret, time.Now()) != "000000" {
		t.Error("an incorrect code verified")
	}
	if VerifyTOTPAt("", "123456", time.Now()) {
		t.Error("an empty secret verified")
	}
}

func TestTOTPProvisioningURI(t *testing.T) {
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"

	raw := TOTPProvisioningURI(secret, "user@example.com", "Mangle VPN")

	// Parsing the URI back must yield a key an authenticator app would enrol
	// against this exact account, with the secret unchanged.
	key, err := otp.NewKeyFromURL(raw)
	if err != nil {
		t.Fatalf("TOTPProvisioningURI() produced an unparsable URI %q: %v", raw, err)
	}

	if got := key.Type(); got != "totp" {
		t.Errorf("key type = %q, want totp", got)
	}
	if got := key.Issuer(); got != "Mangle VPN" {
		t.Errorf("issuer = %q, want %q", got, "Mangle VPN")
	}
	if got := key.AccountName(); got != "user@example.com" {
		t.Errorf("account = %q, want %q", got, "user@example.com")
	}
	if got := key.Secret(); got != secret {
		t.Errorf("secret = %q, want %q", got, secret)
	}

	// A code generated from the enrolled key must satisfy the verifier.
	code, err := totp.GenerateCode(key.Secret(), time.Now())
	if err != nil {
		t.Fatalf("GenerateCode: %v", err)
	}
	if !VerifyTOTPAt(secret, code, time.Now()) {
		t.Error("a code from the provisioned key did not verify")
	}
}

func TestVerifyTOTPRejectsAdjacentWindows(t *testing.T) {
	// Users were enrolled against a verifier that accepted only the current
	// time step, so neither neighbour may be allowed through.
	secret := NewTOTPSecret()
	now := time.Now()

	for _, offset := range []time.Duration{-30 * time.Second, 30 * time.Second} {
		code := TOTPCode(secret, now.Add(offset))
		if code != TOTPCode(secret, now) && VerifyTOTPAt(secret, code, now) {
			t.Errorf("a code from %s away verified", offset)
		}
	}
}

func TestTOTPStepMatchesTheAcceptedCode(t *testing.T) {
	// The step recorded against a user must be the step whose code was
	// accepted, or replay protection would refuse the next valid code.
	at := time.Unix(1_700_000_039, 0) // the last second of a window starting at ..010
	if TOTPStep(at) != TOTPStep(time.Unix(1_700_000_010, 0)) {
		t.Error("two moments in one 30 second window gave different steps")
	}
	if TOTPStep(at.Add(time.Second)) != TOTPStep(at)+1 {
		t.Error("the next window did not give the next step")
	}
}

func TestNeedsRehash(t *testing.T) {
	if NeedsRehash(HashPassword("Password1")) {
		t.Error("a fresh hash asked to be rehashed")
	}
	for _, old := range []string{
		"pbkdf2_sha256$120000$NSVqbRsd9Rhl$uRk5YCo0ctJ9lLHb8VEGa9bnh4ISxE6CSgx4g3LC3U8=",
		"pbkdf2_sha1$260000$salt$hash",
	} {
		if !NeedsRehash(old) {
			t.Errorf("NeedsRehash(%q) = false, want true", old)
		}
	}
}

func TestCheckDecoyPasswordCostsAFullCheck(t *testing.T) {
	CheckDecoyPassword("warm up the decoy hash")

	start := time.Now()
	if CheckDecoyPassword("anything") {
		t.Fatal("the decoy check succeeded")
	}
	decoy := time.Since(start)

	hash := HashPassword("Password1")
	start = time.Now()
	CheckPassword("Password2", hash)
	real := time.Since(start)

	// Not a benchmark, just a guard against the decoy returning early.
	if decoy < real/4 {
		t.Errorf("decoy check took %v against %v for a real one", decoy, real)
	}
}
