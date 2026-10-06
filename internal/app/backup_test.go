package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
)

func TestBackupRestoresOntoAnotherServer(t *testing.T) {
	ctx := t.Context()
	quiet := slog.New(slog.NewTextHandler(io.Discard, nil))

	source, err := Open(ctx, t.TempDir(), quiet)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	// Secrets are encrypted with the source's key, so a restore without
	// that key would lose them.
	source.CreateAuthority(ctx)
	group := &model.Group{Name: "Staff", IsEnabled: true, MaxDevices: 1}
	source.Store.Groups.Save(ctx, group)
	source.Store.Users.Save(ctx, &model.User{Email: "ada@example.com", GroupID: group.ID, IsEnabled: true})
	os.WriteFile(source.Paths.WebCertificate, []byte("certificate"), 0o600)

	var archive bytes.Buffer
	if err := source.Backup(ctx, &archive); err != nil {
		t.Fatal(err)
	}

	// The other server already has an installation of its own.
	root := t.TempDir()
	other, err := Open(ctx, root, quiet)
	if err != nil {
		t.Fatal(err)
	}
	other.Close()

	p, _ := paths.New(root)
	aside, err := Restore(ctx, p, bytes.NewReader(archive.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(aside, "mangle.db")); err != nil {
		t.Errorf("the replaced database was not kept: %v", err)
	}

	restored, err := Open(ctx, root, quiet)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()

	if _, err := restored.Store.Users.ByEmail(ctx, "ada@example.com"); err != nil {
		t.Errorf("the user did not come across: %v", err)
	}
	if restored.Config.Get(config.CAPrivateKey) != source.Config.Get(config.CAPrivateKey) {
		t.Error("the certificate authority's key did not come across readable")
	}
	if got, _ := os.ReadFile(restored.Paths.WebCertificate); string(got) != "certificate" {
		t.Errorf("web certificate = %q", got)
	}
}

// archiveOf builds a backup-shaped archive from name and content pairs.
func archiveOf(t *testing.T, entries ...string) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for i := 0; i < len(entries); i += 2 {
		writeTarEntry(tw, entries[i], []byte(entries[i+1]))
	}
	tw.Close()
	gz.Close()
	return &buf
}

func TestRestoreRefusesArchivesThatAreNotBackups(t *testing.T) {
	root := t.TempDir()
	p, _ := paths.New(root)
	manifest := `{"format": 1, "version": "test"}`

	for name, archive := range map[string]*bytes.Buffer{
		"escape":      archiveOf(t, manifestName, manifest, "../outside", "x", "mangle.db", "", "keys/secret.key", "k"),
		"sneaky keys": archiveOf(t, manifestName, manifest, "keys/../../outside", "x", "mangle.db", "", "keys/secret.key", "k"),
		"other file":  archiveOf(t, manifestName, manifest, "systemd/mangle-web.service", "x", "mangle.db", "", "keys/secret.key", "k"),
		"no manifest": archiveOf(t, "mangle.db", "", "keys/secret.key", "k"),
		"no key":      archiveOf(t, manifestName, manifest, "mangle.db", ""),
		"newer":       archiveOf(t, manifestName, `{"format": 99, "version": "9.9"}`, "mangle.db", "", "keys/secret.key", "k"),
		"not gzip":    bytes.NewBufferString("hello"),
	} {
		_, err := Restore(t.Context(), p, archive)
		if err == nil {
			t.Errorf("%s: restored", name)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(root), "outside")); err == nil {
		t.Error("an archive wrote outside the data directory")
	}
	if entries, _ := os.ReadDir(p.Data); hasPrefix(entries, "pre-restore-") {
		t.Error("a refused archive moved the installation aside")
	}
}

func TestRestoreRefusesAKeyThatDoesNotMatch(t *testing.T) {
	ctx := t.Context()
	source, _ := Open(ctx, t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer source.Close()
	source.CreateAuthority(ctx)

	var archive bytes.Buffer
	source.Backup(ctx, &archive)

	// Swap the secret key in the archive for another.
	gz, _ := gzip.NewReader(&archive)
	tr := tar.NewReader(gz)
	var entries []string
	for {
		header, err := tr.Next()
		if err != nil {
			break
		}
		content, _ := io.ReadAll(tr)
		if header.Name == "keys/secret.key" {
			content = []byte(strings.Repeat("x", 50))
		}
		entries = append(entries, header.Name, string(content))
	}

	p, _ := paths.New(t.TempDir())
	if _, err := Restore(ctx, p, archiveOf(t, entries...)); err == nil || !strings.Contains(err.Error(), "secret key does not match") {
		t.Logf("err = %v", err)
		t.Error("restored a database with a key that cannot read it")
	}
}

func hasPrefix(entries []os.DirEntry, prefix string) bool {
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix) {
			return true
		}
	}
	return false
}
