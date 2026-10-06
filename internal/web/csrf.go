package web

import (
	"crypto/rand"
	"crypto/subtle"
	"net/http"
	"slices"
)

// CSRF protection follows the double submit cookie pattern the Django
// deployment used, because the frontend is built against it: the browser is
// given a readable "csrftoken" cookie and echoes its value back in the
// "X-CSRFToken" header, which a cross-origin page cannot do.
const (
	// csrfCookie is the cookie holding the token. It is deliberately not
	// HttpOnly, because the frontend has to read it.
	csrfCookie = "csrftoken"

	// csrfHeader is the header the frontend echoes the token in.
	csrfHeader = "X-CSRFToken"

	// csrfFormField is the hidden field the server rendered forms carry.
	csrfFormField = "csrfmiddlewaretoken"

	// csrfTokenLength is the length of a token, in characters.
	csrfTokenLength = 64
)

// csrfAlphabet is the character set a token is drawn from.
const csrfAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// safeMethods never change state and so are exempt.
var safeMethods = []string{http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace}

// newCSRFToken returns a fresh token.
func newCSRFToken() string {
	buf := make([]byte, csrfTokenLength)
	rand.Read(buf)
	for i, b := range buf {
		buf[i] = csrfAlphabet[int(b)%len(csrfAlphabet)]
	}
	return string(buf)
}

// csrfTokenFor returns the request's token, issuing and setting one when the
// request does not already carry a usable cookie.
func (s *Server) csrfTokenFor(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(csrfCookie); err == nil && len(cookie.Value) == csrfTokenLength {
		return cookie.Value
	}

	token := newCSRFToken()
	s.setCSRFCookie(w, token)
	return token
}

// setCSRFCookie hands the browser a CSRF token.
func (s *Server) setCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookie,
		Value:    token,
		Path:     "/",
		Secure:   s.secureCookies,
		SameSite: http.SameSiteLaxMode,

		// Readable by script on purpose: the frontend copies it into the
		// request header.
		HttpOnly: false,
	})
}

// protectCSRF rejects unsafe requests that do not prove they came from the
// application itself.
func (s *Server) protectCSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Issue a token on every request so that the first page a visitor
		// loads can already submit a form.
		token := s.csrfTokenFor(w, r)

		if slices.Contains(safeMethods, r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		if !csrfTokenMatches(r, token) {
			s.app.Log.Warn("rejected a request with a bad CSRF token",
				"method", r.Method, "path", r.URL.Path, "remote", s.clientIP(r))
			s.forbidden(w, r, "CSRF token missing or incorrect.")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// csrfTokenMatches reports whether the request presents the expected token,
// in either the header the frontend uses or the field a rendered form
// carries.
func csrfTokenMatches(r *http.Request, expected string) bool {
	if expected == "" {
		return false
	}

	presented := r.Header.Get(csrfHeader)
	if presented == "" {
		// ParseForm is safe to call here: the handlers that follow read the
		// parsed values rather than the body.
		if err := r.ParseForm(); err == nil {
			presented = r.PostForm.Get(csrfFormField)
		}
	}

	return subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) == 1
}
