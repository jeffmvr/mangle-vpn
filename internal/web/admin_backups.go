package web

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"time"

	"github.com/jeffmvr/mangle-vpn/internal/config"
	"github.com/jeffmvr/mangle-vpn/internal/model"
)

// backupName is the form of a backup's file name, which is all a download
// may ask for, so it cannot reach any other file.
var backupName = regexp.MustCompile(`^mangle-\d{8}-\d{6}\.tar\.gz$`)

// backupFileDTO is one backup kept on the machine.
type backupFileDTO struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Created time.Time `json:"created"`
}

// backupsDTO is the state of the backups.
type backupsDTO struct {
	Last      string          `json:"last"`
	LastError string          `json:"last_error"`
	Offsite   bool            `json:"offsite"`
	Files     []backupFileDTO `json:"files"`
}

// adminListBackups returns the backups kept on the machine, newest first,
// and how the latest one went.
func (s *Server) adminListBackups(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Config.Reload(r.Context()); err != nil {
		s.serverError(w, "failed to reload settings", err)
		return
	}

	out := backupsDTO{
		Last:      s.app.Config.Get(config.BackupLast),
		LastError: s.app.Config.Get(config.BackupLastError),
		Offsite:   s.app.Config.Get(config.BackupS3Endpoint) != "",
		Files:     []backupFileDTO{},
	}

	entries, _ := os.ReadDir(s.app.Paths.Backups)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !backupName.MatchString(entry.Name()) {
			continue
		}
		out.Files = append(out.Files, backupFileDTO{Name: entry.Name(), Size: info.Size(), Created: info.ModTime().UTC()})
	}
	slices.SortFunc(out.Files, func(a, b backupFileDTO) int { return b.Created.Compare(a.Created) })

	s.writeJSON(w, http.StatusOK, out)
}

// adminRunBackup takes a backup now, as the nightly one does.
func (s *Server) adminRunBackup(w http.ResponseWriter, r *http.Request) {
	result, err := s.app.RunBackup(r.Context())
	if err != nil {
		s.writeJSON(w, http.StatusBadGateway, detail{"The backup failed: " + err.Error()})
		return
	}
	s.audit(r, model.EventAdminBackup, "Took a backup, %s.", result.File)
	s.writeJSON(w, http.StatusOK, result)
}

// adminDownloadBackup sends one of the backups kept on the machine.
func (s *Server) adminDownloadBackup(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !backupName.MatchString(name) {
		s.notFound(w)
		return
	}

	file, err := os.Open(filepath.Join(s.app.Paths.Backups, name))
	if err != nil {
		s.notFound(w)
		return
	}
	defer file.Close()

	s.audit(r, model.EventAdminBackup, "Downloaded the backup %s.", name)
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "no-store")
	info, _ := file.Stat()
	http.ServeContent(w, r, name, info.ModTime(), file)
}
