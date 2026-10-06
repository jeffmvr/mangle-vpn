package app

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/host"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// AutomaticAddress asks AssignStaticIP for the first free fixed address.
const AutomaticAddress = "auto"

// Why a fixed address could not be assigned.
var (
	ErrAddressOutOfRange = errors.New("app: the address is not in the range kept for fixed addresses")
	ErrAddressTaken      = errors.New("app: another device already has this address")
	ErrNoFreeAddress     = errors.New("app: every fixed address is taken")
)

// AddressPlan returns how the configured client subnet is divided.
func (a *App) AddressPlan() (openvpn.AddressPlan, error) {
	return openvpn.PlanSubnet(a.Config.Get(config.VPNSubnet))
}

// AssignStaticIP gives a device a fixed VPN address: the one requested,
// the first free one for AutomaticAddress, or none for "". The device gets
// it the next time it connects.
func (a *App) AssignStaticIP(ctx context.Context, device *model.Device, requested string) error {
	if requested == "" {
		return a.Store.Devices.SetStaticIP(ctx, device, "")
	}

	plan, err := a.AddressPlan()
	if err != nil {
		return err
	}

	address := requested
	if requested == AutomaticAddress {
		if address, err = a.freeStaticIP(ctx, plan); err != nil {
			return err
		}
	} else if addr, err := netip.ParseAddr(requested); err != nil || !plan.IsFixed(addr) {
		return ErrAddressOutOfRange
	}

	err = a.Store.Devices.SetStaticIP(ctx, device, address)
	if errors.Is(err, store.ErrConflict) {
		return ErrAddressTaken
	}
	return err
}

// freeStaticIP returns the lowest fixed address no device has. Addresses
// ending in .0 or .255 are passed over: they are usable inside a larger
// subnet, but read as network and broadcast addresses to people and to
// some tools.
func (a *App) freeStaticIP(ctx context.Context, plan openvpn.AddressPlan) (string, error) {
	used, err := a.Store.Devices.StaticIPs(ctx)
	if err != nil {
		return "", err
	}
	// Every address in use, and every .0 and .255 passed over, can push the
	// first free one further; allowing for both is enough to find a gap.
	for _, addr := range plan.FixedAddresses(2*len(used) + 3) {
		last := addr.As4()[3]
		if last != 0 && last != 255 && !used[addr.String()] {
			return addr.String(), nil
		}
	}
	return "", ErrNoFreeAddress
}

// clientConnectConfig works out what the server is told about a device as
// it connects, and the VPN address it will have.
func (a *App) clientConnectConfig(device *model.Device, poolAddress string) (openvpn.ClientConnectConfig, string) {
	var conf openvpn.ClientConnectConfig
	address := poolAddress

	if device.StaticIP != "" {
		plan, err := a.AddressPlan()
		addr, parseErr := netip.ParseAddr(device.StaticIP)
		switch {
		case err != nil || parseErr != nil || !plan.IsFixed(addr):
			// The subnet has changed since the address was given; the device
			// connects with one from the pool rather than not at all.
			a.Log.Warn("a fixed address is outside the client subnet, using the pool",
				"device", device.Name, "address", device.StaticIP)
		default:
			conf.FixedAddress, conf.Mask = device.StaticIP, plan.Mask
			address = device.StaticIP
		}
	}

	if group := groupOf(device); group != nil {
		conf.Routes = config.SplitLines(group.Routes)
		conf.Nameservers = config.SplitLines(group.Nameservers)
	}
	return conf, address
}

// groupOf returns the group of the device's owner, if loaded.
func groupOf(device *model.Device) *model.Group {
	if device.User == nil {
		return nil
	}
	return device.User.Group
}

// writeClientConnectConfig writes the lines OpenVPN reads back from the
// client-connect hook, to the file it named.
func writeClientConnectConfig(path string, conf openvpn.ClientConnectConfig) error {
	rendered, err := conf.Render()
	if err != nil {
		return err
	}
	if err := host.WriteFile(path, rendered, 0o600); err != nil {
		return fmt.Errorf("app: write the client's configuration: %w", err)
	}
	return nil
}
