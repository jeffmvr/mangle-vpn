package app

import (
	"context"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/openvpn"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
)

// RenewVPNCertificate issues the OpenVPN server a new certificate and key
// from the same certificate authority. Devices trust the authority rather
// than the server's certificate, so none of them need a new profile; the
// server picks it up when it is next restarted, which is flagged as pending.
func (a *App) RenewVPNCertificate(ctx context.Context) error {
	if err := a.issueVPNCertificate(ctx); err != nil {
		return err
	}
	return a.Config.SetBool(ctx, config.VPNRestartPending, true)
}

// CreateVPNKeys issues the OpenVPN server's certificate and the shared key
// that goes alongside it, leaving them alone when they already exist.
func (a *App) CreateVPNKeys(ctx context.Context) error {
	if a.Config.Get(config.VPNCertificate) == "" {
		if err := a.issueVPNCertificate(ctx); err != nil {
			return err
		}
	}
	if a.Config.Get(config.VPNTLSAuthKey) == "" {
		return a.Config.Set(ctx, config.VPNTLSAuthKey, openvpn.NewTLSAuthKey())
	}
	return nil
}

// issueVPNCertificate stores a new certificate and key for the OpenVPN
// server, signed by the certificate authority.
func (a *App) issueVPNCertificate(ctx context.Context) error {
	ca, err := a.Authority()
	if err != nil {
		return err
	}

	kp, err := ca.IssueServer("OpenVPN Server", pki.Validity)
	if err != nil {
		return err
	}

	certificate, privateKey := kp.PEM()
	if err := a.Config.Set(ctx, config.VPNPrivateKey, privateKey); err != nil {
		return err
	}
	return a.Config.Set(ctx, config.VPNCertificate, certificate)
}
