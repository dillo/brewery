package ui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
)

// fakeLogs stands in for the log files, counting the reads.
type fakeLogs struct {
	text  map[string]string
	reads map[string]int
}

func (f *fakeLogs) read(path string) (string, error) {
	if f.reads == nil {
		f.reads = map[string]int{}
	}
	f.reads[path]++
	text, ok := f.text[path]
	if !ok {
		return "", fs.ErrNotExist
	}
	return text, nil
}

// numbered is n log lines: "line 1" to "line n".
func numbered(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	return strings.Join(lines, "\n")
}

func TestReadLogTail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.log")
	if _, err := readLogTail(path); !os.IsNotExist(err) {
		t.Errorf("a missing log reports that it doesn't exist: %v", err)
	}
	long := strings.Repeat("x", logTailBytes) + "\nfirst whole line\n\x1b[31mred\x1b[0m\tthen a tab\r\n10%\r50%\r100%\n"
	if err := os.WriteFile(path, []byte(long), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := readLogTail(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := "first whole line\nred    then a tab\n100%"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestServicesPreview(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	logs := &fakeLogs{text: map[string]string{
		"/opt/homebrew/var/log/postgresql@16.log": numbered(100),
		"/opt/homebrew/var/log/redis.log":         "Ready to accept connections",
	}}
	a.readLog = logs.read
	keys(t, a, "4") // postgresql@16
	out := stripANSI(a.render())
	for _, want := range []string{
		"error (exit 1)", "LC_ALL=", "/opt/homebrew/var/log/postgresql@16.log",
		"End of the log", "line 100",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "line 60 ") || strings.Contains(out, "more") {
		t.Errorf("only the end of the log fits, and it isn't cut off:\n%s", out)
	}
	if strings.Contains(out, "errors ") {
		t.Errorf("one file for output and errors is shown once:\n%s", out)
	}

	keys(t, a, "j") // redis
	out = stripANSI(a.render())
	for _, want := range []string{"started · pid 80476", "Ready to accept connections"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if logs.reads["/opt/homebrew/var/log/redis.log"] != 1 || logs.reads["/opt/homebrew/var/log/postgresql@16.log"] != 1 {
		t.Errorf("each log is read once: %v", logs.reads)
	}
}

func TestServicesPreviewRereadsAfterARefresh(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "old"}}
	a.readLog = logs.read
	keys(t, a, "4")
	logs.text["/opt/homebrew/var/log/postgresql@16.log"] = "new"
	_, cmd := a.Update(resultMsg{q: qServices, services: a.snap.Services})
	settle(t, a, cmd)
	if out := stripANSI(a.render()); !strings.Contains(out, "new") {
		t.Errorf("after services refresh the log is read again:\n%s", out)
	}
}

func TestServicesPreviewWithoutALog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.snap.Services[1].Log, a.snap.Services[1].ErrorLog = "", "/tmp/redis.err"
	a.sync()
	keys(t, a, "4")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing logged yet") {
		t.Errorf("a log that doesn't exist yet says so:\n%s", out)
	}
	keys(t, a, "j")
	out := stripANSI(a.render())
	if !strings.Contains(out, "brew knows no log for this service") || !strings.Contains(out, "/tmp/redis.err") {
		t.Errorf("no log path: say so, and show the error log:\n%s", out)
	}
}

func TestServicesPreviewAt80Columns(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	keys(t, a, "4")
	out := stripANSI(a.render())
	if !strings.Contains(out, "postgresql@16  ") || !strings.Contains(out, "error (exit 1)") {
		t.Errorf("names and statuses fit beside the preview:\n%s", out)
	}
}

func TestTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip(err)
	}
	if got := tilde(home + "/Library/LaunchAgents/x.plist"); got != "~/Library/LaunchAgents/x.plist" {
		t.Errorf("got %q", got)
	}
	if got := tilde("/opt/homebrew/var/log/x.log"); got != "/opt/homebrew/var/log/x.log" {
		t.Errorf("got %q", got)
	}
}

func TestServiceActionsFollowTheName(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	keys(t, a, "4", "j") // redis
	// A refresh lists redis first now: the cursor follows it.
	a.snap.Services = []brew.Service{a.snap.Services[1], a.snap.Services[0]}
	a.sync()
	keys(t, a, "R")
	if want := []string{"services", "restart", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}
