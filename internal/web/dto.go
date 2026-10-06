package web

import (
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// The types below are the API's wire format. They are written out rather
// than derived from the domain models so that the shape the frontend reads
// stays fixed while the models are free to change.

// record is the identity and timestamps every resource carries.
type record struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// newRecord copies the identity and timestamps out of an entity.
func newRecord(base model.Base) record {
	return record{ID: base.ID, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt}
}

//
// Profile
//

// profileDTO is the signed in user's own account.
type profileDTO struct {
	record
	Devices     []deviceDTO     `json:"devices"`
	Email       string          `json:"email"`
	Group       profileGroupDTO `json:"group"`
	IsAdmin     bool            `json:"is_admin"`
	IsEnabled   bool            `json:"is_enabled"`
	LastLogin   *time.Time      `json:"last_login"`
	MFAEnabled  bool            `json:"mfa_enabled"`
	MFAEnforced *bool           `json:"mfa_enforced"`
	MFARequired bool            `json:"mfa_required"`
	Role        string          `json:"role"`

	// AdminReachable is false for a member of staff whose network the
	// administration pages are not open to, so the interface can leave
	// them out.
	AdminReachable bool `json:"admin_reachable"`
	// VPNPasswordRequired tells the frontend whether connecting asks for
	// the account password as well as the code, so it can say which goes
	// where.
	VPNPasswordRequired bool `json:"vpn_password_required"`
}

// profileGroupDTO is the group shown on a user's own profile, which tells
// them how many devices they may add.
type profileGroupDTO struct {
	record
	MaxDevices int    `json:"max_devices"`
	Name       string `json:"name"`
}

// deviceDTO is one of a user's devices.
type deviceDTO struct {
	record
	LastLogin *time.Time `json:"last_login"`
	Name      string     `json:"name"`
	OS        string     `json:"os"`
	StaticIP  string     `json:"static_ip"`

	// Connected says whether the device is connected now, and Traffic how
	// much data its recorded connections have moved, in bytes.
	Connected bool  `json:"connected"`
	Traffic   int64 `json:"traffic"`
}

// newProfile renders a user's own account.
func newProfile(user *model.User, devices []*model.Device) profileDTO {
	profile := profileDTO{
		record:      newRecord(user.Base),
		Devices:     newDevices(devices),
		Email:       user.Email,
		IsAdmin:     user.IsAdmin,
		IsEnabled:   user.IsEnabled,
		LastLogin:   user.LastLogin,
		MFAEnabled:  user.MFAEnabled,
		MFAEnforced: user.MFAEnforced,
		MFARequired: user.MFARequired(),
		Role:        roleOf(user),
	}

	if user.Group != nil {
		profile.Group = profileGroupDTO{
			record:     newRecord(user.Group.Base),
			MaxDevices: user.Group.MaxDevices,
			Name:       user.Group.Name,
		}
	}
	return profile
}

// newDevice renders one device.
func newDevice(device *model.Device) deviceDTO {
	return deviceDTO{
		record:    newRecord(device.Base),
		LastLogin: device.LastLogin,
		Name:      device.Name,
		OS:        device.OS,
		StaticIP:  device.StaticIP,
	}
}

// newDevices renders a list of devices, never as null.
func newDevices(devices []*model.Device) []deviceDTO {
	out := make([]deviceDTO, 0, len(devices))
	for _, device := range devices {
		out = append(out, newDevice(device))
	}
	return out
}

//
// Users
//

// userDTO is a user as an administrator sees them.
type userDTO struct {
	record
	Email       string        `json:"email"`
	Group       namedGroupDTO `json:"group"`
	GroupID     uuid.UUID     `json:"group_id"`
	IsAdmin     bool          `json:"is_admin"`
	IsEnabled   bool          `json:"is_enabled"`
	LastLogin   *time.Time    `json:"last_login"`
	MFAEnabled  bool          `json:"mfa_enabled"`
	MFAEnforced *bool         `json:"mfa_enforced"`
	Name        string        `json:"name"`
	Role        string        `json:"role"`

	// LockedUntil is when a lockout after too many failed sign ins ends,
	// or null when the account is not locked.
	LockedUntil *time.Time `json:"locked_until"`
}

// namedGroupDTO is a group reduced to its name, for nesting and for
// populating the group chooser.
type namedGroupDTO struct {
	record
	Name string `json:"name"`
}

// accountDTO is a user reduced to the fields a listing shows.
type accountDTO struct {
	record
	Email     string     `json:"email"`
	LastLogin *time.Time `json:"last_login"`
	Name      string     `json:"name"`
}

// identityDTO is a user reduced to their address, for nesting inside a
// client or an event.
type identityDTO struct {
	record
	Email string `json:"email"`
}

// newUser renders a user for an administrator.
func newUser(user *model.User) userDTO {
	dto := userDTO{
		record:      newRecord(user.Base),
		Email:       user.Email,
		GroupID:     user.GroupID,
		IsAdmin:     user.IsAdmin,
		IsEnabled:   user.IsEnabled,
		LastLogin:   user.LastLogin,
		MFAEnabled:  user.MFAEnabled,
		MFAEnforced: user.MFAEnforced,
		Name:        user.Name,
		Role:        roleOf(user),
	}
	if user.IsLocked(time.Now()) {
		dto.LockedUntil = user.LockedUntil
	}
	if user.Group != nil {
		dto.Group = newNamedGroup(user.Group)
	}
	return dto
}

// newNamedGroup renders a group as a name and identity.
func newNamedGroup(group *model.Group) namedGroupDTO {
	return namedGroupDTO{record: newRecord(group.Base), Name: group.Name}
}

// newAccount renders a user as a listing entry.
func newAccount(user *model.User) accountDTO {
	return accountDTO{
		record:    newRecord(user.Base),
		Email:     user.Email,
		LastLogin: user.LastLogin,
		Name:      user.Name,
	}
}

// newIdentity renders a user as an address.
func newIdentity(user *model.User) identityDTO {
	if user == nil {
		return identityDTO{}
	}
	return identityDTO{record: newRecord(user.Base), Email: user.Email}
}

//
// Groups
//

// groupDTO is a group as an administrator sees it.
type groupDTO struct {
	record

	// The number of members and firewall rules, given in a listing.
	MemberCount *int `json:"member_count,omitempty"`
	RuleCount   *int `json:"rule_count,omitempty"`

	Description    string `json:"description"`
	IsEnabled      bool   `json:"is_enabled"`
	MaxDevices     int    `json:"max_devices"`
	MFAEnforced    bool   `json:"mfa_enforced"`
	Name           string `json:"name"`
	Routes         string `json:"routes"`
	Nameservers    string `json:"nameservers"`
	DeviceIdleDays int    `json:"device_idle_days"`
}

// newGroup renders a group.
func newGroup(group *model.Group) groupDTO {
	return groupDTO{
		record:         newRecord(group.Base),
		Description:    group.Description,
		IsEnabled:      group.IsEnabled,
		MaxDevices:     group.MaxDevices,
		MFAEnforced:    group.MFAEnforced,
		Name:           group.Name,
		Routes:         group.Routes,
		Nameservers:    group.Nameservers,
		DeviceIdleDays: group.DeviceIdleDays,
	}
}

//
// Firewall rules
//

// firewallRuleDTO is one rule in a group's chain.
type firewallRuleDTO struct {
	record
	Action      string    `json:"action"`
	Destination string    `json:"destination"`
	GroupID     uuid.UUID `json:"group_id"`
	IsEnabled   bool      `json:"is_enabled"`
	Port        string    `json:"port"`
	Protocol    string    `json:"protocol"`
}

// newFirewallRule renders a firewall rule.
func newFirewallRule(rule *model.FirewallRule) firewallRuleDTO {
	return firewallRuleDTO{
		record:      newRecord(rule.Base),
		Action:      rule.Action,
		Destination: rule.Destination,
		GroupID:     rule.GroupID,
		IsEnabled:   rule.IsEnabled,
		Port:        rule.Port,
		Protocol:    rule.Protocol,
	}
}

//
// Clients
//

// clientDTO is a currently connected VPN client.
type clientDTO struct {
	record
	Device    clientDeviceDTO `json:"device"`
	Duration  int             `json:"duration"`
	Platform  string          `json:"platform"`
	RemoteIP  string          `json:"remote_ip"`
	VirtualIP string          `json:"virtual_ip"`

	// The data moved so far, from the server's side: received from the
	// device, and sent to it.
	BytesReceived int64 `json:"bytes_received"`
	BytesSent     int64 `json:"bytes_sent"`
}

// clientDeviceDTO is the device behind a connection, with its owner.
type clientDeviceDTO struct {
	record
	LastLogin *time.Time  `json:"last_login"`
	Name      string      `json:"name"`
	User      identityDTO `json:"user"`
}

// newClient renders a connected client.
func newClient(client *model.Client) clientDTO {
	dto := clientDTO{
		record:    newRecord(client.Base),
		Duration:  client.Duration(),
		Platform:  client.Platform,
		RemoteIP:  client.RemoteIP,
		VirtualIP: client.VirtualIP,
	}

	if client.Device != nil {
		dto.Device = clientDeviceDTO{
			record:    newRecord(client.Device.Base),
			LastLogin: client.Device.LastLogin,
			Name:      client.Device.Name,
			User:      newIdentity(client.Device.User),
		}
	}
	return dto
}

//
// Connection history
//

// sessionDTO is one finished VPN connection.
type sessionDTO struct {
	ID            uuid.UUID `json:"id"`
	DeviceID      uuid.UUID `json:"device_id"`
	DeviceName    string    `json:"device_name"`
	StartedAt     time.Time `json:"started_at"`
	EndedAt       time.Time `json:"ended_at"`
	Duration      int       `json:"duration"`
	RemoteIP      string    `json:"remote_ip"`
	VirtualIP     string    `json:"virtual_ip"`
	BytesReceived int64     `json:"bytes_received"`
	BytesSent     int64     `json:"bytes_sent"`
}

// newSession renders a finished connection.
func newSession(session *model.VPNSession) sessionDTO {
	return sessionDTO{
		ID:            session.ID,
		DeviceID:      session.DeviceID,
		DeviceName:    session.DeviceName,
		StartedAt:     session.StartedAt,
		EndedAt:       session.EndedAt,
		Duration:      int(session.Duration().Seconds()),
		RemoteIP:      session.RemoteIP,
		VirtualIP:     session.VirtualIP,
		BytesReceived: session.BytesReceived,
		BytesSent:     session.BytesSent,
	}
}

//
// Events
//

// eventDTO is one entry in the audit log.
type eventDTO struct {
	record
	Detail string      `json:"detail"`
	Name   string      `json:"name"`
	User   identityDTO `json:"user"`
}

// newEvent renders an audit log entry.
func newEvent(event *model.Event) eventDTO {
	return eventDTO{
		record: newRecord(event.Base),
		Detail: event.Detail,
		Name:   event.Name,
		User:   newIdentity(event.User),
	}
}
