package jobs

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	h.Save(record("upgrade fd", t0))
	os.WriteFile(filepath.Join(h.Dir, "20990101T000000.000000000.json"), []byte("{not json"), 0o644)
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
	if back := r.Job(); back.Title != j.Title || back.State != Done || !back.Ended.Equal(j.Ended) {
		t.Errorf("Job() lost fields: %+v", back)
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

func TestHistorySaveRemovesStrayTempFiles(t *testing.T) {
	h := History{Dir: filepath.Join(t.TempDir(), "jobs")}
	if err := os.MkdirAll(h.Dir, 0o700); err != nil {
		t.Fatal(err)
	}
	stray := filepath.Join(h.Dir, "123.tmp")
	if err := os.WriteFile(stray, []byte("half"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := h.Save(record("pin git", t0)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Errorf("stray temp file should be removed: %v", err)
	}
}
