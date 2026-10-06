package app

import (
	"context"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/jeffmvr/mangle-vpn/internal/config"
)

func TestBackupsGoOffTheMachineAndAreTrimmed(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()

	backend := s3mem.New()
	faker := gofakes3.New(backend)
	server := httptest.NewServer(faker.Server())
	defer server.Close()
	backend.CreateBucket("vpn-backups")

	for name, value := range map[string]string{
		config.BackupS3Endpoint: server.URL, config.BackupS3Bucket: "vpn-backups",
		config.BackupS3Prefix: "office", config.BackupS3AccessKey: "key", config.BackupS3SecretKey: "secret",
		config.BackupS3Region: "us-east-1", config.BackupKeep: "2",
	} {
		f.a.Config.Set(ctx, name, value)
	}

	// Three backups, as on three nights; each name carries its own second.
	for _, night := range []string{"20261001-030000", "20261002-030000", "20261003-030000"} {
		file := filepath.Join(f.a.Paths.Backups, "mangle-"+night+".tar.gz")
		if err := f.a.WriteBackupFile(ctx, file); err != nil {
			t.Fatal(err)
		}
		if err := f.a.uploadBackup(ctx, file, 2); err != nil {
			t.Fatal(err)
		}
	}

	// Something else in the bucket is no concern of the clean-up.
	client, _ := minio.New(server.Listener.Addr().String(), &minio.Options{Creds: credentials.NewStaticV4("key", "secret", "")})
	if _, err := client.PutObject(ctx, "vpn-backups", "office/notes.txt", strings.NewReader("hi"), 2, minio.PutObjectOptions{}); err != nil {
		t.Fatal(err)
	}

	result, err := f.a.RunBackup(ctx)
	if err != nil || !result.Offsite {
		t.Fatalf("RunBackup = %+v, %v", result, err)
	}

	var keys []string
	for object := range client.ListObjects(context.Background(), "vpn-backups", minio.ListObjectsOptions{Prefix: "office/", Recursive: true}) {
		keys = append(keys, object.Key)
	}
	want := []string{"office/" + result.File, "office/mangle-20261003-030000.tar.gz", "office/notes.txt"}
	if len(keys) != 3 || !contains(keys, want[0]) || !contains(keys, want[1]) || !contains(keys, want[2]) {
		t.Errorf("bucket holds %v, want %v", keys, want)
	}

	// The local copies are trimmed the same way.
	local, _ := filepath.Glob(filepath.Join(f.a.Paths.Backups, "mangle-*.tar.gz"))
	if len(local) != 2 {
		t.Errorf("%d local backups kept, want 2", len(local))
	}
	if f.a.Config.Get(config.BackupLastError) != "" || f.a.Config.Get(config.BackupLast) == "" {
		t.Error("the outcome was not recorded")
	}
}

func TestFailedBackupIsRecordedAndAlerted(t *testing.T) {
	f := newVPNFixture(t, false)
	ctx := t.Context()
	f.a.Config.Set(ctx, config.AlertWebhook, "https://hooks.example.com/abc")
	// A bucket that is not there.
	f.a.Config.Set(ctx, config.BackupS3Endpoint, "http://127.0.0.1:1")
	f.a.Config.Set(ctx, config.BackupS3Bucket, "missing")

	if _, err := f.a.RunBackup(ctx); err == nil {
		t.Fatal("a backup to an unreachable bucket reported success")
	}
	if f.a.Config.Get(config.BackupLastError) == "" {
		t.Error("the failure was not recorded")
	}
	if got := queuedWebhooks(t, f.a); len(got) != 1 {
		t.Errorf("alerts = %q, want one about the failed backup", got)
	}
	// The local backup was still written.
	if local, _ := filepath.Glob(filepath.Join(f.a.Paths.Backups, "mangle-*.tar.gz")); len(local) != 1 {
		t.Errorf("%d local backups, want 1", len(local))
	}
	os.RemoveAll(f.a.Paths.Backups)
}

func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
