package openvpn

import (
	"encoding/binary"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
)

// ExpandCIDR renders an IPv4 CIDR address as the dotted network address and
// subnet mask pair that OpenVPN and iptables expect, for example turning
// "172.25.0.0/16" into "172.25.0.0 255.255.0.0". A bare address is treated as
// a /32. An empty string is returned when value cannot be parsed or carries
// host bits beyond its prefix.
func ExpandCIDR(value string) string {
	if addr, err := netip.ParseAddr(value); err == nil && addr.Is4() {
		return addr.String() + " 255.255.255.255"
	}

	prefix, err := netip.ParsePrefix(value)
	if err != nil || !prefix.Addr().Is4() || prefix.Masked() != prefix {
		slog.Error("failed to expand CIDR value", "value", value)
		return ""
	}

	mask := net.CIDRMask(prefix.Bits(), 32)
	return prefix.Addr().String() + " " + net.IP(mask).String()
}

// AddressPlan is how the client subnet is divided. The server takes the
// first address. OpenVPN hands out addresses from the lower half, the pool,
// and the upper half is kept for fixed addresses an administrator assigns,
// so a fixed address can never already be in use by someone else.
type AddressPlan struct {
	Network netip.Prefix
	Mask    string

	Server               netip.Addr
	PoolStart, PoolEnd   netip.Addr
	FixedStart, FixedEnd netip.Addr
}

// Subnets smaller than this leave too few addresses to divide.
const (
	minSubnetBits = 8
	maxSubnetBits = 29
)

// PlanSubnet divides an IPv4 client subnet, given in CIDR notation, into
// the server's address, the pool and the fixed range.
func PlanSubnet(cidr string) (AddressPlan, error) {
	var plan AddressPlan

	prefix, err := netip.ParsePrefix(cidr)
	switch {
	case err != nil || !prefix.Addr().Is4() || prefix.Masked() != prefix:
		return plan, fmt.Errorf("openvpn: %q is not an IPv4 network address", cidr)
	case prefix.Bits() < minSubnetBits || prefix.Bits() > maxSubnetBits:
		return plan, fmt.Errorf("openvpn: the client subnet must be between /%d and /%d", minSubnetBits, maxSubnetBits)
	}

	base := toUint32(prefix.Addr())
	size := uint32(1) << (32 - prefix.Bits())
	half := base + size/2

	plan.Network = prefix
	plan.Mask = net.IP(net.CIDRMask(prefix.Bits(), 32)).String()
	plan.Server = fromUint32(base + 1)
	plan.PoolStart, plan.PoolEnd = fromUint32(base+2), fromUint32(half-1)
	plan.FixedStart, plan.FixedEnd = fromUint32(half), fromUint32(base+size-2)
	return plan, nil
}

// IsFixed reports whether addr is in the range kept for fixed addresses.
func (p AddressPlan) IsFixed(addr netip.Addr) bool {
	return addr.Is4() && toUint32(addr) >= toUint32(p.FixedStart) && toUint32(addr) <= toUint32(p.FixedEnd)
}

// FixedAddresses returns the fixed range's addresses in order, stopping
// after limit.
func (p AddressPlan) FixedAddresses(limit int) []netip.Addr {
	var out []netip.Addr
	for addr := p.FixedStart; len(out) < limit && toUint32(addr) <= toUint32(p.FixedEnd); addr = addr.Next() {
		out = append(out, addr)
	}
	return out
}

func toUint32(addr netip.Addr) uint32 {
	b := addr.As4()
	return binary.BigEndian.Uint32(b[:])
}

func fromUint32(n uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], n)
	return netip.AddrFrom4(b)
}
