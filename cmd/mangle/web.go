package main

import (
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/web"
	"golang.org/x/crypto/acme/autocert"
)

// Server timeouts. The server faces the network directly, so these are what
// keep a slow or stuck client from holding a connection open.
const (
	readHeaderTimeout = 10 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 2 * time.Minute
	idleTimeout       = 2 * time.Minute
	shutdownTimeout   = 15 * time.Second
)

// webOptions are the flags the web command takes.
type webOptions struct {
	common     commonFlags
	listen     string
	listenHTTP string
	insecure   bool
	trustProxy bool
	metrics    string

	// restart, when set, restarts the server in place after a change to
	// its listener settings, rather than asking systemd.
	restart func()
}

// runWeb runs the web application, or one of its service hooks.
func runWeb(ctx context.Context, args []string) error {
	// The systemd unit calls the hooks as subcommands of "web".
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action, rest := args[0], args[1:]
		switch action {
		case "post-start":
			return runWebPostStart(ctx, rest)
		case "post-stop":
			return runWebPostStop(ctx, rest)
		default:
			return fmt.Errorf("unknown web action %q", action)
		}
	}

	opts, err := parseWebFlags("web", args)
	if err != nil {
		return err
	}

	a, err := opts.common.open(ctx)
	if err != nil {
		return err
	}
	defer a.Close()

	// The background work runs alongside the server: queued email,
	// webhooks and disconnections, and the scheduled checks, backups and
	// clean-up. It stops, and is waited for, before the database closes.
	stopTasks := startTasks(ctx, a)
	defer stopTasks()

	return serveWeb(ctx, a, opts)
}

// parseWebFlags reads the flags of the commands that serve the web
// application.
func parseWebFlags(name string, args []string) (webOptions, error) {
	var opts webOptions

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	opts.common.bind(fs, "mangle.log")
	fs.StringVar(&opts.listen, "listen", "",
		"address to serve HTTPS on (default: the configured HTTPS port); may also be a unix socket path")
	fs.StringVar(&opts.listenHTTP, "listen-http", "",
		"address to serve the HTTPS redirect on (default: the configured HTTP port); \"off\" to disable")
	fs.BoolVar(&opts.insecure, "insecure", false,
		"serve plain HTTP without TLS, for development")
	fs.BoolVar(&opts.trustProxy, "trust-proxy", false,
		"honour X-Forwarded-For and X-Forwarded-Proto, for running behind a reverse proxy")
	fs.StringVar(&opts.metrics, "metrics-listen", "",
		"address to serve Prometheus metrics on, such as 127.0.0.1:9100; off unless set, and not behind a sign in")

	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	return opts, nil
}

// startTasks runs the task worker until the returned function is called,
// which waits for it to finish. Jobs come from a queue in the database, so
// the OpenVPN hooks, which run as processes of their own, can add to it
// too, and a job interrupted by a restart runs again.
func startTasks(ctx context.Context, a *app.App) (stop func()) {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	worker := jobs.NewWorker(a.Store.Jobs, a.Log)
	app.RegisterTasks(a, worker)

	go func() {
		defer close(done)
		if err := worker.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.Log.Error("the task worker stopped", "err", err)
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

// serveWeb brings up the listeners and runs until the context is cancelled.
func serveWeb(ctx context.Context, a *app.App, opts webOptions) error {
	handler := web.New(a, web.Options{
		Insecure:   opts.insecure,
		TrustProxy: opts.trustProxy,
		Restart:    opts.restart,
	})

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	if opts.metrics != "" {
		metrics, err := startMetrics(ctx, a, opts.metrics)
		if err != nil {
			return err
		}
		defer metrics.Shutdown(context.WithoutCancel(ctx))
	}

	httpsPort := a.Config.Int(config.AppHTTPSPort, 443)
	address := orDefault(opts.listen, ":"+strconv.Itoa(httpsPort))

	listener, err := listenOn(address)
	if err != nil {
		return err
	}

	// Without TLS there is nothing to redirect to, so the plain listener
	// serves the application itself.
	if opts.insecure {
		a.Log.Warn("serving without TLS", "address", address)
		a.Log.Info("web application started", "address", address, "root", a.Paths.Root)
		return serve(ctx, server, listener, a)
	}

	loader, err := web.NewCertificateLoader(a.Paths.WebCertificate, a.Paths.WebPrivateKey, a.Log)
	if err != nil {
		return fmt.Errorf("%w (run \"mangle-vpn install\" to generate one)", err)
	}
	server.TLSConfig = loader.TLSConfig()

	// With Let's Encrypt on, its certificate is served, with the one on disk
	// as the fallback. The setting is read at start; changing it restarts
	// the service.
	var acmeManager *autocert.Manager
	if a.Config.Bool(config.AppLetsEncrypt, false) {
		hostname := a.Config.Get(config.AppHostname)
		acmeManager = web.NewACMEManager(a.Paths.ACMECache, hostname, a.Config.Get(config.AppACMEEmail))
		server.TLSConfig = loader.TLSConfigWithACME(acmeManager, a.Log)
		a.Log.Info("using Let's Encrypt", "hostname", hostname)
	}

	// Redirect to the port actually bound, which is the configured one
	// unless -listen overrode it.
	redirect, err := startHTTPRedirect(ctx, a, opts, boundPort(listener, httpsPort), acmeManager)
	if err != nil {
		listener.Close()
		return err
	}
	if redirect != nil {
		defer redirect.Shutdown(context.WithoutCancel(ctx))
	}

	a.Log.Info("web application started",
		"address", address, "root", a.Paths.Root, "tls", true)

	return serve(ctx, server, tls.NewListener(listener, server.TLSConfig), a)
}

// boundPort returns the TCP port a listener ended up on, falling back to
// fallback for a unix socket, which has no port.
func boundPort(listener net.Listener, fallback int) int {
	if addr, ok := listener.Addr().(*net.TCPAddr); ok {
		return addr.Port
	}
	return fallback
}

// startMetrics serves the Prometheus metrics on their own listener, apart
// from the application, so that they can be bound to localhost or a private
// network.
func startMetrics(ctx context.Context, a *app.App, address string) (*http.Server, error) {
	listener, err := listenOn(address)
	if err != nil {
		return nil, err
	}

	server := &http.Server{
		Handler:           web.MetricsHandler(a),
		ReadHeaderTimeout: readHeaderTimeout,
	}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.Log.Error("the metrics listener stopped", "err", err)
		}
	}()

	a.Log.Info("serving metrics", "address", address)
	return server, nil
}

// startHTTPRedirect brings up the plain HTTP listener that sends visitors to
// HTTPS, unless it has been switched off.
func startHTTPRedirect(ctx context.Context, a *app.App, opts webOptions, httpsPort int, acmeManager *autocert.Manager) (*http.Server, error) {
	if opts.listenHTTP == "off" {
		return nil, nil
	}

	address := orDefault(opts.listenHTTP, ":"+strconv.Itoa(a.Config.Int(config.AppHTTPPort, 80)))

	listener, err := listenOn(address)
	if err != nil {
		return nil, err
	}

	handler := web.RedirectToHTTPS(httpsPort)
	if acmeManager != nil {
		handler = web.ACMEChallenges(acmeManager, handler)
	}

	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.Log.Error("the HTTP redirect listener stopped", "err", err)
		}
	}()

	a.Log.Info("redirecting HTTP to HTTPS", "address", address, "https_port", httpsPort)
	return server, nil
}

// serve runs the server until the context is cancelled, then lets in-flight
// requests finish before returning.
func serve(ctx context.Context, server *http.Server, listener net.Listener, a *app.App) error {
	errs := make(chan error, 1)
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		errs <- err
	}()

	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
		a.Log.Info("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

// listenOn opens the listening socket named by addr, which is either a unix
// socket path or a host and port.
func listenOn(addr string) (net.Listener, error) {
	if !strings.HasPrefix(addr, "/") {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return nil, fmt.Errorf("listen on %s: %w", addr, err)
		}
		return listener, nil
	}

	// A socket left behind by a process that did not shut down cleanly
	// would otherwise block the bind.
	if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove stale socket %s: %w", addr, err)
	}

	listener, err := net.Listen("unix", addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", addr, err)
	}

	// A proxy in front of the application runs as its own user, so the
	// socket has to be writable by more than the owner.
	if err := os.Chmod(addr, 0o666); err != nil {
		listener.Close()
		return nil, fmt.Errorf("set permissions on %s: %w", addr, err)
	}
	return listener, nil
}

// orDefault returns value when it is set, and otherwise fallback.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

// runWebPostStart opens the web application's ports in the firewall.
func runWebPostStart(ctx context.Context, args []string) error {
	a, err := openApp(ctx, "web post-start", "mangle.log", args)
	if err != nil {
		return err
	}
	defer a.Close()
	return a.InstallWebFirewall(ctx)
}

// runWebPostStop closes the web application's ports again.
func runWebPostStop(ctx context.Context, args []string) error {
	a, err := openApp(ctx, "web post-stop", "mangle.log", args)
	if err != nil {
		return err
	}
	defer a.Close()
	return a.RemoveWebFirewall(ctx)
}
