package provision

import (
	"context"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/iptables"
)

// Application iptables chains.
const (
	// VPNChain holds the rules every VPN client is filtered through.
	VPNChain = "MangleVPN"

	// ClientsChain dispatches each connected client to its group's chain.
	ClientsChain = "MangleVPN_Clients"
)

// webRulesData fills in the web firewall rule template.
type webRulesData struct {
	HTTPPort  int
	HTTPSPort int
}

// WebRules returns the firewall rules that open the web application's ports.
func WebRules(cfg *config.Config) (string, error) {
	return render("web.rules", webRulesData{
		HTTPPort:  cfg.Int(config.AppHTTPPort, 80),
		HTTPSPort: cfg.Int(config.AppHTTPSPort, 443),
	})
}

// vpnRulesData fills in the VPN firewall rule template.
type vpnRulesData struct {
	DeviceToDevice bool
	Interface      string
	NATInterface   string
	LocalAddresses []string
	Port           int
	Protocol       string
	Nameservers    []string
	Subnet         string
}

// VPNRules returns the base firewall rules for the OpenVPN server: the rules
// that admit tunnelled traffic, masquerade it, and send it through the
// application's filter chain.
func VPNRules(cfg *config.Config) (string, error) {
	return render("vpn.rules", vpnRulesData{
		DeviceToDevice: cfg.Bool(config.VPNDeviceToDevice, false),
		Interface:      cfg.Get(config.VPNInterface),
		NATInterface:   cfg.Get(config.VPNNATInterface),
		LocalAddresses: host.IPAddresses(),
		Port:           cfg.Int(config.VPNPort, 1194),
		Protocol:       cfg.GetOr(config.VPNProtocol, "udp"),
		Nameservers:    cfg.Lines(config.VPNNameservers),
		Subnet:         cfg.Get(config.VPNSubnet),
	})
}

// ApplyRules installs each rule in a rendered rule set.
func ApplyRules(ctx context.Context, rules string) {
	for rule := range strings.SplitSeq(rules, "\n") {
		if strings.TrimSpace(rule) != "" {
			iptables.Run(ctx, rule)
		}
	}
}

// WithdrawRules removes each rule in a rendered rule set by turning its
// appends into deletes. The set is the one recorded when the rules were
// installed, so the rules come out even if the settings have since changed.
func WithdrawRules(ctx context.Context, rules string) {
	for rule := range strings.SplitSeq(rules, "\n") {
		if strings.TrimSpace(rule) != "" {
			iptables.Run(ctx, strings.Replace(rule, "-A", "-D", 1))
		}
	}
}
