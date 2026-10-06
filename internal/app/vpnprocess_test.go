package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fakeOpenVPN puts a stand-in for openvpn on the PATH: a script that runs
// until it is sent SIGTERM, or, with exitAtOnce, exits straight away as a
// server with a broken configuration would.
func fakeOpenVPN(t *testing.T, exitAtOnce bool) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\ntrap 'exit 0' TERM\nwhile true; do sleep 0.05; done\n"
	if exitAtOnce {
		script = "#!/bin/sh\nexit 1\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "openvpn"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newProcessApp(t *testing.T) *App {
	t.Helper()
	a, err := Open(t.Context(), t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

func TestVPNProcessStartsStopsAndRestarts(t *testing.T) {
	fakeOpenVPN(t, false)
	a := newProcessApp(t)
	ctx := t.Context()
	vpn := a.NewVPNProcess()
	t.Cleanup(func() { vpn.Stop(ctx) })

	if !vpn.Start(ctx) || !vpn.IsRunning(ctx) {
		t.Fatal("OpenVPN did not start")
	}
	if _, err := os.Stat(a.Paths.OpenVPNConfig); err != nil {
		t.Errorf("the configuration was not written before starting: %v", err)
	}
	// Starting what is running is a no-op, not a second server.
	first := vpn.cmd.Process.Pid
	vpn.Start(ctx)
	if vpn.cmd.Process.Pid != first {
		t.Error("a second OpenVPN was started")
	}

	if !vpn.Restart(ctx) || !vpn.IsRunning(ctx) || vpn.cmd.Process.Pid == first {
		t.Error("restarting did not start a new OpenVPN")
	}

	if !vpn.Stop(ctx) || vpn.IsRunning(ctx) {
		t.Error("OpenVPN did not stop")
	}
}

func TestVPNProcessNoticesAnExit(t *testing.T) {
	fakeOpenVPN(t, true)
	a := newProcessApp(t)
	ctx := t.Context()
	vpn := a.NewVPNProcess()

	vpn.Start(ctx)
	deadline := time.Now().Add(5 * time.Second)
	for vpn.IsRunning(ctx) && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if vpn.IsRunning(ctx) {
		t.Error("an OpenVPN that exited is still reported as running")
	}
}
