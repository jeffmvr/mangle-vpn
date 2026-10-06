package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/paths"
)

func TestSystemdUnitsRetireTheTaskWorker(t *testing.T) {
	p, err := paths.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(p.Systemd, 0o755); err != nil {
		t.Fatal(err)
	}
	retired := filepath.Join(p.Systemd, "mangle-tasks.service")
	if err := os.WriteFile(retired, []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// systemctl is absent or refuses here; the units are written regardless.
	if err := SystemdUnits(t.Context(), p); err != nil {
		t.Fatalf("SystemdUnits: %v", err)
	}

	if _, err := os.Stat(retired); !os.IsNotExist(err) {
		t.Error("the old task worker unit was left behind")
	}
	for _, unit := range []string{p.WebUnit, p.VPNUnit} {
		if _, err := os.Stat(unit); err != nil {
			t.Errorf("%s was not written: %v", filepath.Base(unit), err)
		}
	}
	vpn, _ := os.ReadFile(p.VPNUnit)
	for _, module := range []string{"ovpn-dco-v2", "ovpn"} {
		if !strings.Contains(string(vpn), "modprobe -q "+module+"\n") {
			t.Errorf("the VPN unit does not load %s", module)
		}
	}

	web, _ := os.ReadFile(p.WebUnit)
	if len(web) == 0 || strings.Contains(string(web), "mangle-tasks") {
		t.Errorf("the web unit still depends on the task worker:\n%s", web)
	}
}
