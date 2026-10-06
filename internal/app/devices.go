package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/jobs"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/pki"
	"github.com/jeffmvr/mangle-vpn/internal/store"
)

// DeviceValidity is how long an issued device certificate lasts unless the
// settings ask for less.
const DeviceValidity = pki.Validity

// deviceValidity is how long a device certificate issued now lasts.
func (a *App) deviceValidity() time.Duration {
	if days := a.Config.Int(config.PKIDeviceDays, 0); days > 0 {
		return time.Duration(days) * 24 * time.Hour
	}
	return DeviceValidity
}

// Errors CreateDevice and IssueDeviceKeyPair return for requests that are
// refused rather than failed.
var (
	ErrDeviceLimitReached  = errors.New("app: the device limit for this group has been reached")
	ErrDuplicateDeviceName = errors.New("app: the user already has a device with this name")
	ErrAlreadyDownloaded   = errors.New("app: the device's configuration has already been downloaded")
	ErrDownloadWindowOver  = errors.New("app: the time to download the device's configuration has passed")
)

// CreateDevice registers a new device for a user, subject to the limit their
// group sets. Names are unique per user: a device's certificate is named
// after its owner and its name, and OpenVPN refuses a second connection with
// a name already in use.
func (a *App) CreateDevice(ctx context.Context, user *model.User, name, operatingSystem string) (*model.Device, error) {
	existing, err := a.Store.Devices.ByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	for _, device := range existing {
		if strings.EqualFold(device.Name, name) {
			return nil, ErrDuplicateDeviceName
		}
	}

	limit := 0
	if user.Group != nil {
		limit = user.Group.MaxDevices
	}

	device := &model.Device{Name: name, OS: operatingSystem, UserID: user.ID, User: user}
	created, err := a.Store.Devices.CreateWithinLimit(ctx, device, limit)
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, ErrDeviceLimitReached
	}

	a.Alert(ctx, config.AlertNewDevice, fmt.Sprintf("%s added a device", user.Email),
		fmt.Sprintf("%s added a device named %s. If they didn't, revoke it from their page.", user.Email, name))
	return device, nil
}

// IssueDeviceKeyPair issues the device's client certificate and records its
// fingerprint and serial, which are how the device is later recognised when
// it connects and how its certificate is revoked.
func (a *App) IssueDeviceKeyPair(ctx context.Context, device *model.Device) (pki.KeyPair, error) {
	ca, err := a.Authority()
	if err != nil {
		return pki.KeyPair{}, err
	}

	kp, err := ca.IssueClient(device.CommonName(), a.deviceValidity())
	if err != nil {
		return pki.KeyPair{}, err
	}

	claimed, err := a.Store.Devices.ClaimCertificate(ctx, device, kp.Fingerprint(), kp.SerialNumber())
	if err != nil {
		return pki.KeyPair{}, err
	}
	if !claimed {
		// Another request downloaded the configuration first. The
		// certificate made for this one is signed, so it is revoked rather
		// than simply dropped.
		if err := a.Store.RevokedDevices.Create(ctx, kp.SerialNumber()); err != nil {
			return pki.KeyPair{}, err
		}
		if err := a.QueueCRLRebuild(ctx); err != nil {
			return pki.KeyPair{}, err
		}
		return pki.KeyPair{}, ErrAlreadyDownloaded
	}
	return kp, nil
}

// DeleteDevice removes a device, disconnecting it first and adding its
// certificate to the revocation list so it cannot be used again.
func (a *App) DeleteDevice(ctx context.Context, device *model.Device) error {
	client, err := a.Store.Clients.ByDevice(ctx, device.ID)
	switch {
	case err == nil:
		if err := a.DisconnectClient(ctx, client); err != nil {
			return err
		}
	case !errors.Is(err, store.ErrNotFound):
		return err
	}

	if err := a.Store.Devices.Delete(ctx, device.ID); err != nil {
		return err
	}

	// A device that never downloaded its configuration has no certificate
	// to revoke.
	if device.Serial == "" {
		return nil
	}
	if err := a.Store.RevokedDevices.Create(ctx, device.Serial); err != nil {
		return fmt.Errorf("app: revoke %s: %w", device.CommonName(), err)
	}
	return a.QueueCRLRebuild(ctx)
}

// QueueCRLRebuild asks the task worker to republish the revocation list now,
// rather than at its hourly rebuild, so a revoked certificate stops working
// straight away.
func (a *App) QueueCRLRebuild(ctx context.Context) error {
	return a.Jobs.Enqueue(ctx, jobs.KindCreateCRL, nil)
}

// DeviceDownloadWindow is how long after creation a device's configuration
// may be fetched, by download or by an OpenVPN Connect import link.
//
// A configuration carries the device's private key and is only ever handed
// out once, shortly after the device is created. Five minutes leaves room to
// start OpenVPN Connect and confirm the import, or to fall back to a
// download when Connect turns out not to be installed. The Django release
// allowed one minute, for downloads only.
const DeviceDownloadWindow = 5 * time.Minute

// CheckDownload reports why a device's configuration may no longer be
// fetched, or nil when it still may.
func CheckDownload(device *model.Device) error {
	switch {
	case device.Fingerprint != "" || device.Serial != "":
		return ErrAlreadyDownloaded
	case time.Since(device.CreatedAt) > DeviceDownloadWindow:
		return ErrDownloadWindowOver
	}
	return nil
}

// CreateImportToken issues a single-use code with which OpenVPN Connect can
// fetch the device's profile itself. The code lapses with the device's
// download window, and only its hash is stored.
func (a *App) CreateImportToken(ctx context.Context, device *model.Device) (string, error) {
	if err := CheckDownload(device); err != nil {
		return "", err
	}

	token := rand.Text()
	expires := device.CreatedAt.Add(DeviceDownloadWindow)
	if err := a.Store.Devices.SetImportToken(ctx, device.ID, hashImportToken(token), expires); err != nil {
		return "", err
	}
	return token, nil
}

// RedeemImportToken spends an import code and returns its device, which
// still has to be issued its certificate. It returns store.ErrNotFound for
// a code that is unknown, used, or lapsed.
func (a *App) RedeemImportToken(ctx context.Context, token string) (*model.Device, error) {
	id, err := a.Store.Devices.TakeImportToken(ctx, hashImportToken(token))
	if err != nil {
		return nil, err
	}
	return a.Store.Devices.Get(ctx, id)
}

// hashImportToken is how an import code is stored: a database leak must not
// hand out working links.
func hashImportToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// CleanDeviceName keeps only the characters a device name may hold: letters,
// digits, spaces, and . _ -. Names are carried in a certificate subject as
// "email:name" and rewritten by OpenVPN when it meets anything else, so
// everything else is dropped rather than risked.
func CleanDeviceName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == ' ', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// RetireIdleDevices removes the devices that have gone unused for longer
// than their owner's group allows, revoking their certificates. Each owner
// sees it in the audit log; they can add the device again.
func (a *App) RetireIdleDevices(ctx context.Context) error {
	groups, err := a.Store.Groups.All(ctx)
	if err != nil {
		return err
	}

	for _, group := range groups {
		if group.DeviceIdleDays <= 0 {
			continue
		}

		cutoff := time.Now().AddDate(0, 0, -group.DeviceIdleDays)
		devices, err := a.Store.Devices.IdleInGroup(ctx, group.ID, cutoff)
		if err != nil {
			return err
		}
		for _, device := range devices {
			if err := a.DeleteDevice(ctx, device); err != nil {
				return err
			}
			a.RecordEvent(ctx, device.User, model.EventDeviceRetire,
				fmt.Sprintf("Removed device %s after %d days without connecting.", device.Name, group.DeviceIdleDays))
			a.Log.Info("retired an unused device", "device", device.Name,
				"user", device.User.Email, "group", group.Name, "idle_days", group.DeviceIdleDays)
		}
	}
	return nil
}
