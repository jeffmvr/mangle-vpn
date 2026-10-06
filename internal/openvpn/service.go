package openvpn

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/jeffmvr/mangle-vpn/internal/host"
)

// Unit is the systemd unit that runs the OpenVPN server.
const Unit = "mangle-vpn"

// Device is the tunnel interface the server configuration names.
const Device = "tun0"

// sysNet is where the kernel lists network interfaces.
var sysNet = "/sys/class/net"

// Offload reports whether the running server's tunnel is handled by data
// channel offload (DCO), where the kernel encrypts and decrypts traffic
// rather than OpenVPN. known is false when there is no tunnel to look at,
// as when OpenVPN is stopped.
//
// A classic tun device exposes tun_flags; an offloaded one is a different
// kind of device and does not.
func Offload() (on, known bool) {
	dir := filepath.Join(sysNet, Device)
	if _, err := os.Stat(dir); err != nil {
		return false, false
	}
	_, err := os.Stat(filepath.Join(dir, "tun_flags"))
	return errors.Is(err, fs.ErrNotExist), true
}

// Service runs the OpenVPN server. On a server it is the systemd unit; in a
// container, where there is no systemd, the application runs OpenVPN
// itself and installs its own Service with [Use].
type Service interface {
	Start(ctx context.Context) bool
	Stop(ctx context.Context) bool
	Restart(ctx context.Context) bool
	IsRunning(ctx context.Context) bool
}

// service is what the functions below act through.
var service Service = systemdService{}

// Use makes s the way the OpenVPN server is run, for this process.
func Use(s Service) { service = s }

// Start starts the OpenVPN server.
func Start(ctx context.Context) bool { return service.Start(ctx) }

// Stop stops the OpenVPN server.
func Stop(ctx context.Context) bool { return service.Stop(ctx) }

// Restart restarts the OpenVPN server.
func Restart(ctx context.Context) bool { return service.Restart(ctx) }

// IsRunning reports whether the OpenVPN server is running.
func IsRunning(ctx context.Context) bool { return service.IsRunning(ctx) }

// systemdService runs the OpenVPN server through its systemd unit.
type systemdService struct{}

func (systemdService) Start(ctx context.Context) bool {
	return host.Run(ctx, "systemctl", "start", Unit)
}

func (systemdService) Stop(ctx context.Context) bool {
	return host.Run(ctx, "systemctl", "stop", Unit)
}

func (systemdService) Restart(ctx context.Context) bool {
	return host.Run(ctx, "systemctl", "restart", Unit)
}

func (systemdService) IsRunning(ctx context.Context) bool {
	return host.Run(ctx, "systemctl", "is-active", "--quiet", Unit)
}

// tlsAuthKeyBytes is the size of an OpenVPN static key.
const tlsAuthKeyBytes = 256

// tlsAuthKeyColumns is the number of key bytes printed per line.
const tlsAuthKeyColumns = 16

// NewTLSAuthKey returns a new OpenVPN static key, in the same layout that
// "openvpn --genkey" writes.
//
// Generating it here rather than shelling out keeps the application working
// across the OpenVPN 2.4 to 2.6 change in --genkey's syntax.
func NewTLSAuthKey() string {
	key := make([]byte, tlsAuthKeyBytes)
	rand.Read(key)

	var b strings.Builder
	b.WriteString("#\n# 2048 bit OpenVPN static key\n#\n")
	b.WriteString("-----BEGIN OpenVPN Static key V1-----\n")

	for chunk := range slices.Chunk(key, tlsAuthKeyColumns) {
		b.WriteString(hex.EncodeToString(chunk))
		b.WriteString("\n")
	}

	b.WriteString("-----END OpenVPN Static key V1-----\n")
	return b.String()
}
