package web

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
	"golang.org/x/oauth2"
)

// ProviderGoogle is the name of the Google OAuth2 provider.
const ProviderGoogle = "google"

// oauth2Timeout bounds the calls made to the provider.
const oauth2Timeout = 15 * time.Second

// oauth2Provider describes an OAuth2 provider the application can sign users
// in through.
type oauth2Provider struct {
	endpoint   oauth2.Endpoint
	profileURL string
	scopes     []string

	// authCodeOptions are added to the authorization redirect.
	authCodeOptions []oauth2.AuthCodeOption
}

// oauth2Providers holds the supported providers.
var oauth2Providers = map[string]oauth2Provider{
	ProviderGoogle: {
		// Spelled out rather than taken from x/oauth2/google, whose
		// package exists mainly to detect Google Cloud credentials and
		// pulls in the compute metadata client to do it.
		endpoint: oauth2.Endpoint{
			AuthURL:   "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:  "https://oauth2.googleapis.com/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		profileURL: "https://www.googleapis.com/userinfo/v2/me",
		scopes: []string{
			"openid",
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		// Show the account chooser rather than silently reusing whichever
		// account the browser is already signed in to.
		authCodeOptions: []oauth2.AuthCodeOption{
			oauth2.SetAuthURLParam("prompt", "select_account"),
		},
	},
}

// oauth2Profile is the part of a provider's user profile the application
// uses.
type oauth2Profile struct {
	Email string `json:"email"`
	Name  string `json:"name"`

	// VerifiedEmail is Google's word that the account owns the address.
	// Without it, anyone could add a colleague's address to an account of
	// their own and sign in as them.
	VerifiedEmail bool `json:"verified_email"`

	// HostedDomain is the Google Workspace domain the account belongs to,
	// empty for a personal account.
	HostedDomain string `json:"hd"`
}

// oauth2Config returns the OAuth2 configuration and provider named in the
// application settings.
func (s *Server) oauth2Config() (*oauth2.Config, oauth2Provider, error) {
	name := s.app.Config.Get(config.OAuth2Provider)

	provider, ok := oauth2Providers[name]
	if !ok {
		return nil, provider, fmt.Errorf("web: unknown OAuth2 provider %q", name)
	}

	clientID := s.app.Config.Get(config.OAuth2ClientID)
	clientSecret := s.app.Config.Get(config.OAuth2ClientSecret)
	if clientID == "" || clientSecret == "" {
		return nil, provider, errors.New("web: the OAuth2 client credentials are not set")
	}

	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.endpoint,
		RedirectURL:  s.app.Config.URL("oauth"),
		Scopes:       provider.scopes,
	}, provider, nil
}

// showOAuth2Login sends the visitor to the provider's sign in page.
func (s *Server) showOAuth2Login(w http.ResponseWriter, r *http.Request) {
	if s.app.Config.Get(config.OAuth2Provider) == ProviderOIDC {
		s.showOIDCLogin(w, r)
		return
	}

	cfg, provider, err := s.oauth2Config()
	if err != nil {
		s.app.Log.Error("cannot start OAuth2 sign in", "err", err)
		s.PutError(r.Context(), "Single sign-on is not configured.")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// The state parameter ties the callback back to this browser, so that
	// an attacker cannot have their own authorization code redeemed here.
	state := newOAuth2State()
	s.sessions.Put(r.Context(), sessionOAuthState, state)

	options := provider.authCodeOptions
	if domain := s.app.Config.Get(config.OAuth2AllowedDomain); domain != "" {
		// Only a hint to Google's account chooser; the callback enforces it.
		options = append(slices.Clone(options), oauth2.SetAuthURLParam("hd", domain))
	}
	http.Redirect(w, r, cfg.AuthCodeURL(state, options...), http.StatusFound)
}

// processOAuth2 handles the provider's callback and signs the user in.
func (s *Server) processOAuth2(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	user, err := s.authenticateOAuth2(r)
	if err != nil {
		s.app.Log.Warn("OAuth2 sign in failed", "err", err, "remote", s.clientIP(r))
		s.SignOut(ctx)
		s.PutError(ctx, "Invalid username or password.")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	if err := s.SignIn(w, r, user); err != nil {
		s.app.Log.Error("failed to start a session", "err", err)
	}
	s.afterSignIn(r, user)

	http.Redirect(w, r, "/", http.StatusFound)
}

// authenticateOAuth2 validates the callback and returns the user it
// identifies.
func (s *Server) authenticateOAuth2(r *http.Request) (*model.User, error) {
	ctx := r.Context()

	expected := s.sessions.PopString(ctx, sessionOAuthState)
	state, code := r.URL.Query().Get("state"), r.URL.Query().Get("code")

	switch {
	case expected == "":
		return nil, errors.New("web: no OAuth2 state in the session")
	case state == "":
		return nil, errors.New("web: no OAuth2 state in the callback")
	case code == "":
		return nil, errors.New("web: no OAuth2 code in the callback")
	case subtle.ConstantTimeCompare([]byte(state), []byte(expected)) != 1:
		return nil, errors.New("web: the OAuth2 state did not match")
	}

	var profile oauth2Profile
	var err error
	if s.app.Config.Get(config.OAuth2Provider) == ProviderOIDC {
		profile, err = s.fetchOIDCProfile(ctx, code, s.sessions.PopString(ctx, sessionOAuthNonce))
	} else {
		profile, err = s.fetchOAuth2Profile(ctx, code)
	}
	if err != nil {
		return nil, err
	}
	if err := acceptOAuth2Profile(profile, s.app.Config.Get(config.OAuth2AllowedDomain)); err != nil {
		return nil, err
	}

	// Single sign-on authenticates an existing account. It creates one only
	// when the settings name a group for newcomers, and then only for the
	// allowed domain, which acceptOAuth2Profile has already checked.
	user, err := s.app.Store.Users.ByEmail(ctx, profile.Email)
	if errors.Is(err, store.ErrNotFound) {
		user, err = s.createSSOAccount(ctx, profile)
	}
	if err != nil {
		return nil, err
	}
	if !user.IsActive() {
		return nil, fmt.Errorf("web: the account for %s is not active", profile.Email)
	}

	if user.Name == "" && profile.Name != "" {
		user.Name = profile.Name
		if err := s.app.Store.Users.SetName(ctx, user); err != nil {
			return nil, err
		}
	}

	// Signing in through a provider means there is no local password to
	// rotate, so the user is never sent to the password page.
	if user.PasswordChange {
		user.PasswordChange = false
		if err := s.app.Store.Users.SetPassword(ctx, user); err != nil {
			return nil, err
		}
	}
	return user, nil
}

// ssoOnly reports whether single sign-on is the only way in, for everyone
// but administrators.
func (s *Server) ssoOnly() bool {
	return s.app.Config.Get(config.OAuth2Provider) != config.NoOAuth2Provider &&
		s.app.Config.Bool(config.OAuth2Only, false)
}

// createSSOAccount makes an account for someone signing in through the
// provider for the first time, in the group the settings name for
// newcomers. Without one, or without a domain limiting who that can be, it
// refuses.
func (s *Server) createSSOAccount(ctx context.Context, profile oauth2Profile) (*model.User, error) {
	groupID, err := uuid.Parse(s.app.Config.Get(config.OAuth2AutoGroup))
	if err != nil || s.app.Config.Get(config.OAuth2AllowedDomain) == "" || !s.app.Store.Groups.Exists(ctx, groupID) {
		return nil, fmt.Errorf("web: no account for %s", profile.Email)
	}

	user := &model.User{Email: profile.Email, Name: profile.Name, GroupID: groupID, IsEnabled: true}
	user.ClearPassword()
	if err := s.app.SaveUser(ctx, user); err != nil {
		return nil, err
	}

	s.app.RecordEvent(ctx, user, model.EventAccountCreate,
		fmt.Sprintf("Account created on first single sign-on, in %s.", user.Group.Name))
	s.app.Log.Info("created an account on first single sign-on", "user", user.Email, "group", user.Group.Name)
	return user, nil
}

// acceptOAuth2Profile checks that a provider's profile may sign in: the
// provider must have verified the address, and when the sign in is limited
// to one Google Workspace domain, the account must belong to it.
func acceptOAuth2Profile(profile oauth2Profile, allowedDomain string) error {
	if profile.Email == "" || !profile.VerifiedEmail {
		return fmt.Errorf("web: the provider has not verified the address %q", profile.Email)
	}
	if allowedDomain != "" && !strings.EqualFold(profile.HostedDomain, allowedDomain) {
		return fmt.Errorf("web: %s is not an account of %s", profile.Email, allowedDomain)
	}
	return nil
}

// fetchOAuth2Profile exchanges an authorization code for the user's profile.
func (s *Server) fetchOAuth2Profile(ctx context.Context, code string) (oauth2Profile, error) {
	var profile oauth2Profile

	cfg, provider, err := s.oauth2Config()
	if err != nil {
		return profile, err
	}

	ctx, cancel := context.WithTimeout(ctx, oauth2Timeout)
	defer cancel()

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return profile, fmt.Errorf("web: exchange the OAuth2 code: %w", err)
	}

	resp, err := cfg.Client(ctx, token).Get(provider.profileURL)
	if err != nil {
		return profile, fmt.Errorf("web: fetch the OAuth2 profile: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return profile, fmt.Errorf("web: the OAuth2 profile request returned %s: %s",
			resp.Status, body)
	}

	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&profile); err != nil {
		return profile, fmt.Errorf("web: decode the OAuth2 profile: %w", err)
	}
	if profile.Email == "" {
		return profile, errors.New("web: the OAuth2 profile carried no e-mail address")
	}
	return profile, nil
}

// showOIDCLogin sends the visitor to the OpenID Connect provider.
func (s *Server) showOIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	cfg, _, err := s.oidcConfig(ctx)
	if err != nil {
		s.app.Log.Error("cannot start OpenID Connect sign in", "err", err)
		s.PutError(ctx, "Single sign-on is not available right now.")
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	state, nonce := newOAuth2State(), newOAuth2State()
	s.sessions.Put(ctx, sessionOAuthState, state)
	s.sessions.Put(ctx, sessionOAuthNonce, nonce)

	options := []oauth2.AuthCodeOption{oidc.Nonce(nonce)}
	http.Redirect(w, r, cfg.AuthCodeURL(state, options...), http.StatusFound)
}

// newOAuth2State returns a random state parameter.
func newOAuth2State() string {
	return rand.Text()
}
