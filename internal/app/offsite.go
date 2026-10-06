package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Backups are copied off the machine to an S3-compatible bucket: Amazon S3,
// Backblaze B2, Cloudflare R2, Wasabi, MinIO and the like. A backup on the
// disk it protects is no use when the disk goes.

// offsiteTimeout bounds one upload, and the clean-up after it.
const offsiteTimeout = 10 * time.Minute

// ErrNoOffsite is returned when no bucket is set up.
var ErrNoOffsite = errors.New("app: no bucket is set up for backups")

// offsite is where the off-machine copies go.
type offsite struct {
	client *minio.Client
	bucket string
	prefix string
}

// offsiteTarget returns the configured bucket, or ErrNoOffsite.
func (a *App) offsiteTarget() (*offsite, error) {
	endpoint := a.Config.Get(config.BackupS3Endpoint)
	if endpoint == "" {
		return nil, ErrNoOffsite
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("app: the backup endpoint %q is not a URL", endpoint)
	}

	client, err := minio.New(u.Host, &minio.Options{
		Creds: credentials.NewStaticV4(a.Config.Get(config.BackupS3AccessKey),
			a.Config.Get(config.BackupS3SecretKey), ""),
		Secure: u.Scheme == "https",
		Region: a.Config.Get(config.BackupS3Region),
	})
	if err != nil {
		return nil, fmt.Errorf("app: the backup bucket: %w", err)
	}
	return &offsite{
		client: client,
		bucket: a.Config.Get(config.BackupS3Bucket),
		prefix: strings.Trim(a.Config.Get(config.BackupS3Prefix), "/"),
	}, nil
}

// key returns the object name a backup file is stored under.
func (o *offsite) key(name string) string {
	return path.Join(o.prefix, name)
}

// uploadBackup copies a backup file to the bucket, then removes all but the
// newest keep backups there.
func (a *App) uploadBackup(ctx context.Context, file string, keep int) error {
	target, err := a.offsiteTarget()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, offsiteTimeout)
	defer cancel()

	if _, err := target.client.FPutObject(ctx, target.bucket, target.key(filepath.Base(file)), file,
		minio.PutObjectOptions{ContentType: "application/gzip"}); err != nil {
		return fmt.Errorf("app: upload the backup: %w", err)
	}

	// Only this application's backups are considered, by their names, so
	// anything else kept under the same prefix is left alone.
	var names []string
	listPrefix := target.key("mangle-")
	for object := range target.client.ListObjects(ctx, target.bucket, minio.ListObjectsOptions{Prefix: listPrefix}) {
		if object.Err != nil {
			return fmt.Errorf("app: list the bucket: %w", object.Err)
		}
		if strings.HasSuffix(object.Key, ".tar.gz") {
			names = append(names, object.Key)
		}
	}
	slices.Sort(names)
	for len(names) > keep {
		if err := target.client.RemoveObject(ctx, target.bucket, names[0], minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("app: remove an old backup from the bucket: %w", err)
		}
		names = names[1:]
	}
	return nil
}

// BackupResult is the outcome of the latest backup.
type BackupResult struct {
	File    string    `json:"file"`
	At      time.Time `json:"at"`
	Offsite bool      `json:"offsite"`
	Error   string    `json:"error"`
}

// RunBackup writes a backup into the backups directory, keeps the newest
// ones as the settings ask, copies it off the machine when a bucket is set
// up, and records how that went.
func (a *App) RunBackup(ctx context.Context) (BackupResult, error) {
	result := BackupResult{At: time.Now().UTC()}
	keep := a.Config.Int(config.BackupKeep, ScheduledBackupsKept)

	name := filepath.Join(a.Paths.Backups, "mangle-"+result.At.Format("20060102-150405")+".tar.gz")
	err := a.WriteBackupFile(ctx, name)
	if err == nil {
		result.File = filepath.Base(name)
		err = a.pruneBackups(keep)
	}
	if err == nil {
		err = a.uploadBackup(ctx, name, keep)
		result.Offsite = err == nil
		if errors.Is(err, ErrNoOffsite) {
			err = nil
		}
	}

	if err != nil {
		result.Error = err.Error()
	}
	a.recordBackup(ctx, result)
	return result, err
}

// pruneBackups removes all but the newest keep backups from the backups
// directory.
func (a *App) pruneBackups(keep int) error {
	entries, err := filepath.Glob(filepath.Join(a.Paths.Backups, "mangle-*.tar.gz"))
	if err != nil {
		return err
	}
	// The names sort by when they were made.
	slices.Sort(entries)
	for len(entries) > keep {
		if err := os.Remove(entries[0]); err != nil {
			return err
		}
		entries = entries[1:]
	}
	return nil
}

// recordBackup stores the outcome of a backup for the settings page, and
// alerts when it failed.
func (a *App) recordBackup(ctx context.Context, result BackupResult) {
	if err := a.Config.Set(ctx, config.BackupLast, result.At.Format(time.RFC3339)); err != nil {
		a.Log.Error("failed to record a backup", "err", err)
	}
	if err := a.Config.Set(ctx, config.BackupLastError, result.Error); err != nil {
		a.Log.Error("failed to record a backup", "err", err)
	}
	if result.Error != "" {
		a.Log.Error("backup failed", "err", result.Error)
		a.Alert(ctx, "", "The backup failed", "Last night's backup did not complete: "+result.Error)
	} else {
		a.Log.Info("backup written", "file", result.File, "offsite", result.Offsite)
	}
}
