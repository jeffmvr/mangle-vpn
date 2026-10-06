package web

import (
	"context"
	"net/http"

	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// access is how far a session has got through signing in. The stages are
// ordered: each one is reached only after all those before it. Pages and
// API routes each name the stage they need, and accessFor is the one place
// that decides which stage a session is at, so the pages and the API cannot
// drift apart.
type access int

const (
	// accessNone: not signed in, or the account is no longer active.
	accessNone access = iota
	// accessNeedsMFASetup: the password is right, but the account must
	// enrol an authenticator before going further.
	accessNeedsMFASetup
	// accessNeedsMFACode: enrolled, but this session has not entered a code.
	accessNeedsMFACode
	// accessNeedsNewPassword: fully signed in on a temporary password, which
	// must be replaced before the account is used.
	accessNeedsNewPassword
	// accessFull: nothing left to do.
	accessFull
)

// accessFor returns the stage the session's user is at.
func (s *Server) accessFor(ctx context.Context, user *model.User) access {
	switch {
	case user == nil || !user.IsActive():
		return accessNone
	case user.MFARequired() && !user.MFAEnabled:
		return accessNeedsMFASetup
	case user.MFARequired() && !s.MFAConfirmed(ctx):
		return accessNeedsMFACode
	case user.PasswordChange:
		return accessNeedsNewPassword
	default:
		return accessFull
	}
}

// accessPages is the page that takes a session on from each stage.
var accessPages = map[access]string{
	accessNone:             "/login",
	accessNeedsMFASetup:    "/mfa/setup",
	accessNeedsMFACode:     "/mfa",
	accessNeedsNewPassword: "/password",
}

// accessReasons is what the API answers a session stopped at each stage
// with, which the frontend turns into the same pages.
var accessReasons = map[access]string{
	accessNone:             reasonNotSignedIn,
	accessNeedsMFASetup:    reasonMFANotEnabled,
	accessNeedsMFACode:     reasonMFAUnconfirmed,
	accessNeedsNewPassword: reasonPasswordChange,
}

// requirePage admits a session that has reached the given stage, sending
// any other to the page that takes it on from where it is.
func (s *Server) requirePage(needed access, next http.Handler) http.Handler {
	return s.requireInstalled(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := s.SessionUser(r.Context())

		reached := s.accessFor(r.Context(), user)
		if reached < needed {
			// A user disabled since signing in loses the session outright.
			if reached == accessNone && user != nil {
				s.SignOut(r.Context())
			}
			http.Redirect(w, r, accessPages[reached], http.StatusFound)
			return
		}
		next.ServeHTTP(w, withUser(r, user))
	}))
}

// requireAPIUser admits only a session that is fully signed in.
func (s *Server) requireAPIUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := s.SessionUser(r.Context())

		if reached := s.accessFor(r.Context(), user); reached < accessFull {
			s.forbidden(w, r, accessReasons[reached])
			return
		}
		next.ServeHTTP(w, withUser(r, user))
	})
}

// requireAPIStaff admits an administrator or a member of the help desk,
// from a network the administration pages may be used from.
func (s *Server) requireAPIStaff(next http.Handler) http.Handler {
	return s.requireAPIUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case !currentUser(r).IsStaff():
			s.forbidden(w, r, "You do not have permission to perform this action.")
		case !s.app.AdminAllowedFrom(s.clientIP(r)):
			s.forbidden(w, r, adminNetworkMessage)
		default:
			next.ServeHTTP(w, r)
		}
	}))
}

// requireAPIAdmin admits only an administrator, from a network the
// administration pages may be used from.
func (s *Server) requireAPIAdmin(next http.Handler) http.Handler {
	return s.requireAPIUser(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case !currentUser(r).IsAdmin:
			s.forbidden(w, r, "You do not have permission to perform this action.")
		case !s.app.AdminAllowedFrom(s.clientIP(r)):
			s.forbidden(w, r, adminNetworkMessage)
		default:
			next.ServeHTTP(w, r)
		}
	}))
}

// adminNetworkMessage refuses a member of staff outside the networks the
// administration pages are limited to.
const adminNetworkMessage = "Administration isn't available from your network."
