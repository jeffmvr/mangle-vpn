package web

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/jeffmvr/mangle-vpn/internal/config"
)

// fakeIdP is an OpenID Connect provider that signs whatever ID token the
// test asks for.
type fakeIdP struct {
	server *httptest.Server
	key    *rsa.PrivateKey

	// claims shapes the next ID token; the test fills in the nonce.
	claims map[string]any
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	idp := &fakeIdP{key: key}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                idp.server.URL,
			"authorization_endpoint":                idp.server.URL + "/authorize",
			"token_endpoint":                        idp.server.URL + "/token",
			"jwks_uri":                              idp.server.URL + "/keys",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"},
		}})
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		signer, _ := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
			(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "test"))
		claims := map[string]any{
			"iss": idp.server.URL, "sub": "1234", "aud": "client-id",
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, v := range idp.claims {
			claims[k] = v
		}
		token, _ := jwt.Signed(signer).Claims(claims).Serialize()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"access_token": "access", "token_type": "Bearer", "expires_in": 3600, "id_token": token,
		})
	})
	idp.server = httptest.NewServer(mux)
	t.Cleanup(idp.server.Close)
	return idp
}

// signIn runs a single sign-on through the fake provider and returns where
// the callback sent the browser. tamper may change the claims after the
// nonce is known.
func (idp *fakeIdP) signIn(t *testing.T, b *browser, claims map[string]any, tamper func(map[string]any)) string {
	t.Helper()
	start := b.get("/login/sso")
	to, err := url.Parse(start.Header.Get("Location"))
	provider, _ := url.Parse(idp.server.URL)
	if err != nil || to.Host != provider.Host {
		t.Fatalf("/login/sso went to %q, not the provider", start.Header.Get("Location"))
	}

	idp.claims = map[string]any{"nonce": to.Query().Get("nonce")}
	for k, v := range claims {
		idp.claims[k] = v
	}
	if tamper != nil {
		tamper(idp.claims)
	}

	callback := b.get("/oauth?" + url.Values{"state": {to.Query().Get("state")}, "code": {"code"}}.Encode())
	return callback.Header.Get("Location")
}

func TestOpenIDConnectSignIn(t *testing.T) {
	ts := newTestServer(t)
	ts.install()
	ts.addUser("ada@example.com", false)
	idp := newFakeIdP(t)

	for name, value := range map[string]string{
		config.OAuth2Provider: ProviderOIDC, config.OAuth2Issuer: idp.server.URL,
		config.OAuth2ClientID: "client-id", config.OAuth2ClientSecret: "secret",
		config.OAuth2Name: "Okta",
	} {
		ts.app.Config.Set(t.Context(), name, value)
	}

	if _, page := ts.browser().page("/login"); !strings.Contains(page, "Continue with Okta") {
		t.Error("the sign in page has no button for the provider")
	}

	good := map[string]any{"email": "Ada@Example.com", "email_verified": true}
	for _, tc := range []struct {
		name   string
		claims map[string]any
		tamper func(map[string]any)
		want   string
	}{
		{"verified address", good, nil, "/"},
		{"unverified address", map[string]any{"email": "ada@example.com", "email_verified": false}, nil, "/login"},
		{"no account", map[string]any{"email": "eve@example.com", "email_verified": true}, nil, "/login"},
		{"another sign in's token", good, func(c map[string]any) { c["nonce"] = "someone else's" }, "/login"},
		{"token for another client", good, func(c map[string]any) { c["aud"] = "other-client" }, "/login"},
		{"expired token", good, func(c map[string]any) { c["exp"] = time.Now().Add(-time.Hour).Unix() }, "/login"},
	} {
		b := ts.browser()
		if got := idp.signIn(t, b, tc.claims, tc.tamper); got != tc.want {
			t.Errorf("%s: callback went to %q, want %q", tc.name, got, tc.want)
		}
		signedIn := b.get("/api/profile").StatusCode == http.StatusOK
		if signedIn != (tc.want == "/") {
			t.Errorf("%s: signed in = %v", tc.name, signedIn)
		}
	}
}

func TestOIDCProfileTrustsAnUnstatedAddressOnlyInTheAllowedDomain(t *testing.T) {
	verified, unverified := true, false
	for _, tc := range []struct {
		name   string
		claims oidcClaims
		domain string
		ok     bool
	}{
		{"verified", oidcClaims{Email: "ada@example.com", EmailVerified: &verified}, "", true},
		{"verified, in the domain", oidcClaims{Email: "ada@example.com", EmailVerified: &verified}, "example.com", true},
		{"verified, other domain", oidcClaims{Email: "ada@acme.test", EmailVerified: &verified}, "example.com", false},
		{"unverified", oidcClaims{Email: "ada@example.com", EmailVerified: &unverified}, "example.com", false},
		{"unstated, no domain limit", oidcClaims{Email: "ada@example.com"}, "", false},
		{"unstated, in the domain", oidcClaims{Email: "ada@example.com"}, "example.com", true},
		{"unstated, other domain", oidcClaims{Email: "ada@acme.test"}, "example.com", false},
	} {
		err := acceptOAuth2Profile(oidcProfile(tc.claims, tc.domain), tc.domain)
		if (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok = %v", tc.name, err, tc.ok)
		}
	}
}

func TestValidIssuer(t *testing.T) {
	for issuer, want := range map[string]bool{
		"https://example.okta.com":                   true,
		"https://login.microsoftonline.com/abc/v2.0": true,
		"https://tenant.auth0.com/":                  true,
		"http://127.0.0.1:5556":                      true,
		"http://localhost:8080/dex":                  true,
		"http://idp.example.com":                     false,
		"ftp://example.com":                          false,
		"https://example.com/?x=1":                   false,
		"example.com":                                false,
	} {
		if got := validIssuer(issuer); got != want {
			t.Errorf("validIssuer(%q) = %v, want %v", issuer, got, want)
		}
	}
}
