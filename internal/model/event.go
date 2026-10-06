package model

import "github.com/jeffmvr/mangle-vpn/internal/uuid"

// Event names recorded in the audit log. The part before the dot is the
// kind of event, which the audit log can be filtered by.
const (
	EventWebLogin      = "web.login"
	EventWebError      = "web.error"
	EventVPNConnect    = "vpn.connect"
	EventVPNDisconnect = "vpn.disconnect"
	EventVPNError      = "vpn.error"
	EventDeviceImport  = "device.import"
	EventDeviceCreate  = "device.create"
	EventDeviceDelete  = "device.delete"

	// EventDeviceRetire is a device removed for going unused longer than
	// its owner's group allows.
	EventDeviceRetire = "device.retire"

	// Changes people make to their own account, and an account made for
	// someone signing in through single sign-on for the first time.
	EventAccountPassword = "account.password"
	EventAccountCreate   = "account.create"

	// Changes administrators make. These are recorded against the
	// administrator who made them, and the detail names what was changed.
	EventAdminUserInvite   = "admin.user.invite"
	EventAdminUserUpdate   = "admin.user.update"
	EventAdminUserDelete   = "admin.user.delete"
	EventAdminUserPassword = "admin.user.password"
	EventAdminUserMFA      = "admin.user.mfa"
	EventAdminUserUnlock   = "admin.user.unlock"
	EventAdminGroupCreate  = "admin.group.create"
	EventAdminGroupUpdate  = "admin.group.update"
	EventAdminGroupDelete  = "admin.group.delete"
	EventAdminRuleCreate   = "admin.firewall.create"
	EventAdminRuleUpdate   = "admin.firewall.update"
	EventAdminRuleDelete   = "admin.firewall.delete"
	EventAdminDeviceDelete = "admin.device.delete"
	EventAdminDeviceUpdate = "admin.device.update"
	EventAdminClientKill   = "admin.client.disconnect"
	EventAdminSettings     = "admin.settings"
	EventAdminOpenVPN      = "admin.openvpn"
	EventAdminBackup       = "admin.backup"
)

// EventKinds are the kinds of event the audit log can be filtered by, each
// the prefix its event names start with.
var EventKinds = []string{"web", "vpn", "device", "account", "admin"}

// Event is one entry in the audit log.
type Event struct {
	Base

	Detail string
	Name   string
	UserID uuid.UUID

	User *User
}

// Setting is a single application setting, stored as text.
type Setting struct {
	Base

	Name  string
	Value string
}
