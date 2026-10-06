package model

import (
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// Client is a device that is currently connected to the OpenVPN server.
type Client struct {
	Base

	CommonName string
	DeviceID   uuid.UUID
	Platform   string
	RemoteIP   string
	VirtualIP  string

	Device *Device
}

// Duration returns how many seconds the client has been connected.
func (c *Client) Duration() int {
	return int(time.Since(c.CreatedAt).Seconds())
}

// User returns the user the connected device belongs to.
func (c *Client) User() *User {
	if c.Device == nil {
		return nil
	}
	return c.Device.User
}

// Group returns the group of the user the connected device belongs to.
func (c *Client) Group() *Group {
	if user := c.User(); user != nil {
		return user.Group
	}
	return nil
}
