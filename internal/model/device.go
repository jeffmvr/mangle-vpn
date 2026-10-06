package model

import (
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// Device is one machine a user connects to the VPN with. Each device holds
// its own client certificate.
//
// OS is the operating system the device's profile was generated for, one of
// the values openvpn.IsSupportedOS accepts, or empty for a device created
// before it was recorded.
type Device struct {
	Base

	Fingerprint string
	LastLogin   *time.Time
	Name        string
	OS          string
	Serial      string
	UserID      uuid.UUID

	// StaticIP is the fixed VPN address the device always gets, or empty
	// for one from the pool. It is written only by the store's
	// SetStaticIP, which keeps it unique.
	StaticIP string

	User *User
}

// CommonName returns the certificate subject the device presents to the
// OpenVPN server.
func (d *Device) CommonName() string {
	email := ""
	if d.User != nil {
		email = d.User.Email
	}
	return email + ":" + d.Name
}

// RevokedDevice records the certificate serial of a deleted device so that it
// can be published in the certificate revocation list.
type RevokedDevice struct {
	Base

	Serial string
}
