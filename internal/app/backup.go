package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/paths"
	"github.com/jeffmvr/mangle-vpn/internal/store"
	"github.com/jeffmvr/mangle-vpn/internal/version"
)

// A backup is a gzipped tar of everything a server cannot rebuild for
// itself: a consistent snapshot of the database, the secret key that
// decrypts the secrets inside it, and the web server's certificate. The
// OpenVPN configuration, the revocation list, the systemd units and the
// logs are all regenerated, so they are left out.
//
// Paths in the archive are relative to the data directory, and a manifest
// says what made it.

// backupManifest is the first entry of every backup.
type backupManifest struct {
	Format  int       `json:"format"`
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

// backupFormat is the layout this release writes and reads.
const backupFormat = 1

// manifestName is the manifest's name in the archive.
const manifestName = "manifest.json"

// ScheduledBackupsKept is how many of the nightly backups are kept unless
// the settings say otherwise.
const ScheduledBackupsKept = 7

// backupFiles are the files a backup holds besides the database, relative
// to the data directory. Any that do not exist are skipped.
func backupFiles(p paths.Paths) []string {
	files := []string{rel(p, p.SecretKey), rel(p, p.WebCertificate), rel(p, p.WebPrivateKey)}

	// Let's Encrypt's account key and certificates, so a restored server
	// does not have to ask again.
	filepath.WalkDir(p.ACMECache, func(path string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			files = append(files, rel(p, path))
		}
		return nil
	})
	return files
}

// rel returns path relative to the data directory, with forward slashes.
func rel(p paths.Paths, path string) string {
	r, _ := filepath.Rel(p.Data, path)
	return filepath.ToSlash(r)
}

// Backup writes a backup of the installation to w.
func (a *App) Backup(ctx context.Context, w io.Writer) error {
	// The snapshot is taken into the data directory, which is private, and
	// removed once it is in the archive.
	snapshot := filepath.Join(a.Paths.Data, fmt.Sprintf(".backup-%d.db", time.Now().UnixNano()))
	if err := a.Store.SnapshotTo(ctx, snapshot); err != nil {
		return err
	}
	defer os.Remove(snapshot)

	gz := gzip.NewWriter(w)
	tw := tar.NewWriter(gz)

	manifest, _ := json.MarshalIndent(backupManifest{
		Format: backupFormat, Version: version.String(), Created: time.Now().UTC(),
	}, "", "  ")
	if err := writeTarEntry(tw, manifestName, manifest); err != nil {
		return err
	}
	if err := addTarFile(tw, rel(a.Paths, a.Paths.Database), snapshot); err != nil {
		return err
	}
	for _, name := range backupFiles(a.Paths) {
		err := addTarFile(tw, name, filepath.Join(a.Paths.Data, filepath.FromSlash(name)))
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}

// writeTarEntry adds a private file with the given contents.
func writeTarEntry(tw *tar.Writer, name string, content []byte) error {
	if err := tw.WriteHeader(&tar.Header{
		Name: name, Mode: 0o600, Size: int64(len(content)), ModTime: time.Now(),
		Typeflag: tar.TypeReg,
	}); err != nil {
		return err
	}
	_, err := tw.Write(content)
	return err
}

// addTarFile adds the file at path under the given name.
func addTarFile(tw *tar.Writer, name, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return writeTarEntry(tw, name, content)
}

// ScheduledBackup writes the nightly backup; see RunBackup.
func (a *App) ScheduledBackup(ctx context.Context) error {
	if !a.isInstalled(ctx) {
		return nil
	}
	// A failure is recorded and alerted on by RunBackup; the schedule
	// carries on either way.
	a.RunBackup(ctx)
	return nil
}

// isInstalled reports whether setup has finished, reading the setting
// fresh, since the task worker's copy may be stale.
func (a *App) isInstalled(ctx context.Context) bool {
	if err := a.Config.Reload(ctx); err != nil {
		return false
	}
	return a.Config.Bool(config.AppInstalled, false)
}

// WriteBackupFile writes a backup to the named file, readable only by its
// owner, replacing the file only once the backup is complete.
func (a *App) WriteBackupFile(ctx context.Context, name string) error {
	partial := name + ".partial"
	f, err := os.OpenFile(partial, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("app: create %s: %w", partial, err)
	}
	if err := a.Backup(ctx, f); err != nil {
		f.Close()
		os.Remove(partial)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(partial)
		return err
	}
	return os.Rename(partial, name)
}

// Restore replaces an installation's data with a backup. The services must
// be stopped. Whatever the backup replaces is moved aside, into a
// directory whose path is returned, rather than deleted.
//
// The backup is unpacked and checked in full before anything is moved, so
// a damaged archive leaves the installation as it was.
func Restore(ctx context.Context, p paths.Paths, r io.Reader) (string, error) {
	if err := p.EnsureDirs(); err != nil {
		return "", err
	}

	stamp := time.Now().UTC().Format("20060102-150405")
	staging := filepath.Join(p.Data, ".restore-"+stamp)
	if err := os.Mkdir(staging, 0o700); err != nil {
		return "", err
	}
	defer os.RemoveAll(staging)

	names, err := unpackBackup(r, staging)
	if err != nil {
		return "", err
	}
	if err := checkRestoredDatabase(ctx, p, staging); err != nil {
		return "", err
	}

	// Everything the backup replaces, plus the database's journal files,
	// which belong to the database being replaced.
	database := rel(p, p.Database)
	replaced := append(slices.Clone(names), database+"-wal", database+"-shm")
	// Let's Encrypt's cache is replaced whole, so no stale certificate
	// sits beside the restored ones.
	replaced = append(replaced, rel(p, p.ACMECache))

	aside := filepath.Join(p.Data, "pre-restore-"+stamp)
	for _, name := range replaced {
		if err := moveIfExists(filepath.Join(p.Data, filepath.FromSlash(name)), filepath.Join(aside, filepath.FromSlash(name))); err != nil {
			return aside, fmt.Errorf("app: move %s aside: %w", name, err)
		}
	}
	for _, name := range names {
		if err := moveIfExists(filepath.Join(staging, filepath.FromSlash(name)), filepath.Join(p.Data, filepath.FromSlash(name))); err != nil {
			return aside, fmt.Errorf("app: restore %s: %w", name, err)
		}
	}
	return aside, nil
}

// unpackBackup extracts a backup into dir, returning the names of the files
// it holds besides the manifest. Only the files a backup is made of are
// accepted, so an archive cannot write anywhere else.
func unpackBackup(r io.Reader, dir string) ([]string, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("app: not a backup: %w", err)
	}
	tr := tar.NewReader(gz)

	var names []string
	var manifest backupManifest
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("app: read the backup: %w", err)
		}
		if header.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("app: the backup holds %q, which is not a file", header.Name)
		}
		if !allowedBackupName(header.Name) {
			return nil, fmt.Errorf("app: the backup holds %q, which no backup does", header.Name)
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("app: read %s from the backup: %w", header.Name, err)
		}
		if header.Name == manifestName {
			if err := json.Unmarshal(content, &manifest); err != nil {
				return nil, fmt.Errorf("app: read the backup's manifest: %w", err)
			}
			continue
		}

		target := filepath.Join(dir, filepath.FromSlash(header.Name))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return nil, err
		}
		if err := os.WriteFile(target, content, 0o600); err != nil {
			return nil, err
		}
		names = append(names, header.Name)
	}

	switch {
	case manifest.Format == 0:
		return nil, errors.New("app: the backup has no manifest")
	case manifest.Format > backupFormat:
		return nil, fmt.Errorf("app: the backup was made by a newer release (%s)", manifest.Version)
	case !slices.Contains(names, "mangle.db") || !slices.Contains(names, "keys/secret.key"):
		return nil, errors.New("app: the backup lacks the database or the secret key")
	}
	return names, nil
}

// allowedBackupName reports whether name is one a backup may hold: the
// manifest, the database, or a file under keys/, with no way out of the
// directory it is unpacked into.
func allowedBackupName(name string) bool {
	if name == manifestName || name == "mangle.db" {
		return true
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(name)))
	return clean == name && strings.HasPrefix(name, "keys/") && !strings.Contains(name, "..") &&
		!filepath.IsAbs(filepath.FromSlash(name))
}

// checkRestoredDatabase opens the unpacked database with the unpacked
// secret key, which brings its schema up to date and proves the key can
// read the secrets in it.
func checkRestoredDatabase(ctx context.Context, p paths.Paths, dir string) error {
	secret, err := os.ReadFile(filepath.Join(dir, "keys", "secret.key"))
	if err != nil {
		return err
	}

	st, err := store.OpenContext(ctx, filepath.Join(dir, "mangle.db"))
	if err != nil {
		return fmt.Errorf("app: the backup's database: %w", err)
	}
	defer st.Close()

	if err := st.UseSecretKey(secret, nil); err != nil {
		return err
	}
	if _, err := st.Settings.All(ctx); err != nil {
		return fmt.Errorf("app: the backup's secret key does not match its database: %w", err)
	}
	return nil
}

// moveIfExists renames from to to, creating to's directory, and does
// nothing when from does not exist.
func moveIfExists(from, to string) error {
	if _, err := os.Lstat(from); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(to), 0o700); err != nil {
		return err
	}
	return os.Rename(from, to)
}
