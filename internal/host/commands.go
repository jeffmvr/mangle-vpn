package host

import (
	"context"
	"log/slog"
	"os/exec"
	"strings"
)

// Run reports whether the program args[0], run with the remaining arguments,
// exited successfully. The arguments are passed to the program as they are,
// never through a shell, so no value can be read as shell syntax.
func Run(ctx context.Context, args ...string) bool {
	if len(args) == 0 {
		return false
	}

	var stderr strings.Builder
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		slog.Debug("command failed", "command", strings.Join(args, " "),
			"err", err, "stderr", strings.TrimSpace(stderr.String()))
		return false
	}
	return true
}

// Which returns the absolute path of the named executable, or an empty string
// when it cannot be found.
func Which(name string) string {
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return path
}
