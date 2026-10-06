package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/host"
)

// stopTimeout is how long OpenVPN is given to exit after being asked,
// before it is killed. It tells connected clients it is going first.
const stopTimeout = 10 * time.Second

// VPNProcess runs OpenVPN as a child of this process, for a container,
// where there is no systemd to run it. It does what the systemd unit does:
// writes the configuration before starting OpenVPN, installs the firewall
// rules once it is up, and removes them once it has exited.
//
// It implements openvpn.Service.
type VPNProcess struct {
	a *App

	mu   sync.Mutex
	cmd  *exec.Cmd
	done chan struct{}
}

// NewVPNProcess returns a VPNProcess for the application.
func (a *App) NewVPNProcess() *VPNProcess {
	return &VPNProcess{a: a}
}

// Start starts OpenVPN unless it is already running.
func (p *VPNProcess) Start(ctx context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd != nil {
		return true
	}

	binary := host.Which("openvpn")
	if binary == "" {
		p.a.Log.Error("OpenVPN is not installed")
		return false
	}
	if err := p.a.WriteVPNServerConfig(ctx); err != nil {
		p.a.Log.Error("failed to write the OpenVPN configuration", "err", err)
		return false
	}

	// Not tied to ctx, which is usually a request's: OpenVPN runs until it
	// is stopped. It logs to its own file once started; anything before
	// that goes to this process's output.
	cmd := exec.Command(binary, "--config", p.a.Paths.OpenVPNConfig)
	cmd.Dir = p.a.Paths.Root
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		p.a.Log.Error("failed to start OpenVPN", "err", err)
		return false
	}

	done := make(chan struct{})
	p.cmd, p.done = cmd, done
	go p.wait(cmd, done)

	if err := p.a.InstallVPNFirewall(ctx); err != nil {
		p.a.Log.Error("failed to install the OpenVPN firewall rules", "err", err)
	}
	p.a.Log.Info("OpenVPN started", "pid", cmd.Process.Pid)
	return true
}

// wait reaps OpenVPN when it exits, however that happens, and takes its
// firewall rules down after it.
func (p *VPNProcess) wait(cmd *exec.Cmd, done chan struct{}) {
	defer close(done)
	err := cmd.Wait()

	p.mu.Lock()
	if p.cmd == cmd {
		p.cmd = nil
	}
	p.mu.Unlock()

	if err != nil && !isSignalled(err) {
		p.a.Log.Error("OpenVPN exited", "err", err)
	} else {
		p.a.Log.Info("OpenVPN stopped")
	}
	if err := p.a.RemoveVPNFirewall(context.Background()); err != nil {
		p.a.Log.Error("failed to remove the OpenVPN firewall rules", "err", err)
	}
}

// Stop stops OpenVPN and waits for it, and its firewall rules, to be gone.
func (p *VPNProcess) Stop(ctx context.Context) bool {
	p.mu.Lock()
	cmd, done := p.cmd, p.done
	p.mu.Unlock()
	if cmd == nil {
		return true
	}

	cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-done:
	case <-time.After(stopTimeout):
		p.a.Log.Warn("OpenVPN did not stop in time, killing it")
		cmd.Process.Kill()
		<-done
	}
	return true
}

// Restart stops OpenVPN, if it is running, and starts it again.
func (p *VPNProcess) Restart(ctx context.Context) bool {
	p.Stop(ctx)
	return p.Start(ctx)
}

// IsRunning reports whether OpenVPN is running.
func (p *VPNProcess) IsRunning(context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.cmd != nil
}

// isSignalled reports whether a process exited because of a signal, as it
// does when it is stopped.
func isSignalled(err error) bool {
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		return false
	}
	status, ok := exit.Sys().(syscall.WaitStatus)
	return ok && status.Signaled()
}
