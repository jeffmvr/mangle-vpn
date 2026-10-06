package model

import (
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// VPNSession is one finished VPN connection. Byte counts are from the
// server's side: received from the device, and sent to it.
type VPNSession struct {
	ID            uuid.UUID
	DeviceID      uuid.UUID
	UserID        uuid.UUID
	StartedAt     time.Time
	EndedAt       time.Time
	RemoteIP      string
	VirtualIP     string
	BytesReceived int64
	BytesSent     int64

	// DeviceName is the device's name, loaded alongside for listing.
	DeviceName string
}

// Duration returns how long the connection lasted.
func (s *VPNSession) Duration() time.Duration {
	return s.EndedAt.Sub(s.StartedAt)
}
