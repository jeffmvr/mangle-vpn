package web

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeLog(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.log")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTailLinesReturnsTheEnd(t *testing.T) {
	path := writeLog(t, "one\ntwo\r\nthree\nfour\n")

	lines, truncated, _, err := tailLines(path, 2)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lines, []string{"three", "four"}) {
		t.Errorf("lines = %q, want three, four", lines)
	}
	if !truncated {
		t.Error("truncated = false, but earlier lines were left out")
	}

	all, truncated, _, _ := tailLines(path, 10)
	if !slices.Equal(all, []string{"one", "two", "three", "four"}) {
		t.Errorf("lines = %q, want all four with the carriage return removed", all)
	}
	if truncated {
		t.Error("truncated = true for a whole file")
	}
}

func TestTailLinesDropsThePartialFirstLine(t *testing.T) {
	// More than maxLogTail bytes, so reading starts partway through a line.
	long := strings.Repeat("x", maxLogTail)
	path := writeLog(t, long+"\nlast\n")

	lines, truncated, _, err := tailLines(path, 10)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(lines, []string{"last"}) {
		t.Errorf("lines = %d entries, want just the last line", len(lines))
	}
	if !truncated {
		t.Error("truncated = false, but the start of the file was skipped")
	}
}

func TestTailLinesEmptyAndMissing(t *testing.T) {
	lines, _, _, err := tailLines(writeLog(t, ""), 10)
	if err != nil || lines == nil || len(lines) != 0 {
		t.Errorf("empty file: lines = %q, err = %v; want an empty, non-nil slice", lines, err)
	}

	_, _, _, err = tailLines(filepath.Join(t.TempDir(), "absent.log"), 10)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing file: err = %v, want fs.ErrNotExist", err)
	}
}
