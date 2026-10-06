package provision

import (
	"strings"
	"testing"
)

func TestDeviceToDeviceRule(t *testing.T) {
	const rule = "-A MangleVPN -i tun0 -o tun0 -d 10.8.0.0/24 -j ACCEPT"
	for _, on := range []bool{true, false} {
		rules, err := render("vpn.rules", vpnRulesData{
			DeviceToDevice: on, Interface: "eth0", NATInterface: "eth0",
			Port: 1194, Protocol: "udp", Subnet: "10.8.0.0/24",
		})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Contains(rules, rule); got != on {
			t.Errorf("device to device %v: rule present = %v", on, got)
		}
		// It must come before the per-group dispatch, or group rules would
		// still decide.
		if on && strings.Index(rules, rule) > strings.Index(rules, "-j MangleVPN_Clients") {
			t.Error("the rule comes after the group dispatch")
		}
	}
}
