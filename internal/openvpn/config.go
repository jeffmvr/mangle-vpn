// Package openvpn renders OpenVPN configuration, controls the server
// process, and talks to its management socket.
package openvpn

import (
	"embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed templates/*.conf
var templateFS embed.FS

// templates holds the server and client configuration templates.
var templates = template.Must(template.New("openvpn").
	Funcs(template.FuncMap{"expandCIDR": ExpandCIDR}).
	ParseFS(templateFS, "templates/*.conf"))

// Supported client operating systems. Each needs slightly different DNS
// handling in its configuration.
const (
	OSWindows = "windows"
	OSMacOS   = "macos"
	OSLinux   = "linux"
)

// IsSupportedOS reports whether a client configuration can be built for the
// named operating system.
func IsSupportedOS(name string) bool {
	switch name {
	case OSWindows, OSMacOS, OSLinux:
		return true
	default:
		return false
	}
}

// ServerConfig describes the OpenVPN server configuration to render.
//
// There is no Diffie-Hellman field: the rendered configuration sets
// "dh none", which turns off the finite-field key exchange and leaves the
// control channel on ECDHE alone.
type ServerConfig struct {
	BindAddress      string
	BindPort         int
	CACertificate    string
	Certificate      string
	CRLFile          string
	Domain           string
	Executable       string
	LogFile          string
	LogLevel         int
	ManagementSocket string
	Nameservers      []string
	PrivateKey       string
	Protocol         string
	RedirectGateway  bool
	Routes           []string
	SessionLifetime  int
	StatusFile       string
	Subnet           string
	TLSAuthKey       string

	// PortShare, when set, is the local port of the web server, which
	// OpenVPN hands any connection that is not OpenVPN's own. It needs TCP.
	PortShare int

	// MSSFix, when set, is the largest TCP segment carried through the
	// tunnel, which helps on links with a smaller MTU. It needs UDP.
	MSSFix int

	// Plan is worked out from Subnet when the configuration is rendered.
	Plan AddressPlan
}

// Render returns the OpenVPN server configuration file.
func (c ServerConfig) Render() (string, error) {
	plan, err := PlanSubnet(c.Subnet)
	if err != nil {
		return "", err
	}
	c.Plan = plan
	return render("server.conf", c)
}

// ClientConnectConfig is what the server is told about one client as it
// connects: its fixed address, if it has one, and the routes and DNS
// servers its group adds to the server-wide ones.
type ClientConnectConfig struct {
	FixedAddress string
	Mask         string
	Routes       []string
	Nameservers  []string
}

// Render returns the lines OpenVPN reads from the client-connect hook's
// configuration file.
func (c ClientConnectConfig) Render() (string, error) {
	return render("client-connect.conf", c)
}

// ClientConfig describes the OpenVPN configuration handed to a device.
//
// ProfileName and FriendlyName, when set, are written as the comment lines
// Access Server puts in its profiles, which clients importing from a URL
// read to name the profile. They are reduced to a single line, since they
// are written into the file as they are.
type ClientConfig struct {
	CACertificate string
	Certificate   string
	FriendlyName  string
	Hostname      string
	IdleTimeout   int
	OS            string
	Port          int
	PrivateKey    string
	ProfileName   string
	Protocol      string
	TLSAuthKey    string

	// StaticChallenge, when set, makes the client ask for a code as well as
	// the password, with this prompt, and send both. See ParseStaticChallenge.
	StaticChallenge string

	// MSSFix matches the server's, as it works best set at both ends.
	MSSFix int
}

// Render returns the OpenVPN client configuration file.
func (c ClientConfig) Render() (string, error) {
	c.FriendlyName = singleLine(c.FriendlyName)
	c.ProfileName = singleLine(c.ProfileName)
	c.StaticChallenge = strings.ReplaceAll(singleLine(c.StaticChallenge), `"`, "'")
	return render("client.conf", c)
}

// singleLine drops control characters, newlines included, so that a value
// cannot add lines of its own to a configuration file.
func singleLine(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, value)
}

// render executes a configuration template and tidies up the blank lines the
// conditional sections leave behind.
func render(name string, data any) (string, error) {
	var b strings.Builder
	if err := templates.ExecuteTemplate(&b, name, data); err != nil {
		return "", fmt.Errorf("openvpn: render %s: %w", name, err)
	}
	return removeEmptyLines(b.String()) + "\n", nil
}

// removeEmptyLines strips the blank lines that a template's conditional
// sections leave behind.
func removeEmptyLines(value string) string {
	lines := strings.Split(value, "\n")
	kept := lines[:0]
	for _, line := range lines {
		if line = strings.TrimRight(line, "\r"); line != "" {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}
