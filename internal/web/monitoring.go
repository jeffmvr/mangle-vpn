package web

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
)

// serveHealth answers uptime checks: 200 while the application can serve,
// 503 when it cannot. It needs no session and says nothing a stranger could
// use.
func (s *Server) serveHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := s.app.Healthy(r.Context()); err != nil {
		s.app.Log.Error("health check failed", "err", err)
		s.writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// adminListCertificates returns the certificates the application depends on
// and when each expires.
func (s *Server) adminListCertificates(w http.ResponseWriter, r *http.Request) {
	type certificateDTO struct {
		app.Certificate
		ExpiresSoon bool `json:"expires_soon"`
	}

	now := time.Now()
	out := []certificateDTO{}
	for _, c := range s.app.Certificates() {
		out = append(out, certificateDTO{Certificate: c, ExpiresSoon: c.ExpiresSoon(now)})
	}
	s.writeJSON(w, http.StatusOK, out)
}

// adminRenewVPNCertificate issues the OpenVPN server a new certificate. It
// takes effect when OpenVPN is next restarted.
func (s *Server) adminRenewVPNCertificate(w http.ResponseWriter, r *http.Request) {
	if err := s.app.RenewVPNCertificate(r.Context()); err != nil {
		s.app.Log.Error("failed to renew the OpenVPN server certificate", "err", err)
		s.writeJSON(w, http.StatusInternalServerError, detail{"The certificate could not be renewed."})
		return
	}
	s.audit(r, model.EventAdminOpenVPN, "Renewed the OpenVPN server certificate.")
	s.adminListCertificates(w, r)
}

// MetricsHandler returns the Prometheus metrics for the application. It is
// served on its own listener, which should only be reachable by the
// monitoring system: it is not behind a sign in.
func MetricsHandler(a *app.App) http.Handler {
	registry := prometheus.NewRegistry()
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		&appCollector{app: a},
	)
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{})
}

// appCollector reads the application's state at each scrape, so the numbers
// are current whichever process last changed them.
type appCollector struct {
	app *app.App
}

var (
	usersDesc = prometheus.NewDesc("mangle_users",
		"Users with an account.", nil, nil)
	lockedUsersDesc = prometheus.NewDesc("mangle_users_locked",
		"Accounts locked after too many failed sign in attempts.", nil, nil)
	devicesDesc = prometheus.NewDesc("mangle_devices",
		"Devices issued a profile.", nil, nil)
	clientsDesc = prometheus.NewDesc("mangle_clients_connected",
		"Devices connected to the VPN now.", nil, nil)
	openvpnUpDesc = prometheus.NewDesc("mangle_openvpn_up",
		"Whether the OpenVPN server is running (1) or not (0).", nil, nil)
	certificateExpiryDesc = prometheus.NewDesc("mangle_certificate_expiry_timestamp_seconds",
		"When a certificate expires, as a Unix time.", []string{"certificate"}, nil)
)

// Describe implements [prometheus.Collector].
func (c *appCollector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{usersDesc, lockedUsersDesc, devicesDesc,
		clientsDesc, openvpnUpDesc, certificateExpiryDesc} {
		ch <- d
	}
}

// Collect implements [prometheus.Collector].
func (c *appCollector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if stats, err := c.app.Store.Stats(ctx); err == nil {
		ch <- prometheus.MustNewConstMetric(usersDesc, prometheus.GaugeValue, float64(stats.Users))
		ch <- prometheus.MustNewConstMetric(lockedUsersDesc, prometheus.GaugeValue, float64(stats.LockedUsers))
		ch <- prometheus.MustNewConstMetric(devicesDesc, prometheus.GaugeValue, float64(stats.Devices))
		ch <- prometheus.MustNewConstMetric(clientsDesc, prometheus.GaugeValue, float64(stats.Clients))
	} else {
		c.app.Log.Error("failed to collect metrics", "err", err)
	}

	up := 0.0
	if openvpn.IsRunning(ctx) {
		up = 1
	}
	ch <- prometheus.MustNewConstMetric(openvpnUpDesc, prometheus.GaugeValue, up)

	for _, certificate := range c.app.Certificates() {
		ch <- prometheus.MustNewConstMetric(certificateExpiryDesc, prometheus.GaugeValue,
			float64(certificate.NotAfter.Unix()), certificate.Name)
	}
}
