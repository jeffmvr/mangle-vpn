package web

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"golang.org/x/oauth2"
)

// Sign in through any OpenID Connect provider: Okta, Microsoft Entra ID,
// Authentik, Keycloak and the like. The provider's endpoints and signing
// keys are discovered from its issuer URL, and the user is identified by
// the ID token it signs, which go-oidc verifies: the signature against the
// provider's published keys, the issuer, the audience, and the expiry.

// ProviderOIDC is the name of the OpenID Connect provider.
const ProviderOIDC = "oidc"

// oidcScopes are what the sign in asks the provider for: the ID token, and
// the address and name in it.
var oidcScopes = []string{oidc.ScopeOpenID, "email", "profile"}

// oidcRefresh is how long a discovered provider is reused before its
// discovery document is fetched again, which picks up rotated keys and
// changed endpoints.
const oidcRefresh = time.Hour

// oidcCache holds the provider discovered from the configured issuer, so
// that a sign in does not fetch the discovery document every time.
type oidcCache struct {
	mu       sync.Mutex
	issuer   string
	provider *oidc.Provider
	fetched  time.Time
}

// get returns the provider for the issuer, discovering it when there is
// none yet, the issuer has changed, or the last discovery is stale.
func (c *oidcCache) get(ctx context.Context, issuer string) (*oidc.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.provider != nil && c.issuer == issuer && time.Since(c.fetched) < oidcRefresh {
		return c.provider, nil
	}

	provider, err := discoverOIDC(ctx, issuer)
	if err != nil {
		return nil, err
	}
	c.issuer, c.provider, c.fetched = issuer, provider, time.Now()
	return provider, nil
}

// discoverOIDC fetches a provider's discovery document.
func discoverOIDC(ctx context.Context, issuer string) (*oidc.Provider, error) {
	ctx, cancel := context.WithTimeout(ctx, oauth2Timeout)
	defer cancel()

	// The provider keeps using the context it was discovered with to fetch
	// signing keys, so it gets one that outlives this call.
	provider, err := oidc.NewProvider(oidc.ClientContext(context.WithoutCancel(ctx), nil), issuer)
	if err != nil {
		return nil, fmt.Errorf("web: discover the OpenID Connect provider %s: %w", issuer, err)
	}
	return provider, nil
}

// oidcConfig returns the OAuth2 configuration and provider for the
// configured OpenID Connect issuer.
func (s *Server) oidcConfig(ctx context.Context) (*oauth2.Config, *oidc.Provider, error) {
	issuer := s.app.Config.Get(config.OAuth2Issuer)
	clientID := s.app.Config.Get(config.OAuth2ClientID)
	clientSecret := s.app.Config.Get(config.OAuth2ClientSecret)
	if issuer == "" || clientID == "" || clientSecret == "" {
		return nil, nil, errors.New("web: the OpenID Connect settings are incomplete")
	}

	provider, err := s.oidc.get(ctx, issuer)
	if err != nil {
		return nil, nil, err
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  s.app.Config.URL("oauth"),
		Scopes:       oidcScopes,
	}, provider, nil
}

// oidcClaims are the ID token claims the sign in reads.
type oidcClaims struct {
	Email string `json:"email"`
	Name  string `json:"name"`

	// EmailVerified is the provider's word that the account owns the
	// address. Some providers, Microsoft Entra ID among them, leave it out.
	EmailVerified *bool `json:"email_verified"`

	// HostedDomain is Google's Workspace domain claim, should Google be
	// set up through OpenID Connect.
	HostedDomain string `json:"hd"`
}

// fetchOIDCProfile exchanges an authorization code for a verified ID token
// and returns the profile it carries. nonce is the one the sign in was
// started with.
func (s *Server) fetchOIDCProfile(ctx context.Context, code, nonce string) (oauth2Profile, error) {
	var profile oauth2Profile

	cfg, provider, err := s.oidcConfig(ctx)
	if err != nil {
		return profile, err
	}

	ctx, cancel := context.WithTimeout(ctx, oauth2Timeout)
	defer cancel()

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return profile, fmt.Errorf("web: exchange the OpenID Connect code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return profile, errors.New("web: the provider returned no ID token")
	}

	idToken, err := provider.Verifier(&oidc.Config{ClientID: cfg.ClientID}).Verify(ctx, raw)
	if err != nil {
		return profile, fmt.Errorf("web: verify the ID token: %w", err)
	}
	if nonce == "" || idToken.Nonce != nonce {
		return profile, errors.New("web: the ID token was not issued for this sign in")
	}

	var claims oidcClaims
	if err := idToken.Claims(&claims); err != nil {
		return profile, fmt.Errorf("web: read the ID token: %w", err)
	}
	return oidcProfile(claims, s.app.Config.Get(config.OAuth2AllowedDomain)), nil
}

// oidcProfile turns ID token claims into a profile acceptOAuth2Profile can
// judge.
//
// An address counts as verified when the provider says so. When the
// provider says nothing, as Microsoft Entra ID does, it counts only if the
// sign in is limited to a domain and the address is in it: limiting the
// sign in to a domain is the administrator saying that this provider
// speaks for that domain's addresses.
func oidcProfile(claims oidcClaims, allowedDomain string) oauth2Profile {
	email := strings.ToLower(strings.TrimSpace(claims.Email))
	_, domain, _ := strings.Cut(email, "@")

	hosted := claims.HostedDomain
	if hosted == "" {
		hosted = domain
	}

	verified := claims.EmailVerified != nil && *claims.EmailVerified
	if claims.EmailVerified == nil && allowedDomain != "" && strings.EqualFold(domain, allowedDomain) {
		verified = true
	}

	return oauth2Profile{
		Email:         email,
		Name:          claims.Name,
		VerifiedEmail: verified,
		HostedDomain:  hosted,
	}
}

// validIssuer reports whether an issuer URL is one the application will
// talk to: HTTPS, or plain HTTP to this machine for testing a provider
// that runs alongside it.
func validIssuer(issuer string) bool {
	u, err := url.Parse(issuer)
	if err != nil || u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		ip := net.ParseIP(host)
		return host == "localhost" || (ip != nil && ip.IsLoopback())
	default:
		return false
	}
}
