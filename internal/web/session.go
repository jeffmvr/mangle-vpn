package web

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// Session settings, carried over from the Django deployment.
const (
	// sessionIdleTimeout is how long a session survives without a request.
	// Every request refreshes it.
	sessionIdleTimeout = 15 * time.Minute

	// sessionLifetime caps how long a session can live however active it is.
	sessionLifetime = 12 * time.Hour

	// sessionCookie is the name of the session cookie.
	sessionCookie = "sessionid"
)

// Session keys.
const (
	// sessionUserID holds the signed in user's identifier.
	sessionUserID = "user_id"

	// sessionMFAConfirmed records that the user has entered a valid
	// two-factor code during this session.
	sessionMFAConfirmed = "mfa_confirmed"

	// sessionOAuthState holds the OAuth2 state parameter between the
	// redirect to the provider and the callback.
	sessionOAuthState = "oauth_state"

	// sessionOAuthNonce ties an OpenID Connect ID token to the sign in
	// that asked for it, so one issued for another sign in is refused.
	sessionOAuthNonce = "oauth_nonce"

	// sessionEpoch holds the user's session epoch at sign in. A session
	// whose epoch is behind the user's has been ended.
	sessionEpoch = "session_epoch"

	// sessionError holds a message to show on the next page rendered.
	sessionError = "error"

	// sessionForm holds submitted values and validation errors so that a
	// rejected form can be shown again with what the user typed.
	sessionForm = "form"
)

// newSessionManager returns the session manager.
//
// Sessions are kept in the database rather than in the cookie, so that
// signing a user out takes effect immediately and the cookie carries nothing
// but an opaque token.
func newSessionManager(sessions *store.SessionStore, secure bool, idle, lifetime time.Duration) *scs.SessionManager {
	manager := scs.New()
	manager.Store = sessions
	manager.IdleTimeout = cmp.Or(idle, sessionIdleTimeout)
	manager.Lifetime = cmp.Or(lifetime, sessionLifetime)

	manager.Cookie.Name = sessionCookie
	manager.Cookie.HttpOnly = true
	manager.Cookie.SameSite = http.SameSiteLaxMode
	manager.Cookie.Secure = secure

	// The session ends when the browser does, which is what the previous
	// deployment did and what suits an administrative tool.
	manager.Cookie.Persist = false

	return manager
}

// SignIn records a successful authentication in the session.
//
// Everything already in the session is discarded first. Renewing the token
// alone would carry it over, including another account's confirmed
// two-factor flag, which would let someone already signed in take over any
// account whose password they know without its code. The token is then
// replaced so that one observed before sign in cannot be used afterwards.
//
// The CSRF token is replaced too, so that a token planted in the browser
// before sign in, by a page on a sibling domain for instance, is worthless
// after it.
func (s *Server) SignIn(w http.ResponseWriter, r *http.Request, user *model.User) error {
	ctx := r.Context()

	if err := s.sessions.Clear(ctx); err != nil {
		return err
	}
	if err := s.sessions.RenewToken(ctx); err != nil {
		return err
	}
	s.sessions.Put(ctx, sessionUserID, user.ID.String())
	s.sessions.Put(ctx, sessionEpoch, user.SessionEpoch)
	s.setCSRFCookie(w, newCSRFToken())

	// Every way in records the sign in, so that the time shown against an
	// account is accurate however the user got there.
	if err := s.app.Store.Users.TouchLastLogin(ctx, user); err != nil {
		s.app.Log.Error("failed to record the sign in time", "err", err)
	}
	return nil
}

// SignOut abandons the session.
func (s *Server) SignOut(ctx context.Context) error {
	return s.sessions.Destroy(ctx)
}

// ConfirmMFA records that the user has passed two-factor authentication.
// The token is replaced again, since the session has just gained access.
func (s *Server) ConfirmMFA(ctx context.Context) error {
	if err := s.sessions.RenewToken(ctx); err != nil {
		return err
	}
	s.sessions.Put(ctx, sessionMFAConfirmed, true)
	return nil
}

// MFAConfirmed reports whether two-factor authentication has been passed in
// this session.
func (s *Server) MFAConfirmed(ctx context.Context) bool {
	return s.sessions.GetBool(ctx, sessionMFAConfirmed)
}

// SessionUser returns the signed in user, or nil when there is none.
//
// The user is read back from the database on each request rather than being
// cached in the session, so that a change to their group, their permissions,
// or their enabled flag takes effect at once.
func (s *Server) SessionUser(ctx context.Context) *model.User {
	raw := s.sessions.GetString(ctx, sessionUserID)
	if raw == "" {
		return nil
	}

	id, err := uuid.Parse(raw)
	if err != nil {
		return nil
	}

	user, err := s.app.Store.Users.Get(ctx, id)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			s.app.Log.Error("failed to load the session user", "err", err)
		}
		return nil
	}

	// Ended by a password change or reset since it began.
	if s.sessions.GetInt(ctx, sessionEpoch) != user.SessionEpoch {
		return nil
	}
	return user
}

// PutError stores a message to show on the next page rendered.
func (s *Server) PutError(ctx context.Context, message string) {
	s.sessions.Put(ctx, sessionError, message)
}

// takeError removes and returns the pending message, if any.
func (s *Server) takeError(ctx context.Context) string {
	return s.sessions.PopString(ctx, sessionError)
}
