package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

// Keep is how many finished jobs the history holds.
const Keep = 100

// MaxOutput is how much of a job's output its record keeps: the end, where
// brew reports what happened.
const MaxOutput = 128 << 10

// Record is a finished job as the history keeps it.
type Record struct {
	Title    string    `json:"title"`
	Args     []string  `json:"args"`
	State    State     `json:"state"`
	ExitCode int       `json:"exit_code"`
	Takeover bool      `json:"takeover,omitempty"` // ran full screen
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`
	Output   string    `json:"output"` // the last MaxOutput bytes it printed
}

// NewRecord records a finished job and the output it printed.
func NewRecord(j *Job, output string) Record {
	return Record{
		Title: j.Title, Args: j.Args, State: j.State, ExitCode: j.ExitCode, Takeover: j.Takeover,
		Started: j.Started, Ended: j.Ended, Output: Tail(output, MaxOutput),
	}
}

// Tail is the end of s, at most n bytes, starting at a line when it can.
func Tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 0 {
		return ""
	}
	cut := len(s) - n
	for cut < len(s) && !utf8.RuneStart(s[cut]) {
		cut++ // never start inside a multi-byte character
	}
	midLine := s[cut-1] != '\n'
	s = s[cut:]
	if i := strings.IndexByte(s, '\n'); midLine && i >= 0 && i < len(s)-1 {
		s = s[i+1:]
	}
	return s
}

// MarshalText stores a state by name, so records survive reordering.
func (s State) MarshalText() ([]byte, error) { return []byte(s.String()), nil }

// UnmarshalText reads a state stored by name.
func (s *State) UnmarshalText(b []byte) error {
	for i := Queued; i <= Canceled; i++ {
		if i.String() == string(b) {
			*s = i
			return nil
		}
	}
	return fmt.Errorf("unknown job state %q", b)
}

// History stores finished jobs as one JSON file each in Dir, named by
// start time so they sort oldest first.
type History struct {
	Dir  string
	link func(oldname, newname string) error // os.Link; tests make it fail
}

// DefaultHistoryDir is $XDG_STATE_HOME/brewery/jobs, falling back to
// ~/.local/state/brewery/jobs.
func DefaultHistoryDir() string {
	return filepath.Join(DefaultStateDir(), "jobs")
}

// DefaultStateDir is $XDG_STATE_HOME/brewery, falling back to
// ~/.local/state/brewery when it's unset or not absolute. Without a home directory it uses the temp
// directory rather than a path relative to wherever brewery started.
func DefaultStateDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(dir) { // unset, or relative, which the XDG spec says to ignore
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			home = os.TempDir()
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "brewery")
}

// Save writes r and removes the oldest records beyond Keep.
func (h History) Save(r Record) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	// Records hold brew's output and typed args: keep them private.
	if err := os.MkdirAll(h.Dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(h.Dir, "*.tmp") // 0600
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(b)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		os.Remove(tmp)
		return werr
	}
	// Jobs that start within the clock's precision share a name. publish
	// fails if the name is taken, so another save is never replaced: try
	// "-1", "-2", … instead. (The order among records with the same start
	// time is unspecified.)
	stem := filepath.Join(h.Dir, r.Started.UTC().Format("20060102T150405.000000000"))
	name := stem + ".json"
	for i := 1; ; i++ {
		err := h.publish(tmp, name)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			os.Remove(tmp)
			return err
		}
		name = fmt.Sprintf("%s-%d.json", stem, i)
	}
	os.Remove(tmp) // still there after a link; already gone after a rename
	files, err := h.files()
	if err != nil {
		return err
	}
	for _, f := range files[:max(len(files)-Keep, 0)] {
		os.Remove(f)
	}
	return nil
}

// publish puts the file tmp at name, failing with fs.ErrExist if name is
// taken. A hard link does that atomically. Some file systems (network
// shares, FUSE) have no hard links: there it checks, then renames, which a
// save at the same instant could race.
func (h History) publish(tmp, name string) error {
	link := h.link
	if link == nil {
		link = os.Link
	}
	err := link(tmp, name)
	if !noHardLinks(err) {
		return err
	}
	switch _, err := os.Lstat(name); {
	case err == nil:
		return fs.ErrExist
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	return os.Rename(tmp, name)
}

// noHardLinks reports whether err says the file system can't make hard
// links.
func noHardLinks(err error) bool {
	for _, no := range []syscall.Errno{syscall.EPERM, syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.EXDEV} {
		if errors.Is(err, no) {
			return true
		}
	}
	return false
}

// Load reads the history, newest first. A missing directory is an empty
// history, and files that can't be read are skipped.
func (h History) Load() ([]Record, error) {
	h.sweepTemps()
	files, err := h.files()
	if err != nil {
		return nil, err
	}
	var recs []Record
	for _, file := range slices.Backward(files) {
		b, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		var r Record
		if json.Unmarshal(b, &r) == nil {
			recs = append(recs, r)
		}
	}
	return recs, nil
}

// sweepTemps removes temp files a crash left mid-save. A recent one may
// belong to a save in progress (another instance's), so it stays.
func (h History) sweepTemps() {
	stray, _ := filepath.Glob(filepath.Join(h.Dir, "*.tmp"))
	for _, f := range stray {
		if fi, err := os.Stat(f); err == nil && time.Since(fi.ModTime()) > time.Minute {
			os.Remove(f)
		}
	}
}

// files lists the record files, oldest first.
func (h History) files() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(h.Dir, "*.json"))
	sort.Strings(files)
	return files, err
}
