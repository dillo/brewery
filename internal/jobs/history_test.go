package jobs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unicode/utf8"
)

func record(title string, started time.Time) Record {
	return Record{Title: title, Args: strings.Fields(title), State: Done, Started: started, Ended: started.Add(time.Second)}
}

func TestHistorySavesAndLoadsNewestFirst(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")} // created on first save
	a := record("upgrade fd", t0)
	b := record("pin git", t0.Add(time.Minute))
	b.State, b.ExitCode, b.Output = Failed, 1, "Error: no such keg\n"
	for _, r := range []Record{a, b} {
		if err := h.Save(r); err != nil {
			t.Fatal(err)
		}
	}
	got, err := h.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Title != "pin git" || got[1].Title != "upgrade fd" {
		t.Fatalf("want newest first, got %+v", got)
	}
	if !reflect.DeepEqual(got[0], b) {
		t.Errorf("round trip changed the record:\n got %+v\nwant %+v", got[0], b)
	}
}

func TestHistoryKeepsTheNewest(t *testing.T) {
	h := History{Dir: t.TempDir()}
	for i := range Keep + 3 {
		if err := h.Save(record("job", t0.Add(time.Duration(i)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := h.Load()
	if len(got) != Keep {
		t.Fatalf("kept %d records, want %d", len(got), Keep)
	}
	if want := t0.Add((Keep + 2) * time.Second); !got[0].Started.Equal(want) {
		t.Errorf("newest record started %v, want %v", got[0].Started, want)
	}
	if want := t0.Add(3 * time.Second); !got[Keep-1].Started.Equal(want) {
		t.Errorf("oldest kept record started %v, want %v (the 3 oldest are dropped)", got[Keep-1].Started, want)
	}
}

func TestHistoryLoadSkipsBadFilesAndMissingDir(t *testing.T) {
	got, err := History{Dir: filepath.Join(t.TempDir(), "none")}.Load()
	if err != nil || len(got) != 0 {
		t.Errorf("a missing history is empty, got %v %v", got, err)
	}
	h := History{Dir: t.TempDir()}
	if err := h.Save(record("upgrade fd", t0)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(h.Dir, "20990101T000000.000000000.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = h.Load()
	if err != nil || len(got) != 1 || got[0].Title != "upgrade fd" {
		t.Errorf("a corrupt file should be skipped: %+v %v", got, err)
	}
}

func TestStateIsStoredByName(t *testing.T) {
	b, _ := Canceled.MarshalText()
	if string(b) != "canceled" {
		t.Errorf("Canceled is stored as %q", b)
	}
	var s State
	if err := s.UnmarshalText([]byte("failed")); err != nil || s != Failed {
		t.Errorf("read %v, %v", s, err)
	}
	if err := s.UnmarshalText([]byte("exploded")); err == nil {
		t.Error("an unknown state should be an error")
	}
}

func TestNewRecordKeepsTheEndOfTheOutput(t *testing.T) {
	j := &Job{Title: "upgrade fd", Args: []string{"upgrade", "fd"}, State: Done, Started: t0, Ended: t0.Add(time.Second)}
	out := strings.Repeat("progress line\n", MaxOutput/14+10) + "==> fd 10.5.0 installed\n"
	r := NewRecord(j, out)
	if len(r.Output) > MaxOutput || !strings.HasSuffix(r.Output, "==> fd 10.5.0 installed\n") {
		t.Errorf("record output is %d bytes and ends %q", len(r.Output), r.Output[max(len(r.Output)-30, 0):])
	}
	if !strings.HasPrefix(r.Output, "progress line\n") {
		t.Errorf("the kept output should start at a line: %q", r.Output[:20])
	}
}

func TestTail(t *testing.T) {
	for _, tt := range []struct {
		s    string
		n    int
		want string
	}{
		{"short", 10, "short"},
		{"one\ntwo\nthree", 7, "three"},      // cut mid-line, then skip to the next line
		{"one\ntwo\nthree", 9, "two\nthree"}, // the cut is at a line start: keep it
		{"aaaaaaaaaaaa", 4, "aaaa"},          // no newline to start at
		{"abc\n", 2, "c\n"},                  // the only newline ends the text
	} {
		if got := Tail(tt.s, tt.n); got != tt.want {
			t.Errorf("Tail(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestTailKeepsRunesWhole(t *testing.T) {
	s := strings.Repeat("é", 10) // 2 bytes each
	for n := 1; n < len(s); n++ {
		if got := Tail(s, n); !utf8.ValidString(got) {
			t.Errorf("Tail(n=%d) = %q, not valid UTF-8", n, got)
		}
	}
	if got := Tail("abc", 0); got != "" {
		t.Errorf("Tail(s, 0) = %q, want empty", got)
	}
	if got := Tail("abc", -1); got != "" {
		t.Errorf("Tail(s, -1) = %q, want empty", got)
	}
}

func TestHistoryKeepsRecordsWithTheSameStart(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")}
	for _, title := range []string{"one", "two", "three"} {
		if err := h.Save(record(title, t0)); err != nil {
			t.Fatal(err)
		}
	}
	got, err := h.Load()
	if err != nil || len(got) != 3 {
		t.Fatalf("Load = %d records, %v; want 3", len(got), err)
	}
}

func TestHistoryWithoutHardLinks(t *testing.T) {
	for _, errno := range []syscall.Errno{syscall.EPERM, syscall.ENOTSUP, syscall.EOPNOTSUPP, syscall.EXDEV} {
		h := History{
			Dir:  filepath.Join(t.TempDir(), "jobs"),
			link: func(o, n string) error { return &os.LinkError{Op: "link", Old: o, New: n, Err: errno} },
		}
		for _, title := range []string{"one", "two"} {
			if err := h.Save(record(title, t0)); err != nil {
				t.Fatalf("%v: %v", errno, err)
			}
		}
		got, err := h.Load()
		if err != nil || len(got) != 2 {
			t.Errorf("%v: Load = %d records, %v; want 2", errno, len(got), err)
		}
		if left, _ := filepath.Glob(filepath.Join(h.Dir, "*.tmp")); len(left) != 0 {
			t.Errorf("%v: temp files left: %v", errno, left)
		}
	}
}

func TestHistoryLinkFailureIsAnError(t *testing.T) {
	h := History{
		Dir:  filepath.Join(t.TempDir(), "jobs"),
		link: func(o, n string) error { return &os.LinkError{Op: "link", Old: o, New: n, Err: syscall.EIO} },
	}
	if err := h.Save(record("one", t0)); !errors.Is(err, syscall.EIO) {
		t.Errorf("Save = %v, want the I/O error", err)
	}
	if left, _ := os.ReadDir(h.Dir); len(left) != 0 {
		t.Errorf("files left: %v", left)
	}
}

func TestHistoryFilesArePrivate(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")}
	if err := h.Save(record("pin git", t0)); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(h.Dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v; want 0700", fi.Mode().Perm(), err)
	}
	files, _ := h.files()
	if fi, err := os.Stat(files[0]); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, %v; want 0600", fi.Mode().Perm(), err)
	}
}

func TestHistorySaveLeavesTempFilesAndLoadSweepsOldOnes(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")}
	if err := os.MkdirAll(h.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	inflight := filepath.Join(h.Dir, "inflight.tmp") // another save, mid-write
	stray := filepath.Join(h.Dir, "stray.tmp")       // left by a crash
	for _, f := range []string{inflight, stray} {
		if err := os.WriteFile(f, []byte("half"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(stray, old, old); err != nil {
		t.Fatal(err)
	}
	if err := h.Save(record("pin git", t0)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inflight); err != nil {
		t.Errorf("Save must not touch another save's temp file: %v", err)
	}
	if _, err := h.Load(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("Load should remove an old stray temp file: %v", err)
	}
	if _, err := os.Stat(inflight); err != nil {
		t.Errorf("Load must keep a fresh temp file: %v", err)
	}
}

func TestHistoryConcurrentSavesWithTheSameStartAllSurvive(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")}
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			if err := h.Save(record(fmt.Sprintf("job %d", i), t0)); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	got, err := h.Load()
	if err != nil || len(got) != 8 {
		t.Fatalf("Load = %d records, %v; want 8", len(got), err)
	}
}

func TestDefaultStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/xdg")
	if got := DefaultStateDir(); got != "/tmp/xdg/brewery" {
		t.Errorf("got %s", got)
	}
	if got := DefaultHistoryDir(); got != "/tmp/xdg/brewery/jobs" {
		t.Errorf("got %s", got)
	}
	t.Setenv("XDG_STATE_HOME", "")
	t.Setenv("HOME", "")
	if got := DefaultStateDir(); !filepath.IsAbs(got) || !strings.HasPrefix(got, os.TempDir()) {
		t.Errorf("without a home directory state goes in the temp directory, got %s", got)
	}
}

func TestDefaultStateDirIgnoresARelativeXDG(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "relative/state")
	if got := DefaultStateDir(); !filepath.IsAbs(got) {
		t.Errorf("a relative XDG_STATE_HOME is ignored, got %s", got)
	}
}

func TestNewRecordKeepsTakeover(t *testing.T) {
	j := &Job{Title: "info fd", Args: []string{"info", "fd"}, State: Done, Takeover: true}
	if r := NewRecord(j, ""); !r.Takeover {
		t.Error("a full-screen job's record says so")
	}
	b, err := json.Marshal(NewRecord(&Job{Args: []string{"update"}}, ""))
	if err != nil || strings.Contains(string(b), "takeover") {
		t.Errorf("a drawer job leaves the field out: %s %v", b, err)
	}
}
