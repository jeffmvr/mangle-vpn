package host

import (
	"log/slog"
	"net"
	"net/netip"
	"slices"
	"strings"
)

// InterfaceNames returns the names of every local interface that carries an
// IPv4 address, skipping those whose name begins with one of the given
// prefixes. The result is ordered by the kernel's interface index.
func InterfaceNames(ignore ...string) []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		slog.Error("failed to list network interfaces", "err", err)
		return nil
	}

	var names []string
	for _, iface := range ifaces {
		if slices.ContainsFunc(ignore, func(p string) bool {
			return strings.HasPrefix(iface.Name, p)
		}) {
			continue
		}
		if InterfaceIP(iface.Name) != "" {
			names = append(names, iface.Name)
		}
	}
	return names
}

// InterfaceIP returns the first IPv4 address assigned to the named interface,
// or an empty string when it has none.
func InterfaceIP(name string) string {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return ""
	}

	addrs, err := iface.Addrs()
	if err != nil {
		slog.Error("failed to get IP for iface", "iface", name, "err", err)
		return ""
	}

	for _, addr := range addrs {
		prefix, err := netip.ParsePrefix(addr.String())
		if err != nil {
			continue
		}
		if ip := prefix.Addr(); ip.Is4() {
			return ip.String()
		}
	}
	return ""
}

// IPAddresses returns every IPv4 address assigned to the machine, excluding
// the loopback interface.
func IPAddresses() []string {
	var addrs []string
	for _, name := range InterfaceNames("lo") {
		addrs = append(addrs, InterfaceIP(name))
	}
	return addrs
}
