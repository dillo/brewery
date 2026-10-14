package jobs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	Started  time.Time `json:"started"`
	Ended    time.Time `json:"ended"`
	Output   string    `json:"output"` // the last MaxOutput bytes it printed
}

// NewRecord records a finished job and the output it printed.
func NewRecord(j *Job, output string) Record {
	return Record{
		Title: j.Title, Args: j.Args, State: j.State, ExitCode: j.ExitCode,
		Started: j.Started, Ended: j.Ended, Output: Tail(output, MaxOutput),
	}
}

// Job rebuilds the job a record describes, for showing it again.
func (r Record) Job() *Job {
	return &Job{Title: r.Title, Args: r.Args, State: r.State, ExitCode: r.ExitCode, Started: r.Started, Ended: r.Ended}
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
type History struct{ Dir string }

// DefaultHistoryDir is $XDG_STATE_HOME/brewery/jobs, falling back to
// ~/.local/state/brewery/jobs.
func DefaultHistoryDir() string {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "brewery", "jobs")
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
	// Jobs that start within the clock's precision share a name: add a
	// suffix, which still sorts after the timestamp, rather than lose one.
	stem := filepath.Join(h.Dir, r.Started.UTC().Format("20060102T150405.000000000"))
	name := stem + ".json"
	for i := 1; ; i++ {
		if _, err := os.Lstat(name); os.IsNotExist(err) {
			break
		}
		name = fmt.Sprintf("%s-%d.json", stem, i)
	}
	if err := os.Rename(tmp, name); err != nil {
		os.Remove(tmp)
		return err
	}
	// A crash mid-save leaves a temp file behind.
	if stray, _ := filepath.Glob(filepath.Join(h.Dir, "*.tmp")); len(stray) > 0 {
		for _, s := range stray {
			os.Remove(s)
		}
	}
	files, err := h.files()
	if err != nil {
		return err
	}
	for _, f := range files[:max(len(files)-Keep, 0)] {
		os.Remove(f)
	}
	return nil
}

// Load reads the history, newest first. A missing directory is an empty
// history, and files that can't be read are skipped.
func (h History) Load() ([]Record, error) {
	files, err := h.files()
	if err != nil {
		return nil, err
	}
	var recs []Record
	for i := len(files) - 1; i >= 0; i-- {
		b, err := os.ReadFile(files[i])
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

// files lists the record files, oldest first.
func (h History) files() ([]string, error) {
	files, err := filepath.Glob(filepath.Join(h.Dir, "*.json"))
	sort.Strings(files)
	return files, err
}
