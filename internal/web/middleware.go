package web

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// contextKey is the type of this package's context keys.
type contextKey int

// userContextKey holds the signed in user for the current request.
const userContextKey contextKey = iota

// withUser returns a request carrying the signed in user.
func withUser(r *http.Request, user *model.User) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userContextKey, user))
}

// currentUser returns the signed in user, or nil when there is none.
func currentUser(r *http.Request) *model.User {
	user, _ := r.Context().Value(userContextKey).(*model.User)
	return user
}

// recoverPanics turns a panic in a handler into a 500 rather than a dropped
// connection, and logs the stack.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				s.app.Log.Error("panic serving request",
					"method", r.Method, "path", r.URL.Path,
					"panic", recovered, "stack", string(debug.Stack()))

				w.Header().Set("Connection", "close")
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// logRequests records the outcome of each request.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(recorder, r)

		level := slog.LevelInfo
		if recorder.status >= http.StatusInternalServerError {
			level = slog.LevelError
		}

		s.app.Log.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.status,
			"duration", time.Since(started).Round(time.Millisecond),
			"remote", s.clientIP(r))
	})
}

// statusRecorder remembers the status code written to a response.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

// WriteHeader implements [http.ResponseWriter].
func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status, r.wroteHeader = status, true
		r.ResponseWriter.WriteHeader(status)
	}
}

// Write implements [io.Writer].
func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}

// reloadConfig refreshes the cached settings at the start of each request.
//
// The web, task, and VPN hook processes each hold their own cache, so a
// setting changed in one is only seen by the others once they reload. Doing
// it per request is what makes a change through the admin UI take effect
// straight away.
func (s *Server) reloadConfig(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := s.app.Config.Reload(r.Context()); err != nil {
			s.app.Log.Error("failed to reload settings", "err", err)
		}
		next.ServeHTTP(w, r)
	})
}

// requireInstalled sends a visitor to the setup page until the application
// has been installed.
func (s *Server) requireInstalled(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.app.Config.Bool(config.AppInstalled, false) {
			http.Redirect(w, r, "/install", http.StatusFound)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the address the request came from.
//
// The forwarded headers are only consulted when the server has been told a
// reverse proxy sets them. Served directly, they are attacker controlled,
// and this address is what lands in the audit log.
func (s *Server) clientIP(r *http.Request) string {
	if s.trustProxy {
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			first, _, _ := strings.Cut(forwarded, ",")
			return strings.TrimSpace(first)
		}
		if real := r.Header.Get("X-Real-IP"); real != "" {
			return strings.TrimSpace(real)
		}
	}

	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
