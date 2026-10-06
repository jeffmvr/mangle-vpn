package model

// chainPrefix begins the name of every per-group iptables chain.
const chainPrefix = "MangleVPN_Group_"

// chainNameLimit is the longest chain name iptables will accept.
const chainNameLimit = 28

// Group is a set of users that share device limits and firewall rules.
type Group struct {
	Base

	Description string
	IsEnabled   bool
	MaxDevices  int
	MFAEnforced bool
	Name        string

	// Routes and Nameservers are pushed to members as they connect, on top
	// of the server-wide ones: networks in CIDR notation and DNS server
	// addresses, one per line.
	Routes      string
	Nameservers string

	// DeviceIdleDays removes a member's device once it has gone unused for
	// this many days; 0 keeps devices however long they sit.
	DeviceIdleDays int
}

// Chain returns the name of the group's iptables chain.
func (g *Group) Chain() string {
	name := chainPrefix + g.ID.Hex()
	return name[:min(len(name), chainNameLimit)]
}
