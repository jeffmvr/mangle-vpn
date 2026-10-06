package app

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"gopkg.in/natefinch/lumberjack.v2"
)

// Log rotation, matching what the Django release configured. A VPN gateway
// runs for a long time between touches, so the logs have to have a ceiling.
const (
	// logMaxSizeMB is how large a log file grows before it is rotated.
	logMaxSizeMB = 10

	// logBackups is how many rotated files are kept.
	logBackups = 10

	// logMaxAgeDays is how long a rotated file is kept, whatever the count.
	logMaxAgeDays = 90
)

// NewLogger returns a logger writing to the given file, rotating it as it
// grows.
//
// Output also goes to standard error, so that running a command by hand
// shows what it did while systemd's journal and the log file both keep a
// copy. An unopenable file leaves standard error as the only destination
// rather than failing the command.
func NewLogger(path string, level slog.Level) *slog.Logger {
	return slog.New(newFileHandler(path, level))
}

// newFileHandler builds the handler behind [NewLogger].
func newFileHandler(path string, level slog.Level) slog.Handler {
	options := &slog.HandlerOptions{Level: level}

	if path == "" {
		return slog.NewTextHandler(os.Stderr, options)
	}

	// lumberjack creates the file itself, but not the directory holding it.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		handler := slog.NewTextHandler(os.Stderr, options)
		slog.New(handler).Warn("falling back to stderr logging", "file", path, "err", err)
		return handler
	}

	rotating := &lumberjack.Logger{
		Filename:   path,
		MaxSize:    logMaxSizeMB,
		MaxBackups: logBackups,
		MaxAge:     logMaxAgeDays,
		Compress:   true,
	}

	return slog.NewTextHandler(io.MultiWriter(os.Stderr, rotating), options)
}
