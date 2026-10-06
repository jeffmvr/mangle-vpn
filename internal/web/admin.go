package web

import "net/http"

// registerAdminAPI adds the routes behind the administration pages.
func (s *Server) registerAdminAPI(mux *http.ServeMux) {
	// Every route below is wrapped, so access is enforced in one place
	// rather than being re-checked in each handler. Administrators may use
	// all of them. The help desk may use those marked staff: everything
	// that only looks, and the few changes that help someone back in.
	admin := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, s.requireAPIAdmin(handler))
	}
	staff := func(pattern string, handler http.HandlerFunc) {
		mux.Handle(pattern, s.requireAPIStaff(handler))
	}

	// Users.
	staff("GET /api/admin/users", s.adminListUsers)
	admin("POST /api/admin/users", s.adminInviteUsers)
	staff("GET /api/admin/users/{id}", s.adminGetUser)
	admin("PUT /api/admin/users/{id}", s.adminUpdateUser)
	admin("DELETE /api/admin/users/{id}", s.adminDeleteUser)
	staff("PUT /api/admin/users/{id}/mfa", s.adminResetUserMFA)
	staff("DELETE /api/admin/users/{id}/password", s.adminResetUserPassword)
	staff("DELETE /api/admin/users/{id}/lockout", s.adminUnlockUser)
	staff("GET /api/admin/users/{id}/devices", s.adminListUserDevices)
	staff("GET /api/admin/users/{id}/sessions", s.adminListUserSessions)

	// Groups.
	staff("GET /api/admin/groups", s.adminListGroups)
	admin("POST /api/admin/groups", s.adminCreateGroup)
	staff("GET /api/admin/groups/all", s.adminListAllGroups)
	staff("GET /api/admin/groups/{id}", s.adminGetGroup)
	admin("PUT /api/admin/groups/{id}", s.adminUpdateGroup)
	admin("DELETE /api/admin/groups/{id}", s.adminDeleteGroup)
	staff("GET /api/admin/groups/{id}/firewall", s.adminListGroupFirewall)
	staff("GET /api/admin/groups/{id}/users", s.adminListGroupUsers)

	// Firewall rules.
	admin("POST /api/admin/firewall", s.adminCreateFirewallRule)
	staff("GET /api/admin/firewall/{id}", s.adminGetFirewallRule)
	admin("PUT /api/admin/firewall/{id}", s.adminUpdateFirewallRule)
	admin("DELETE /api/admin/firewall/{id}", s.adminDeleteFirewallRule)

	// Devices, connected clients, and the audit log.
	staff("DELETE /api/admin/devices/{id}", s.adminDeleteDevice)
	admin("PUT /api/admin/devices/{id}", s.adminUpdateDevice)
	staff("GET /api/admin/clients", s.adminListClients)
	staff("DELETE /api/admin/clients/{id}", s.adminDisconnectClient)
	staff("GET /api/admin/events", s.adminListEvents)
	admin("GET /api/admin/logs/{name}", s.adminReadLog)
	staff("GET /api/admin/certificates", s.adminListCertificates)
	admin("POST /api/admin/certificates/openvpn/renew", s.adminRenewVPNCertificate)

	// The OpenVPN server itself.
	staff("GET /api/admin/openvpn", s.adminOpenVPNStatus)
	admin("POST /api/admin/openvpn/toggle", s.adminToggleOpenVPN)
	admin("POST /api/admin/openvpn/restart", s.adminRestartOpenVPN)

	// Settings.
	admin("GET /api/admin/settings/{section}", s.adminGetSettings)
	admin("PUT /api/admin/settings/{section}", s.adminUpdateSettings)
	admin("POST /api/admin/settings/mail/test", s.adminSendTestEmail)
	admin("POST /api/admin/settings/alerts/test", s.adminSendTestAlert)

	// Backups.
	admin("GET /api/admin/backups", s.adminListBackups)
	admin("POST /api/admin/backups", s.adminRunBackup)
	admin("GET /api/admin/backups/{name}", s.adminDownloadBackup)
}
