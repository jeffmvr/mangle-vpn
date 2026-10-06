package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
	"github.com/jeffmvr/mangle-vpn/internal/version"
)

// registerAPI adds the endpoints the frontend uses on a user's own behalf.
func (s *Server) registerAPI(mux *http.ServeMux) {
	user := s.requireAPIUser

	// Information about the running application, for the interface. The
	// server rendered sign in pages read the organization name themselves,
	// so this needs a sign in too: the exact version is worth something to
	// someone looking for a vulnerable release.
	mux.Handle("GET /api/info", user(http.HandlerFunc(s.apiInfo)))

	mux.Handle("GET /api/profile", user(http.HandlerFunc(s.apiProfile)))
	mux.Handle("POST /api/devices", user(http.HandlerFunc(s.apiCreateDevice)))
	mux.Handle("GET /api/devices/{id}", user(http.HandlerFunc(s.apiDownloadDevice)))
	mux.Handle("POST /api/devices/{id}/import-link", user(http.HandlerFunc(s.apiCreateImportLink)))
	mux.Handle("DELETE /api/devices/{id}", user(http.HandlerFunc(s.apiDeleteDevice)))
}

// infoDTO describes the running application.
//
// This is the one endpoint readable without signing in, because the sign in
// page shows the organization name, so it carries nothing else.
type infoDTO struct {
	Organization      string `json:"app_organization"`
	Version           string `json:"app_version"`
	LogoURL           string `json:"logo_url"`
	VPNRestartPending bool   `json:"vpn_restart_pending"`
}

// apiInfo returns general information about the application.
func (s *Server) apiInfo(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, infoDTO{
		Organization:      s.app.Config.Organization(),
		Version:           version.String(),
		LogoURL:           s.logoURL(),
		VPNRestartPending: s.app.Config.Bool(config.VPNRestartPending, false),
	})
}

// apiProfile returns the signed in user's own account.
func (s *Server) apiProfile(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)

	devices, err := s.app.Store.Devices.ByUser(r.Context(), user.ID)
	if s.handleStoreError(w, err, "failed to load the user's devices") {
		return
	}

	profile := newProfile(user, devices)
	s.addDeviceStatus(r, user.ID, profile.Devices)
	profile.VPNPasswordRequired = s.app.VPNRequiresPassword()
	profile.AdminReachable = user.IsStaff() && s.app.AdminAllowedFrom(s.clientIP(r))
	s.writeJSON(w, http.StatusOK, profile)
}

// createDeviceRequest is the body of a device creation request.
type createDeviceRequest struct {
	Name string `json:"name"`
	OS   string `json:"os"`
}

// apiCreateDevice registers a new device for the signed in user.
func (s *Server) apiCreateDevice(w http.ResponseWriter, r *http.Request) {
	var body createDeviceRequest
	if !s.decodeJSON(w, r, &body) {
		return
	}

	errs := fieldErrors{}

	name := app.CleanDeviceName(body.Name)
	switch {
	case name == "":
		errs.add("name", "This field is required.")
	case len(name) > deviceNameLimit:
		errs.add("name", fmt.Sprintf("Ensure this field has no more than %d characters.", deviceNameLimit))
	}

	if !openvpn.IsSupportedOS(body.OS) {
		errs.add("os", "Unknown operating system.")
	}
	if !errs.empty() {
		s.invalid(w, errs)
		return
	}

	device, err := s.app.CreateDevice(r.Context(), currentUser(r), name, body.OS)
	if errors.Is(err, app.ErrDeviceLimitReached) {
		s.invalidField(w, "name", "You cannot create any more devices.")
		return
	}
	if errors.Is(err, app.ErrDuplicateDeviceName) {
		s.invalidField(w, "name", "You already have a device with this name.")
		return
	}
	if s.handleStoreError(w, err, "failed to create a device") {
		return
	}

	s.audit(r, model.EventDeviceCreate, "Added device %s.", device.Name)
	s.writeJSON(w, http.StatusCreated, newDevice(device))
}

// deviceNameLimit is the longest device name the database column holds.
const deviceNameLimit = 32

// apiDownloadDevice issues the device's certificate and returns its OpenVPN
// configuration as a download.
//
// This is the only time the device's private key leaves the server, so it is
// allowed exactly once and only just after the device was created.
func (s *Server) apiDownloadDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := currentUser(r)

	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}

	device, err := s.app.Store.Devices.Get(ctx, id)
	if s.handleStoreError(w, err, "failed to load a device") {
		return
	}
	if device.UserID != user.ID {
		s.notFound(w)
		return
	}
	if err := app.CheckDownload(device); err != nil {
		s.refuseDownload(w, err)
		return
	}

	operatingSystem := r.URL.Query().Get("os")
	if !openvpn.IsSupportedOS(operatingSystem) {
		s.invalidField(w, "os", "Unknown operating system.")
		return
	}

	s.serveDeviceProfile(w, r, device, operatingSystem)
}

// serveDeviceProfile issues the device's certificate and writes its OpenVPN
// profile as the response. Issuing claims the device's one download, so a
// second request, by either route, is refused.
func (s *Server) serveDeviceProfile(w http.ResponseWriter, r *http.Request, device *model.Device, operatingSystem string) {
	conf, err := s.deviceProfile(r.Context(), device, operatingSystem)
	if errors.Is(err, app.ErrAlreadyDownloaded) {
		s.refuseDownload(w, err)
		return
	}
	if err != nil {
		s.serverError(w, "failed to build a device profile", err)
		return
	}
	s.writeProfile(w, device, conf)
}

// deviceProfile issues the device's certificate and renders its profile.
func (s *Server) deviceProfile(ctx context.Context, device *model.Device, operatingSystem string) (string, error) {
	kp, err := s.app.IssueDeviceKeyPair(ctx, device)
	if err != nil {
		return "", err
	}

	certificate, privateKey := kp.PEM()
	hostname := s.app.Config.Get(config.VPNHostname)
	email := ""
	if device.User != nil {
		email = device.User.Email
	}

	return openvpn.ClientConfig{
		CACertificate: s.app.Config.Get(config.CACertificate),
		Certificate:   certificate,
		FriendlyName:  s.app.Config.Organization() + " VPN - " + device.Name,
		Hostname:      hostname,
		IdleTimeout:   s.app.Config.Int(config.VPNIdleMinutes, 60) * 60,
		OS:            operatingSystem,
		Port:          s.app.Config.Int(config.VPNPort, 1194),
		PrivateKey:    privateKey,
		ProfileName:   email + "@" + hostname,
		Protocol:      s.app.Config.GetOr(config.VPNProtocol, "udp"),
		TLSAuthKey:    s.app.Config.Get(config.VPNTLSAuthKey),

		StaticChallenge: s.app.VPNChallengePrompt(),
		MSSFix:          s.app.Config.Int(config.VPNMSSFix, 0),
	}.Render()
}

// writeProfile sends a rendered profile as a download.
func (s *Server) writeProfile(w http.ResponseWriter, device *model.Device, conf string) {
	filename := fmt.Sprintf("%s - %s.ovpn", s.app.Config.Organization(), device.Name)

	// The profile holds the device's private key: nothing may keep a copy.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/x-openvpn-profile")
	w.Header().Set("Content-Length", fmt.Sprint(len(conf)))
	w.Header().Set("Content-Disposition", contentDisposition(filename))
	w.Write([]byte(conf))
}

// refuseDownload explains why a device's configuration cannot be fetched.
func (s *Server) refuseDownload(w http.ResponseWriter, err error) {
	message := "This device's profile has already been downloaded. " +
		"If you no longer have it, revoke the device and add it again."
	if errors.Is(err, app.ErrDownloadWindowOver) {
		message = "The time to download this device's profile has passed. " +
			"Revoke the device and add it again."
	}
	s.writeJSON(w, http.StatusBadRequest, detail{message})
}

// importLinkDTO is the link that hands a device's profile to OpenVPN Connect.
type importLinkDTO struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
}

// apiCreateImportLink returns an openvpn://import-profile/ link for one of
// the signed in user's new devices. Opening it starts OpenVPN Connect, which
// fetches the profile from the link itself, once.
func (s *Server) apiCreateImportLink(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}

	device, err := s.app.Store.Devices.Get(ctx, id)
	if s.handleStoreError(w, err, "failed to load a device") {
		return
	}
	if device.UserID != currentUser(r).ID {
		s.notFound(w)
		return
	}

	token, err := s.app.CreateImportToken(ctx, device)
	if errors.Is(err, app.ErrAlreadyDownloaded) || errors.Is(err, app.ErrDownloadWindowOver) {
		s.refuseDownload(w, err)
		return
	}
	if err != nil {
		s.serverError(w, "failed to create an import link", err)
		return
	}

	// The link names the address this browser reached the server on, which
	// is the one OpenVPN Connect on the same device can reach it on too.
	profileURL := s.absoluteURL(r, &url.URL{Path: "/import/" + token})
	s.writeJSON(w, http.StatusCreated, importLinkDTO{
		URL:       "openvpn://import-profile/" + profileURL,
		ExpiresAt: device.CreatedAt.Add(app.DeviceDownloadWindow).UTC(),
	})
}

// serveImport answers OpenVPN Connect's fetch of an import link. Connect
// sends no session, so the single-use code in the path is the only
// credential, and an unknown, used, or lapsed one gets nothing.
func (s *Server) serveImport(w http.ResponseWriter, r *http.Request) {
	device, err := s.app.RedeemImportToken(r.Context(), r.PathValue("token"))
	if errors.Is(err, store.ErrNotFound) {
		http.Error(w, "This import link has expired or has already been used.", http.StatusNotFound)
		return
	}
	if err != nil {
		s.serverError(w, "failed to redeem an import link", err)
		return
	}

	operatingSystem := device.OS
	if !openvpn.IsSupportedOS(operatingSystem) {
		operatingSystem = "linux"
	}
	s.serveDeviceProfile(w, r, device, operatingSystem)
}

// apiDeleteDevice removes one of the signed in user's devices.
func (s *Server) apiDeleteDevice(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	id, ok := s.pathID(w, r, "id")
	if !ok {
		return
	}

	device, err := s.app.Store.Devices.Get(ctx, id)
	if s.handleStoreError(w, err, "failed to load a device") {
		return
	}
	if device.UserID != currentUser(r).ID {
		s.notFound(w)
		return
	}

	if err := s.app.DeleteDevice(ctx, device); err != nil {
		s.serverError(w, "failed to delete a device", err)
		return
	}

	s.audit(r, model.EventDeviceDelete, "Removed device %s.", device.Name)
	s.noContent(w)
}

// addDeviceStatus fills in whether each of a user's devices is connected
// now and how much data it has moved. A failure leaves them blank rather
// than failing the page.
func (s *Server) addDeviceStatus(r *http.Request, userID uuid.UUID, devices []deviceDTO) {
	ctx := r.Context()
	clients, err := s.app.Store.Clients.ByUser(ctx, userID)
	if err != nil {
		s.app.Log.Error("failed to look up a user's connections", "err", err)
	}
	traffic, err := s.app.Store.VPNSessions.TrafficByDevice(ctx, userID)
	if err != nil {
		s.app.Log.Error("failed to look up a user's data used", "err", err)
	}

	connected := map[uuid.UUID]bool{}
	for _, client := range clients {
		connected[client.DeviceID] = true
	}
	for i := range devices {
		devices[i].Connected = connected[devices[i].ID]
		devices[i].Traffic = traffic[devices[i].ID]
	}
}
