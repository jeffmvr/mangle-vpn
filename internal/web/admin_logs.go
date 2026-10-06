package web

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Bounds on how much of a log one request returns.
const (
	defaultLogLines = 500
	maxLogLines     = 2000

	// maxLogTail is how far back from the end of a file a request reads.
	// It bounds the work done for a log with very long lines; the lines
	// asked for are taken from within it.
	maxLogTail = 1 << 20
)

// logDTO is the end of one log file.
type logDTO struct {
	Name      string     `json:"name"`
	Lines     []string   `json:"lines"`
	Truncated bool       `json:"truncated"`
	UpdatedAt *time.Time `json:"updated_at"`
}

// logPath returns the file behind a log name the API accepts.
func (s *Server) logPath(name string) (string, bool) {
	switch name {
	case "app":
		return s.app.Paths.AppLog, true
	case "openvpn":
		return s.app.Paths.OpenVPNLog, true
	}
	return "", false
}

// adminReadLog returns the last lines of the application or OpenVPN log, newest last (?lines=N, at most maxLogLines).
//
// A log that does not exist yet, such as OpenVPN's before the server first
// starts, is returned empty rather than as an error.
func (s *Server) adminReadLog(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	path, ok := s.logPath(name)
	if !ok {
		s.notFound(w)
		return
	}

	count := defaultLogLines
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			s.invalidField(w, "lines", "Must be a positive whole number.")
			return
		}
		count = min(n, maxLogLines)
	}

	out := logDTO{Name: name, Lines: []string{}}

	lines, truncated, modified, err := tailLines(path, count)
	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		s.serverError(w, "failed to read a log", err)
		return
	default:
		out.Lines, out.Truncated, out.UpdatedAt = lines, truncated, &modified
	}

	s.writeJSON(w, http.StatusOK, out)
}

// tailLines returns up to n of the last lines of a file, reading no more
// than maxLogTail bytes from its end. truncated reports whether the file
// holds earlier lines than those returned.
func tailLines(path string, n int) (lines []string, truncated bool, modified time.Time, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, false, time.Time{}, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, false, time.Time{}, err
	}

	offset := max(info.Size()-maxLogTail, 0)
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil && !errors.Is(err, io.EOF) {
		return nil, false, time.Time{}, err
	}

	// Starting mid-file almost always lands inside a line, so the first
	// partial line is dropped rather than shown cut off.
	if offset > 0 {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}

	buf = bytes.TrimRight(buf, "\n")
	if len(buf) == 0 {
		return []string{}, offset > 0, info.ModTime().UTC(), nil
	}

	all := bytes.Split(buf, []byte("\n"))
	truncated = offset > 0 || len(all) > n
	if len(all) > n {
		all = all[len(all)-n:]
	}

	lines = make([]string, len(all))
	for i, line := range all {
		lines[i] = string(bytes.TrimRight(line, "\r"))
	}
	return lines, truncated, info.ModTime().UTC(), nil
}
