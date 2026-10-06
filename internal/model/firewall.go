package model

import (
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/uuid"
)

// Firewall rule actions.
const (
	ActionAccept = "ACCEPT"
	ActionDrop   = "DROP"
)

// Firewall rule protocols.
const (
	ProtocolAll = "all"
	ProtocolTCP = "tcp"
	ProtocolUDP = "udp"
)

// FirewallRule is one entry in a group's iptables chain, describing traffic
// that members of the group may or may not send over the VPN.
type FirewallRule struct {
	Base

	Action      string
	Destination string
	GroupID     uuid.UUID
	IsEnabled   bool
	Port        string
	Protocol    string
}

// Args returns the iptables arguments that express the rule.
func (r *FirewallRule) Args() []string {
	var args []string

	if r.Destination != "" {
		args = append(args, "-d", r.Destination)
	}

	if r.Protocol != "" {
		args = append(args, "-p", r.Protocol)

		// Ports only mean anything once a protocol is set. They may be given
		// as a single port, a low:high range, or a comma separated mix of
		// both, and anything but a single port needs the multiport match.
		if r.Port != "" {
			if strings.ContainsAny(r.Port, ",:") {
				args = append(args, "--match", "multiport", "--dports", r.Port)
			} else {
				args = append(args, "--dport", r.Port)
			}
		}
	}

	return append(args, "-j", r.Action)
}
