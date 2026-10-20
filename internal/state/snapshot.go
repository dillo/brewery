package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
)

// snapshotVersion changes whenever Snapshot's shape changes; older cache
// files are ignored rather than half-loaded.
const snapshotVersion = 7

// Snapshot is everything brewery knows about the installation at one time.
type Snapshot struct {
	Version         int                  `json:"version"`
	Packages        []brew.Package       `json:"packages"`
	Outdated        []brew.Outdated      `json:"outdated"`
	Services        []brew.Service       `json:"services"`
	Taps            []brew.Tap           `json:"taps"`
	Doctor          []brew.DoctorWarning `json:"doctor"`
	DoctorRan       bool                 `json:"doctor_ran"`
	Cleanup         brew.Cleanup         `json:"cleanup"`
	CleanupRan      bool                 `json:"cleanup_ran"`
	Autoremove      []string             `json:"autoremove"` // formulae brew autoremove would uninstall
	AutoremoveRan   bool                 `json:"autoremove_ran"`
	ServiceFiles    []string             `json:"service_files"` // brew's launchd files, by full path
	ServiceFilesRan bool                 `json:"service_files_ran"`
	TapChecks       []github.TapCheck    `json:"tap_checks"`      // what the last tap check found, by tap
	TapsCheckedAt   time.Time            `json:"taps_checked_at"` // when it finished; zero if never
	Sizes           map[string]int64     `json:"sizes"`           // bytes on disk, by Package.Key
	DataAt          time.Time            `json:"data_at"`         // when brew last fetched package data
	FetchedAt       time.Time            `json:"fetched_at"`
}

// DefaultCachePath is $XDG_CACHE_HOME/brewery/snapshot.json, falling back
// to ~/.cache/brewery/snapshot.json when it's unset or not absolute. Without a home directory it uses the
// temp directory rather than a path relative to wherever brewery started.
func DefaultCachePath() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if !filepath.IsAbs(dir) { // unset, or relative, which the XDG spec says to ignore
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "brewery", "snapshot.json")
}

// Load reads a cached snapshot. A missing, corrupt or outdated file returns
// an empty Snapshot and an error the caller may ignore.
func Load(path string) (Snapshot, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- our own cache file
	if err != nil {
		return Snapshot{}, err
	}
	var s Snapshot
	if err := json.Unmarshal(b, &s); err != nil {
		return Snapshot{}, fmt.Errorf("read snapshot cache: %w", err)
	}
	if s.Version != snapshotVersion {
		return Snapshot{}, fmt.Errorf("snapshot cache version %d, want %d", s.Version, snapshotVersion)
	}
	return s, nil
}

// Save writes the snapshot atomically (temp file + rename).
func Save(path string, s Snapshot) error {
	s.Version = snapshotVersion
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "snapshot-*.json")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		_ = os.Remove(tmp.Name())
		return err
	}
	return nil
}
