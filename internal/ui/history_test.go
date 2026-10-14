package ui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"brewery/internal/jobs"
)

// newHistoryApp is newJobApp with its job history kept in a temp dir.
func newHistoryApp(t *testing.T, fs *fakeStarter) (*App, jobs.History) {
	t.Helper()
	a := newJobApp(t, fs)
	h := jobs.History{Dir: t.TempDir()}
	a.UseHistory(h, nil)
	return a, h
}

func TestFinishedJobsAreRecorded(t *testing.T) {
	p := newFakeProc(1, "==> Upgrading fd\r\n", "Error: download failed\r\n")
	p.exit()
	a, h := newHistoryApp(t, &fakeStarter{procs: []*fakeProc{p}})
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}, refresh: packageQueries}))

	if len(a.history) != 1 {
		t.Fatalf("history has %d records, want 1", len(a.history))
	}
	r := a.history[0]
	if r.Title != "upgrade fd" || r.State != jobs.Failed || r.ExitCode != 1 || !strings.Contains(r.Output, "download failed") {
		t.Errorf("record: %+v", r)
	}
	if !r.Started.Equal(testNow) || !r.Ended.Equal(testNow) {
		t.Errorf("record times %v–%v, want %v", r.Started, r.Ended, testNow)
	}
	saved, err := h.Load()
	if err != nil || len(saved) != 1 || saved[0].Title != "upgrade fd" {
		t.Errorf("the record should be saved to disk: %+v %v", saved, err)
	}
	if len(a.acts) != 0 {
		t.Errorf("a finished job's action should be dropped: %v", a.acts)
	}
}

func TestStartFailureIsRecorded(t *testing.T) {
	a, _ := newHistoryApp(t, &fakeStarter{err: errors.New("fork/exec brew: no such file")})
	settle(t, a, a.run(action{title: "upgrade fd", args: []string{"upgrade", "fd"}}))
	if len(a.history) != 1 || a.history[0].State != jobs.Failed || !strings.Contains(a.history[0].Output, "couldn't start brew: fork/exec brew") {
		t.Errorf("a job that could not start should be recorded as failed: %+v", a.history)
	}
	if len(a.acts) != 0 {
		t.Errorf("its action should be dropped: %v", a.acts)
	}
}

func TestNewestRecordComesFirstAndHistoryIsCapped(t *testing.T) {
	a, _ := newHistoryApp(t, &fakeStarter{})
	old := make([]jobs.Record, jobs.Keep)
	for i := range old {
		old[i] = jobs.Record{Title: "old", Started: testNow.Add(-time.Duration(i+1) * time.Hour)}
	}
	a.UseHistory(jobs.History{Dir: t.TempDir()}, old)
	settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))
	if len(a.history) != jobs.Keep || a.history[0].Title != "pin git" {
		t.Errorf("want %d records with the new one first, got %d starting %q", jobs.Keep, len(a.history), a.history[0].Title)
	}
}

func TestHistorySaveFailureShowsInStatusBar(t *testing.T) {
	a, _ := newHistoryApp(t, &fakeStarter{})
	file := filepath.Join(t.TempDir(), "file")
	os.WriteFile(file, nil, 0o644)
	a.UseHistory(jobs.History{Dir: filepath.Join(file, "jobs")}, nil) // can't make a dir under a file
	settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "history not saved") {
		t.Errorf("status bar: %q", bar)
	}
	if len(a.history) != 1 {
		t.Error("the record is still kept for this session")
	}
}

func TestJobsAreRecordedWithoutAHistoryDir(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.run(action{title: "pin git", args: []string{"pin", "git"}}))
	if len(a.history) != 1 || a.history[0].State != jobs.Done {
		t.Errorf("without a history dir, jobs are still kept for the session: %+v", a.history)
	}
}
