// Package web serves the application: the server rendered sign in and setup
// pages, the single page frontend, and the JSON API behind it.
package web

import (
	"io/fs"
	"net/http"
	"strings"
	"sync"

	"github.com/alexedwards/scs/v2"
	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/webui"
)

// Server holds the web application's dependencies and its routing table.
type Server struct {
	app      *app.App
	sessions *scs.SessionManager
	handler  http.Handler

	// secureCookies marks cookies as HTTPS-only. It is on whenever the
	// server is reached over TLS, which is everything but a development
	// run over plain HTTP.
	secureCookies bool

	// trustProxy honours the X-Forwarded-* headers. It is off by default,
	// because without a proxy in front those headers are set by the
	// client and would let a caller choose the address recorded against
	// them in the audit log.
	trustProxy bool

	// attempts limits sign in attempts per client address.
	attempts *attemptLimiter

	// challenges holds two-factor challenges issued to OpenVPN Connect.
	challenges *restChallenges

	// oidc holds the OpenID Connect provider discovered from the settings.
	oidc oidcCache

	// installing serialises first run setup, so two submissions racing
	// each other cannot both create an administrator.
	installing sync.Mutex

	// restart brings the web server back up with new listener settings;
	// nil asks systemd.
	restart func()
}

// Options configure the server.
type Options struct {
	// Insecure serves cookies without the Secure attribute, for reaching
	// the server directly over HTTP in development.
	Insecure bool

	// TrustProxy honours the X-Forwarded-For, X-Real-IP, and
	// X-Forwarded-Proto headers. Turn it on only when a reverse proxy in
	// front of this server is known to set them.
	TrustProxy bool

	// Restart, when set, is how the server is restarted after a change to
	// its ports or certificate source. Unset, systemd restarts the
	// mangle-web service.
	Restart func()
}

// New returns a server for the application.
func New(a *app.App, opts Options) *Server {
	// Behind a TLS terminating proxy the server itself speaks plain HTTP,
	// which is what -insecure is for, but browsers still reach it over
	// HTTPS, so cookies keep the Secure flag.
	secure := !opts.Insecure || opts.TrustProxy

	// The session lengths are read once; changing them restarts the web
	// service, as a port change does.
	idle, lifetime := a.SessionTimeouts()

	s := &Server{
		app:           a,
		sessions:      newSessionManager(a.Store.Sessions(), secure, idle, lifetime),
		secureCookies: secure,
		trustProxy:    opts.TrustProxy,
		attempts:      newAttemptLimiter(),
		challenges:    newRESTChallenges(),
		restart:       opts.Restart,
	}
	s.handler = s.routes()
	return s
}

// ServeHTTP implements [http.Handler].
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// routes builds the routing table and wraps it in the middleware every
// request passes through.
func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	s.registerPages(mux)
	s.registerAPI(mux)
	s.registerAdminAPI(mux)

	// The frontend's assets, from the build compiled into the binary.
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticAssets()))

	// canonicalPath turns "/static/" into "/static", which the mux would
	// redirect back to "/static/" forever. There is nothing to list there.
	mux.Handle("GET /static", http.NotFoundHandler())

	// The order below is the order a request travels through: recover from
	// panics, log the outcome, add the security headers, settle on a
	// canonical path, load the session, refresh the settings this request
	// will read, and only then check the CSRF token.
	var handler http.Handler = mux
	handler = s.protectCSRF(handler)
	handler = s.reloadConfig(handler)
	handler = s.sessions.LoadAndSave(handler)
	handler = canonicalPath(handler)
	handler = s.secureHeaders(handler)
	handler = s.logRequests(handler)
	handler = s.recoverPanics(handler)

	return handler
}

// registerPages adds the server rendered pages.
func (s *Server) registerPages(mux *http.ServeMux) {
	// Installation. These are the only pages reachable before setup.
	mux.HandleFunc("GET /install", s.showSetupStart)
	// What setup did, for the administrator it just signed in, before they
	// set up two-factor authentication.
	mux.Handle("GET /install/done", s.requirePage(accessNeedsMFASetup, http.HandlerFunc(s.showSetupDone)))
	mux.HandleFunc("GET /install/{step}", s.showSetupStep)
	mux.HandleFunc("POST /install/{step}", s.processSetupStep)

	// Authentication.
	mux.Handle("GET /login", s.requireInstalled(http.HandlerFunc(s.showLogin)))
	mux.Handle("POST /login/process", s.requireInstalled(http.HandlerFunc(s.processLogin)))
	mux.Handle("GET /login/sso", s.requireInstalled(http.HandlerFunc(s.showOAuth2Login)))
	// The address single sign-on started at when Google was the only
	// provider, kept for bookmarks.
	mux.Handle("GET /login/google", s.requireInstalled(http.HandlerFunc(s.showOAuth2Login)))
	mux.Handle("GET /oauth", s.requireInstalled(http.HandlerFunc(s.processOAuth2)))
	mux.Handle("POST /logout", s.requireInstalled(http.HandlerFunc(s.processLogout)))

	// The organization's logo, which the sign in page shows.
	mux.HandleFunc("GET /logo", s.serveLogo)

	// Uptime checks. No session, and it works before setup too.
	mux.HandleFunc("GET /healthz", s.serveHealth)

	// OpenVPN Connect's "Import Profile > URL", which signs in itself.
	mux.Handle("GET /rest/GetUserlogin", s.requireInstalled(http.HandlerFunc(s.restGetUserlogin)))
	mux.Handle("GET /rest/GetAutologin", s.requireInstalled(http.HandlerFunc(s.restGetAutologin)))

	// OpenVPN Connect fetching a profile through an import link. The link
	// carries its own single-use credential, so there is no session check.
	mux.Handle("GET /import/{token}", s.requireInstalled(http.HandlerFunc(s.serveImport)))

	// Two-factor authentication. These need credentials but not, by
	// definition, a confirmed two-factor code.
	mux.Handle("GET /mfa", s.requirePage(accessNeedsMFASetup, http.HandlerFunc(s.showMFAConfirm)))
	mux.Handle("GET /mfa/setup", s.requirePage(accessNeedsMFASetup, http.HandlerFunc(s.showMFASetup)))
	mux.Handle("POST /mfa/process", s.requirePage(accessNeedsMFASetup, http.HandlerFunc(s.processMFA)))

	// Choosing a password through an e-mailed link, and asking for one.
	// The link is the credential, so neither needs a session.
	mux.Handle("GET /password/set", s.requireInstalled(http.HandlerFunc(s.showSetPassword)))
	mux.Handle("POST /password/set/process", s.requireInstalled(http.HandlerFunc(s.processSetPassword)))
	mux.Handle("GET /password/forgot", s.requireInstalled(http.HandlerFunc(s.showForgotPassword)))
	mux.Handle("POST /password/forgot/process", s.requireInstalled(http.HandlerFunc(s.processForgotPassword)))

	// Password reset, and the application itself.
	mux.Handle("GET /password", s.requirePage(accessNeedsNewPassword, http.HandlerFunc(s.showPasswordReset)))
	mux.Handle("POST /password/process", s.requirePage(accessNeedsNewPassword, http.HandlerFunc(s.processPasswordReset)))
	mux.Handle("GET /{$}", s.requirePage(accessFull, http.HandlerFunc(s.showApp)))
}

// canonicalPath removes a trailing slash from the request path so that a
// route need only be registered once.
//
// The frontend calls some endpoints with a trailing slash and others
// without, following the conventions of the framework it was written
// against.
func canonicalPath(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if path := r.URL.Path; len(path) > 1 && strings.HasSuffix(path, "/") {
			r.URL.Path = strings.TrimRight(path, "/")
		}
		next.ServeHTTP(w, r)
	})
}

// staticAssets serves the frontend's static directory.
//
// Bundles are named after a hash of their content, so a browser may keep
// them for good: a new build changes the names index.html refers to. The
// few files without a hash are revalidated on every use instead.
func staticAssets() http.Handler {
	assets, err := fs.Sub(webui.FS(), "static")
	if err != nil {
		panic(err)
	}
	files := http.FileServerFS(assets)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if webui.Immutable(r.URL.Path) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}
