package openvpn

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeServer starts a stand-in for the OpenVPN management socket. It greets
// each connection, records the commands it receives, and answers them with
// reply. Returns the socket path and the channel of received commands.
func fakeServer(t *testing.T, reply func(command string) string) (string, <-chan string) {
	t.Helper()

	// macOS caps a unix socket path at around a hundred characters, which
	// the default per-test temporary directory already exceeds.
	dir, err := os.MkdirTemp("/tmp", "mangle")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	socket := filepath.Join(dir, "m.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	commands := make(chan string, 8)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		// OpenVPN greets a new connection before accepting commands.
		conn.Write([]byte(">INFO:OpenVPN Management Interface Version 1\r\n"))

		buf := make([]byte, 256)
		for {
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			command := strings.TrimSpace(string(buf[:n]))
			commands <- command

			if answer := reply(command); answer != "" {
				conn.Write([]byte(answer))
			}
		}
	}()

	return socket, commands
}

func TestServerConfigRender(t *testing.T) {
	config := ServerConfig{
		BindAddress:      "198.51.100.10",
		BindPort:         1194,
		CACertificate:    "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----",
		Certificate:      "-----BEGIN CERTIFICATE-----\nserver\n-----END CERTIFICATE-----",
		CRLFile:          "/opt/mangle-vpn/data/keys/crl.pem",
		Domain:           "example.com",
		Executable:       "/opt/mangle-vpn/mangle",
		LogFile:          "/opt/mangle-vpn/data/logs/openvpn.log",
		ManagementSocket: "/run/mangle-vpn.sock",
		Nameservers:      []string{"10.0.0.53", "10.0.0.54"},
		PrivateKey:       "-----BEGIN RSA PRIVATE KEY-----\nkey\n-----END RSA PRIVATE KEY-----",
		Protocol:         "udp",
		RedirectGateway:  true,
		Routes:           []string{"10.10.0.0/16"},
		StatusFile:       "/opt/mangle-vpn/data/logs/openvpn-status.log",
		Subnet:           "172.25.0.0/16",
		TLSAuthKey:       NewTLSAuthKey(),
	}

	got, err := config.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	want := []string{
		"proto udp",
		"local 198.51.100.10",
		"lport 1194",
		// A CIDR subnet has to reach OpenVPN as an address and mask pair.
		"server 172.25.0.0 255.255.0.0",
		"management /run/mangle-vpn.sock unix",
		// The finite-field key exchange is off; the control channel uses
		// ECDHE alone.
		"dh none",
		"crl-verify /opt/mangle-vpn/data/keys/crl.pem",
		`auth-user-pass-verify "/opt/mangle-vpn/mangle vpn client-authenticate" via-env`,
		`client-connect "/opt/mangle-vpn/mangle vpn client-connect"`,
		`client-disconnect "/opt/mangle-vpn/mangle vpn client-disconnect"`,
		`push "redirect-gateway def1"`,
		`push "dhcp-option DNS 10.0.0.53"`,
		`push "route 10.0.0.54"`,
		`push "dhcp-option DOMAIN example.com"`,
		`push "route 10.10.0.0 255.255.0.0"`,
		"<ca>",
		"<tls-auth>",
	}
	for _, line := range want {
		if !strings.Contains(got, line) {
			t.Errorf("server config is missing %q\n\n%s", line, got)
		}
	}

	// OpenVPN reads this line by line, so stray blank lines from the
	// conditional sections must not survive.
	for i, line := range strings.Split(strings.TrimRight(got, "\n"), "\n") {
		if strings.TrimSpace(line) == "" {
			t.Errorf("server config line %d is blank", i+1)
		}
	}
}

func TestServerConfigOmitsUnsetSections(t *testing.T) {
	config := ServerConfig{Protocol: "tcp", Subnet: "172.25.0.0/16", BindPort: 443}

	got, err := config.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}

	for _, absent := range []string{"redirect-gateway", "dhcp-option DNS", "dhcp-option DOMAIN"} {
		if strings.Contains(got, absent) {
			t.Errorf("server config should not mention %q\n\n%s", absent, got)
		}
	}
}

func TestClientConfigPerOperatingSystem(t *testing.T) {
	base := ClientConfig{
		CACertificate: "ca",
		Certificate:   "cert",
		Hostname:      "vpn.example.com",
		Port:          1194,
		PrivateKey:    "key",
		Protocol:      "udp",
		TLSAuthKey:    "tls",
	}

	tests := []struct {
		os      string
		want    []string
		notWant []string
	}{
		{
			os:      OSWindows,
			want:    []string{"block-outside-dns"},
			notWant: []string{"update-resolv-conf"},
		},
		{
			// OpenVPN Connect sets DNS itself, and the classic client's
			// resolver script is absent on many systems, where naming it
			// stops the connection.
			os:      OSLinux,
			notWant: []string{"block-outside-dns", "update-resolv-conf", "script-security"},
		},
		{
			// macOS needs neither; its resolver is handled by the client.
			os:      OSMacOS,
			notWant: []string{"block-outside-dns", "update-resolv-conf"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			config := base
			config.OS = tt.os

			got, err := config.Render()
			if err != nil {
				t.Fatalf("Render: %v", err)
			}

			if !strings.Contains(got, "remote vpn.example.com 1194 udp") {
				t.Errorf("client config is missing its remote\n\n%s", got)
			}
			// explicit-exit-notify is only meaningful over UDP.
			if !strings.Contains(got, "explicit-exit-notify") {
				t.Error("a UDP client config should ask for an exit notification")
			}
			for _, line := range tt.want {
				if !strings.Contains(got, line) {
					t.Errorf("client config is missing %q\n\n%s", line, got)
				}
			}
			for _, line := range tt.notWant {
				if strings.Contains(got, line) {
					t.Errorf("client config should not mention %q\n\n%s", line, got)
				}
			}
		})
	}
}

func TestClientConfigTCPHasNoExitNotify(t *testing.T) {
	config := ClientConfig{Protocol: "tcp", OS: OSWindows, Hostname: "vpn.example.com", Port: 443}

	got, err := config.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if strings.Contains(got, "explicit-exit-notify") {
		t.Errorf("explicit-exit-notify is not valid over TCP\n\n%s", got)
	}
}

func TestNewTLSAuthKey(t *testing.T) {
	key := NewTLSAuthKey()

	if !strings.Contains(key, "-----BEGIN OpenVPN Static key V1-----") {
		t.Fatalf("key has no opening marker:\n%s", key)
	}

	var body []string
	for _, line := range strings.Split(strings.TrimSpace(key), "\n") {
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "-----") {
			body = append(body, line)
		}
	}

	// A 2048 bit static key is 16 lines of 32 hex characters, which is what
	// "openvpn --genkey" writes and what OpenVPN expects to read back.
	if len(body) != 16 {
		t.Errorf("key body has %d lines, want 16", len(body))
	}
	for _, line := range body {
		if len(line) != 32 {
			t.Errorf("key line %q is %d characters, want 32", line, len(line))
		}
	}
	if NewTLSAuthKey() == key {
		t.Error("two generated keys were identical")
	}
}

func TestManagementRunAndKill(t *testing.T) {
	socket, commands := fakeServer(t, func(command string) string {
		if !strings.HasPrefix(command, "kill") {
			return ""
		}
		// A real-time notification may arrive before the reply, and must
		// not be mistaken for it.
		return ">BYTECOUNT:1,2\r\nSUCCESS: common name person@example.com:laptop found\r\n"
	})

	if err := KillClient(t.Context(), socket, "person@example.com:laptop"); err != nil {
		t.Fatalf("KillClient: %v", err)
	}

	if got := <-commands; got != "kill person@example.com:laptop" {
		t.Errorf("sent %q, want the kill command", got)
	}
}

func TestManagementReportsServerErrors(t *testing.T) {
	socket, _ := fakeServer(t, func(string) string {
		return "ERROR: common name 'ghost' not found\r\n"
	})

	err := KillClient(t.Context(), socket, "ghost")
	if err == nil {
		t.Fatal("killing an unknown client reported success")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %v, want it to carry the server's message", err)
	}
}

func TestConnectToMissingSocket(t *testing.T) {
	// The VPN hooks run whether or not the server is up, so a dead socket
	// has to come back as an error rather than a hang.
	_, err := Connect(t.Context(), filepath.Join(t.TempDir(), "absent.sock"))
	if err == nil {
		t.Fatal("connecting to a missing socket reported success")
	}
}

func TestClientConfigNamesStayOnOneLine(t *testing.T) {
	conf, err := ClientConfig{
		FriendlyName: "Acme\nscript-security 2\nup /bin/sh",
		ProfileName:  "ada@example.com@vpn.example.com",
		Hostname:     "vpn.example.com",
		Port:         1194,
		Protocol:     "udp",
	}.Render()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(conf, "\nscript-security") || strings.Contains(conf, "\nup ") {
		t.Errorf("a name added lines to the profile:\n%s", conf)
	}
	if !strings.HasPrefix(conf, "# OVPN_ACCESS_SERVER_PROFILE=ada@example.com@vpn.example.com\n") {
		t.Errorf("profile does not open with its name:\n%s", conf)
	}
}

func TestExpandCIDR(t *testing.T) {
	for in, want := range map[string]string{
		"172.25.0.0/16": "172.25.0.0 255.255.0.0",
		"10.0.0.5":      "10.0.0.5 255.255.255.255",
		"10.0.0.5/24":   "", // host bits set
		"2001:db8::/32": "",
		"nonsense":      "",
	} {
		if got := ExpandCIDR(in); got != want {
			t.Errorf("ExpandCIDR(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestServerConfigSettings(t *testing.T) {
	conf, err := ServerConfig{
		BindPort: 1194, Protocol: "udp", Subnet: "10.8.0.0/24",
		LogLevel: 4, SessionLifetime: 12 * 3600,
	}.Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"verb 4\n",
		"auth-gen-token 43200\n",
		"data-ciphers AES-256-GCM:AES-128-GCM:CHACHA20-POLY1305\n",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("server configuration lacks %q", want)
		}
	}
	// Deprecated since OpenVPN 2.5, and warned about by 2.6.
	for _, gone := range []string{"\ncipher ", "ncp-ciphers"} {
		if strings.Contains(conf, gone) {
			t.Errorf("server configuration still has %q", gone)
		}
	}
}

func TestClientConfigIdleTimeout(t *testing.T) {
	base := ClientConfig{Hostname: "vpn.example.com", Port: 1194, Protocol: "udp"}

	withIdle := base
	withIdle.IdleTimeout = 1800
	conf, _ := withIdle.Render()
	if !strings.Contains(conf, "\ninactive 1800\n") {
		t.Error("an idle timeout was not written")
	}

	conf, _ = base.Render()
	if strings.Contains(conf, "inactive") {
		t.Error("no idle timeout should mean no inactive line")
	}
	// Clients negotiate the cipher with the server; naming one here is
	// deprecated, and OpenVPN 2.4 clients negotiate without it.
	if strings.Contains(conf, "\ncipher ") || strings.Contains(conf, "ncp-ciphers") {
		t.Error("the client profile still names a cipher")
	}
}

func TestHardeningLines(t *testing.T) {
	server, _ := ServerConfig{BindPort: 1194, Protocol: "udp", Subnet: "10.8.0.0/24", LogLevel: 3}.Render()
	client, _ := ClientConfig{Hostname: "vpn.example.com", Port: 1194, Protocol: "udp"}.Render()

	for name, conf := range map[string]string{"server": server, "client": client} {
		if !strings.Contains(conf, "\ntls-version-min 1.2\n") {
			t.Errorf("%s does not require TLS 1.2", name)
		}
		// Left to the operating system, which tunes them better.
		if strings.Contains(conf, "sndbuf") || strings.Contains(conf, "rcvbuf") {
			t.Errorf("%s still fixes the socket buffers", name)
		}
	}
	if !strings.Contains(server, "\nallow-compression no\n") {
		t.Error("the server does not refuse compression")
	}
	// Pushed by the server, so not repeated in the profile.
	for _, line := range []string{"\ntopology", "\nkeepalive"} {
		if strings.Contains(client, line) {
			t.Errorf("the profile repeats %q", line)
		}
	}
}

func TestStaticChallenge(t *testing.T) {
	plain, _ := ClientConfig{Hostname: "vpn.example.com", Port: 1194, Protocol: "udp"}.Render()
	if strings.Contains(plain, "static-challenge") {
		t.Error("a profile without a challenge prompt asks for one")
	}

	asking, _ := ClientConfig{Hostname: "vpn.example.com", Port: 1194, Protocol: "udp",
		StaticChallenge: `Code "now"` + "\nup /bin/sh"}.Render()
	if !strings.Contains(asking, "\nstatic-challenge \"Code 'now'up /bin/sh\" 1\n") {
		t.Errorf("the prompt was not written safely on one line:\n%s", asking)
	}

	for in, want := range map[string][3]any{
		"SCRV1:UGFzc3dvcmQx:MTIzNDU2": {"Password1", "123456", true},
		"SCRV1:UGFzc3dvcmQx:":         {"Password1", "", true},
		"123456":                      {"", "", false},
		"SCRV1:not base64!:MTIz":      {"", "", false},
		"SCRV1:onlyonepart":           {"", "", false},
	} {
		password, response, ok := ParseStaticChallenge(in)
		if password != want[0] || response != want[1] || ok != want[2] {
			t.Errorf("ParseStaticChallenge(%q) = %q, %q, %v; want %v", in, password, response, ok, want)
		}
	}
}

func TestPlanSubnet(t *testing.T) {
	plan, err := PlanSubnet("172.25.0.0/16")
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"172.25.0.1":     plan.Server.String(),
		"172.25.0.2":     plan.PoolStart.String(),
		"172.25.127.255": plan.PoolEnd.String(),
		"172.25.128.0":   plan.FixedStart.String(),
		"172.25.255.254": plan.FixedEnd.String(),
		"255.255.0.0":    plan.Mask,
	} {
		if got != name {
			t.Errorf("got %s, want %s", got, name)
		}
	}

	for _, bad := range []string{"172.25.0.1/16", "10.0.0.0/30", "10.0.0.0/7", "fd00::/64", "nonsense"} {
		if _, err := PlanSubnet(bad); err == nil {
			t.Errorf("PlanSubnet(%q) accepted", bad)
		}
	}
}

func TestServerConfigKeepsTheUpperHalfOutOfThePool(t *testing.T) {
	got, err := ServerConfig{Protocol: "udp", Subnet: "10.8.0.0/24", BindPort: 1194}.Render()
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"server 10.8.0.0 255.255.255.0 nopool", "ifconfig-pool 10.8.0.2 10.8.0.127 255.255.255.0"} {
		if !strings.Contains(got, line+"\n") {
			t.Errorf("server config lacks %q", line)
		}
	}
}

func TestClientConnectConfigOnlyHoldsWhatIsSet(t *testing.T) {
	got, err := ClientConnectConfig{}.Render()
	if err != nil || strings.TrimSpace(got) != "" {
		t.Errorf("an empty client config rendered %q, %v", got, err)
	}
}

func TestReadStatusVersion2(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status.log")
	os.WriteFile(path, []byte(`TITLE,OpenVPN 2.6.12 x86_64-pc-linux-gnu
TIME,2026-10-05 20:10:00,1791249000
HEADER,CLIENT_LIST,Common Name,Real Address,Virtual Address,Virtual IPv6 Address,Bytes Received,Bytes Sent,Connected Since,Connected Since (time_t),Username,Client ID,Peer ID,Data Channel Cipher
CLIENT_LIST,ada@example.com:laptop,203.0.113.4:5555,172.25.0.6,,1048576,52428800,2026-10-05 19:00:00,1791244800,ada@example.com,0,0,AES-256-GCM
CLIENT_LIST,grace@example.com:Home desktop,198.51.100.7:6000,172.25.128.0,,2048,4096,2026-10-05 20:00:00,1791248400,grace@example.com,1,1,AES-256-GCM
HEADER,ROUTING_TABLE,Virtual Address,Common Name,Real Address,Last Ref,Last Ref (time_t)
ROUTING_TABLE,172.25.0.6,ada@example.com:laptop,203.0.113.4:5555,2026-10-05 20:09:58,1791248998
GLOBAL_STATS,Max bcast/mcast queue length,0
END
`), 0o644)

	got := ReadStatus(path)
	if len(got) != 2 {
		t.Fatalf("read %d clients, want 2: %v", len(got), got)
	}
	if ada := got["ada@example.com:laptop"]; ada.BytesReceived != 1048576 || ada.BytesSent != 52428800 {
		t.Errorf("ada = %+v", ada)
	}
	if grace := got["grace@example.com:Home desktop"]; grace.BytesSent != 4096 {
		t.Errorf("grace = %+v", grace)
	}

	if missing := ReadStatus(filepath.Join(t.TempDir(), "absent")); len(missing) != 0 {
		t.Errorf("a missing file read as %v", missing)
	}
}

func TestFastIOForUDPOnly(t *testing.T) {
	udp, _ := ServerConfig{BindPort: 1194, Protocol: "udp", Subnet: "10.8.0.0/24"}.Render()
	tcp, _ := ServerConfig{BindPort: 443, Protocol: "tcp", Subnet: "10.8.0.0/24"}.Render()
	if !strings.Contains(udp, "\nfast-io\n") {
		t.Error("a UDP server does not use fast-io")
	}
	// fast-io only applies to UDP; OpenVPN warns about it on TCP.
	if strings.Contains(tcp, "fast-io") {
		t.Error("a TCP server uses fast-io")
	}
}

func TestOffload(t *testing.T) {
	sysNet = t.TempDir()
	t.Cleanup(func() { sysNet = "/sys/class/net" })

	if _, known := Offload(); known {
		t.Error("offload was reported with no tunnel")
	}

	tunnel := filepath.Join(sysNet, Device)
	if err := os.MkdirAll(tunnel, 0o755); err != nil {
		t.Fatal(err)
	}
	if on, known := Offload(); !on || !known {
		t.Errorf("an offloaded tunnel = %v, %v", on, known)
	}

	// A classic tun device has tun_flags.
	if err := os.WriteFile(filepath.Join(tunnel, "tun_flags"), []byte("0x1001\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if on, known := Offload(); on || !known {
		t.Errorf("a classic tun device = %v, %v", on, known)
	}
}
