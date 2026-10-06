package web

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// adminListClients returns a page of currently connected clients.
func (s *Server) adminListClients(w http.ResponseWriter, r *http.Request) {
	q := listQuery(r)

	result, err := s.app.Store.Clients.List(r.Context(), q)
	if s.handleStoreError(w, err, "failed to list clients") {
		return
	}

	// Traffic comes from the server's status file, which it rewrites
	// every few seconds; a client not in it yet shows none.
	traffic := openvpn.ReadStatus(s.app.Paths.OpenVPNStatusLog)
	writeList(s, w, r, q, result, func(client *model.Client) clientDTO {
		dto := newClient(client)
		dto.BytesReceived = traffic[client.CommonName].BytesReceived
		dto.BytesSent = traffic[client.CommonName].BytesSent
		return dto
	})
}

// adminDisconnectClient drops a client's VPN connection.
func (s *Server) adminDisconnectClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}

	client, err := s.app.Store.Clients.Get(ctx, id)
	if s.handleStoreError(w, err, "failed to load a client") {
		return
	}

	if err := s.app.DisconnectClient(ctx, client); err != nil {
		s.serverError(w, "failed to disconnect a client", err)
		return
	}

	s.audit(r, model.EventAdminClientKill, "Disconnected %s's device %s.", client.User().Email, client.Device.Name)
	s.noContent(w)
}

// adminListEvents returns a page of the audit log, optionally narrowed to
// one kind of event (?kind=admin) or the events of one user (?user=<id>).
func (s *Server) adminListEvents(w http.ResponseWriter, r *http.Request) {
	q := listQuery(r)

	var filter store.EventFilter
	if kind := r.URL.Query().Get("kind"); kind != "" {
		if !slices.Contains(model.EventKinds, kind) {
			s.invalidField(w, "kind", "Unknown kind of event.")
			return
		}
		filter.Kind = kind
	}
	if user := r.URL.Query().Get("user"); user != "" {
		id, err := uuid.Parse(user)
		if err != nil {
			s.invalidField(w, "user", "Not a user ID.")
			return
		}
		filter.UserID = id
	}

	result, err := s.app.Store.Events.List(r.Context(), q, filter)
	if s.handleStoreError(w, err, "failed to list events") {
		return
	}
	writeList(s, w, r, q, result, newEvent)
}

// openvpnStatusDTO reports the state of the OpenVPN server.
type openvpnStatusDTO struct {
	RestartPending bool `json:"restart_pending"`
	Status         bool `json:"status"`

	// Offload says whether data channel offload is in use, and is absent
	// while there is no tunnel to tell from.
	Offload *bool `json:"offload,omitempty"`
}

// settleDelay gives systemd a moment to act before the new state is read
// back, so the response reflects the change that was just asked for.
const settleDelay = 500 * time.Millisecond

// adminOpenVPNStatus reports whether the OpenVPN server is running and
// whether its configuration has changed since it last started.
func (s *Server) adminOpenVPNStatus(w http.ResponseWriter, r *http.Request) {
	s.writeOpenVPNStatus(w, r)
}

// adminToggleOpenVPN starts the OpenVPN server if it is stopped, and stops
// it if it is running.
func (s *Server) adminToggleOpenVPN(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// A stop someone asked for is not one to alert about.
	if openvpn.IsRunning(ctx) {
		s.markVPNStoppedByAdmin(ctx, true)
		openvpn.Stop(ctx)
		s.app.Log.Info("OpenVPN stopped", "by", currentUser(r).Email)
		s.audit(r, model.EventAdminOpenVPN, "Stopped OpenVPN.")
	} else {
		s.markVPNStoppedByAdmin(ctx, false)
		openvpn.Start(ctx)
		s.app.Log.Info("OpenVPN started", "by", currentUser(r).Email)
		s.audit(r, model.EventAdminOpenVPN, "Started OpenVPN.")
	}

	time.Sleep(settleDelay)
	s.writeOpenVPNStatus(w, r)
}

// adminRestartOpenVPN restarts the OpenVPN server, which is how a pending
// configuration change is applied.
func (s *Server) adminRestartOpenVPN(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	s.markVPNStoppedByAdmin(ctx, false)
	openvpn.Restart(ctx)
	s.app.Log.Info("OpenVPN restarted", "by", currentUser(r).Email)
	s.audit(r, model.EventAdminOpenVPN, "Restarted OpenVPN.")

	time.Sleep(settleDelay)
	s.writeOpenVPNStatus(w, r)
}

// markVPNStoppedByAdmin records whether OpenVPN is down because someone
// asked, which the health check reads before alerting.
func (s *Server) markVPNStoppedByAdmin(ctx context.Context, stopped bool) {
	if err := s.app.Config.SetBool(ctx, config.VPNStoppedByAdmin, stopped); err != nil {
		s.app.Log.Error("failed to record why OpenVPN stopped", "err", err)
	}
}

// writeOpenVPNStatus sends the current state of the OpenVPN server.
func (s *Server) writeOpenVPNStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The restart flag is set by whichever process changed the settings, so
	// re-read it rather than trusting this process's cache.
	if err := s.app.Config.Reload(ctx); err != nil {
		s.app.Log.Error("failed to reload settings", "err", err)
	}

	status := openvpnStatusDTO{
		RestartPending: s.app.Config.Bool(config.VPNRestartPending, false),
		Status:         openvpn.IsRunning(ctx),
	}
	if on, known := openvpn.Offload(); known && status.Status {
		status.Offload = &on
	}
	s.writeJSON(w, http.StatusOK, status)
}
