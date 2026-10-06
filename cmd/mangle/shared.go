package main

import (
	"context"
	"flag"
	"log/slog"
	"path/filepath"

	"github.com/jeffmvr/mangle-vpn/internal/app"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
)

// commonFlags are the flags every command accepts.
type commonFlags struct {
	root    string
	logFile string
	verbose bool
}

// bind registers the common flags on a flag set.
func (f *commonFlags) bind(fs *flag.FlagSet, defaultLogFile string) {
	fs.StringVar(&f.root, "root", "",
		"installation directory (defaults to the directory holding this binary)")
	fs.StringVar(&f.logFile, "log", defaultLogFile,
		"log file, relative to the data directory; empty logs only to stderr")
	fs.BoolVar(&f.verbose, "v", false, "log at debug level")
}

// level returns the log level the flags ask for.
func (f *commonFlags) level() slog.Level {
	if f.verbose {
		return slog.LevelDebug
	}
	return slog.LevelInfo
}

// openApp parses a command's flags and prepares the application for it.
// defaultLog is the log file used unless -log says otherwise.
func openApp(ctx context.Context, name, defaultLog string, args []string) (*app.App, error) {
	a, _, err := openAppArgs(ctx, name, defaultLog, args)
	return a, err
}

// openAppArgs is openApp for a command that also takes arguments after its
// flags, which it returns.
func openAppArgs(ctx context.Context, name, defaultLog string, args []string) (*app.App, []string, error) {
	var flags commonFlags

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.bind(fs, defaultLog)
	if err := fs.Parse(args); err != nil {
		return nil, nil, err
	}
	a, err := flags.open(ctx)
	return a, fs.Args(), err
}

// open prepares the application for a command.
//
// The logger is built before the application so that failures during start
// up are recorded the same way as everything else.
func (f *commonFlags) open(ctx context.Context) (*app.App, error) {
	paths, err := resolveLogPath(f.root, f.logFile)
	if err != nil {
		return nil, err
	}

	return app.Open(ctx, f.root, app.NewLogger(paths, f.level()))
}

// resolveLogPath turns the log flag into an absolute path under the data
// directory, or an empty string when logging only to stderr.
func resolveLogPath(root, name string) (string, error) {
	if name == "" {
		return "", nil
	}

	p, err := paths.New(root)
	if err != nil {
		return "", err
	}
	return filepath.Join(p.Logs, name), nil
}
