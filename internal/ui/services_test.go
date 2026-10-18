package ui

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
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
	show(t, a, "Services") // postgresql@16
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
	show(t, a, "Services")
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
	show(t, a, "Services")
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
	show(t, a, "Services")
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

// fakePager records the files handed to the log viewer.
type fakePager struct{ calls [][]string }

func (f *fakePager) cmd(files []string) *exec.Cmd {
	f.calls = append(f.calls, files)
	return exec.Command("true")
}

func TestLogKeyOpensTheLogs(t *testing.T) {
	dir := t.TempDir()
	out, errs := filepath.Join(dir, "out.log"), filepath.Join(dir, "err.log")
	for _, f := range []string{out, errs} {
		if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	pg := &fakePager{}
	a.UsePager(pg.cmd)
	a.snap.Services[0].Log, a.snap.Services[0].ErrorLog = out, errs // postgresql@16
	a.snap.Services[1].Log, a.snap.Services[1].ErrorLog = out, out  // redis
	a.sync()
	show(t, a, "Services")
	keys(t, a, "L", "j", "L")
	if want := [][]string{{out, errs}, {out}}; !reflect.DeepEqual(pg.calls, want) {
		t.Errorf("opened %v, want %v", pg.calls, want)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "L log") {
		t.Errorf("the hint should name L: %q", bar)
	}
}

func TestLogKeyExplainsMissingLogs(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	pg := &fakePager{}
	a.UsePager(pg.cmd)
	a.snap.Services[0].Log = filepath.Join(t.TempDir(), "none.log")
	a.snap.Services[0].ErrorLog = ""
	a.snap.Services[1].Log, a.snap.Services[1].ErrorLog = "", ""
	a.sync()
	show(t, a, "Services")
	keys(t, a, "L")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "postgresql@16 hasn't logged anything yet") {
		t.Errorf("status bar %q", bar)
	}
	keys(t, a, "j", "L")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "brew knows no log for redis") {
		t.Errorf("status bar %q", bar)
	}
	if len(pg.calls) != 0 {
		t.Errorf("nothing to show, so no viewer: %v", pg.calls)
	}
}

func TestLogKeyWaitsForTheRunningJob(t *testing.T) {
	log := filepath.Join(t.TempDir(), "x.log")
	if err := os.WriteFile(log, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p := newFakeProc(0)
	a := newJobApp(t, &fakeStarter{procs: []*fakeProc{p}})
	a.readLog = (&fakeLogs{}).read
	pg := &fakePager{}
	a.UsePager(pg.cmd)
	a.snap.Services[0].Log = log
	a.sync()
	keys(t, a, "U")
	show(t, a, "Services")
	keys(t, a, "L")
	if len(pg.calls) != 0 || !strings.Contains(stripANSI(a.statusBar()), "wait for update to finish") {
		t.Errorf("the viewer must wait for the job: %v %q", pg.calls, stripANSI(a.statusBar()))
	}
}

func TestPagerDoneRereadsTheLog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "x"}}
	a.readLog = logs.read
	show(t, a, "Services")
	_, cmd := a.Update(pagerDoneMsg{err: errors.New("exec: \"less\": executable file not found in $PATH")})
	settle(t, a, cmd)
	if logs.reads["/opt/homebrew/var/log/postgresql@16.log"] != 2 {
		t.Errorf("reads %v: the log is read again after viewing it", logs.reads)
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "couldn't show the log") {
		t.Errorf("a failed viewer says so: %q", bar)
	}
}

func TestServiceActionsFollowTheName(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	a.readLog = (&fakeLogs{}).read
	show(t, a, "Services")
	keys(t, a, "j") // redis
	// A refresh lists redis first now: the cursor follows it.
	a.snap.Services = []brew.Service{a.snap.Services[1], a.snap.Services[0]}
	a.sync()
	keys(t, a, "R")
	if want := []string{"services", "restart", "redis"}; !reflect.DeepEqual(lastCall(fs), want) {
		t.Errorf("ran %v, want %v", lastCall(fs), want)
	}
}

func TestServicesPreviewRereadsAfterAServiceWithoutALog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{"/opt/homebrew/var/log/postgresql@16.log": "x"}}
	a.readLog = logs.read
	a.snap.Services[1].Log = "" // redis
	a.sync()
	show(t, a, "Services")
	keys(t, a, "j", "k") // postgresql@16, redis, postgresql@16
	if n := logs.reads["/opt/homebrew/var/log/postgresql@16.log"]; n != 2 {
		t.Errorf("coming back to a service reads its log again: %d reads", n)
	}
}

func TestLogKeyWithoutAViewer(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{}).read
	show(t, a, "Services")
	keys(t, a, "L")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "the log viewer isn't available") {
		t.Errorf("status bar %q", bar)
	}
}

// longLog is n log lines: "log line 1" to "log line n".
func longLog(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("log line %d", i+1)
	}
	return strings.Join(lines, "\n")
}

const (
	pgLog    = "/opt/homebrew/var/log/postgresql@16.log"
	redisLog = "/opt/homebrew/var/log/redis.log"
)

func TestServicesDetailsScrollTheLog(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: longLog(200)}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	out := stripANSI(a.render())
	if !strings.Contains(out, "log line 200") {
		t.Fatalf("the details start at the newest line:\n%s", out)
	}
	keys(t, a, "k")
	if out := stripANSI(a.render()); strings.Contains(out, "log line 200") || !strings.Contains(out, "log line 199") || !strings.Contains(out, "The log · 1 line back") {
		t.Errorf("k scrolls back one line:\n%s", out)
	}
	keys(t, a, "j")
	if !strings.Contains(stripANSI(a.render()), "log line 200") {
		t.Error("j scrolls forward again")
	}
	keys(t, a, "g", "g")
	if !regexp.MustCompile(`\blog line 1\b`).MatchString(stripANSI(a.render())) {
		t.Error("gg goes to the oldest line read")
	}
	keys(t, a, "G")
	if !strings.Contains(stripANSI(a.render()), "log line 200") {
		t.Error("G goes back to the newest line")
	}
}

func TestShortLogDoesNotScroll(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: "only line\n"}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	before := stripANSI(a.render())
	keys(t, a, "k", "k", "ctrl+u")
	if after := stripANSI(a.render()); after != before {
		t.Errorf("a log that fits doesn't move:\n%s", after)
	}
}

func TestLogScrollStartsAtTheNewestForEachService(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	logs := &fakeLogs{text: map[string]string{pgLog: longLog(200), redisLog: longLog(300)}}
	a.readLog = logs.read
	show(t, a, "Services")
	keys(t, a, "l")
	keys(t, a, "k", "k", "k")
	if !strings.Contains(stripANSI(a.render()), "3 lines back") {
		t.Fatalf("scrolled back on the first service:\n%s", stripANSI(a.render()))
	}
	keys(t, a, "h", "j")
	out := stripANSI(a.render())
	if !strings.Contains(out, "log line 300") || !strings.Contains(out, "End of the log") {
		t.Errorf("the next service's log starts at its newest line:\n%s", out)
	}
}

func TestLogScrollKeepsItsPlaceOnARefresh(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: longLog(200)}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	keys(t, a, "k", "k")
	_, cmd := a.Update(resultMsg{q: qServices, services: a.snap.Services})
	settle(t, a, cmd)
	if out := stripANSI(a.render()); !strings.Contains(out, "2 lines back") {
		t.Errorf("a fresh read of the same log keeps the place:\n%s", out)
	}
}

// newestVisible is the number of the newest "log line N" in the output.
func newestVisible(t *testing.T, out string) int {
	t.Helper()
	m := regexp.MustCompile(`log line (\d+)`).FindAllStringSubmatch(out, -1)
	if len(m) == 0 {
		t.Fatalf("no log lines shown:\n%s", out)
	}
	n, _ := strconv.Atoi(m[len(m)-1][1])
	return n
}

// logRoom is how many log lines the Services details pane shows, and so
// how far ctrl+d pages: half of it.
func logRoom(a *App) int {
	_, _, _, room, _ := a.logWindow(a.services.pvW, a.services.h)
	return room
}

func TestLogScrollPagesByHalfThePane(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: longLog(200)}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	page := max(logRoom(a)/2, 1)
	if newestVisible(t, stripANSI(a.render())) != 200 {
		t.Fatal("starts at the newest line")
	}
	keys(t, a, "ctrl+u")
	if got := newestVisible(t, stripANSI(a.render())); got != 200-page {
		t.Errorf("ctrl+u pages back %d lines: newest is %d", page, got)
	}
	keys(t, a, "ctrl+u", "ctrl+d")
	if got := newestVisible(t, stripANSI(a.render())); got != 200-page {
		t.Errorf("ctrl+d pages forward again: newest is %d", got)
	}
	keys(t, a, "ctrl+d", "ctrl+d")
	if got := newestVisible(t, stripANSI(a.render())); got != 200 {
		t.Errorf("paging forward stops at the newest: %d", got)
	}
}

func TestLogScrollReclampsWhenThePaneGrows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: longLog(200)}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	keys(t, a, "g", "g")
	a.Update(tea.WindowSizeMsg{Width: a.width, Height: a.height + 10})
	keys(t, a, "k")
	out := stripANSI(a.render())
	if !regexp.MustCompile(`\blog line 1\b`).MatchString(out) {
		t.Errorf("k at the oldest line still shows it:\n%s", out)
	}
	before := newestVisible(t, out)
	keys(t, a, "j")
	if got := newestVisible(t, stripANSI(a.render())); got != before+1 {
		t.Errorf("j moves forward at once: newest %d, then %d", before, got)
	}
}

func TestLogEndingInANewlineShowsItsNewestLine(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.readLog = (&fakeLogs{text: map[string]string{pgLog: longLog(200) + "\n"}}).read
	show(t, a, "Services")
	keys(t, a, "l")
	lines := strings.Split(strings.TrimRight(stripANSI(a.render()), "\n"), "\n")
	for i, l := range lines {
		if strings.Contains(l, "log line 200") {
			if next := strings.TrimSpace(strings.Trim(lines[i+1], "│ ╰╯─")); next != "" {
				t.Errorf("the newest line is the last in the pane, not %q", next)
			}
			return
		}
	}
	t.Fatal("log line 200 isn't shown")
}
