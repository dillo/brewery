package ui

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/brew"
)

func TestUnusedServices(t *testing.T) {
	dir := "/Users/me/Library/LaunchAgents"
	file := func(label string) string { return filepath.Join(dir, label+".plist") }
	svcs := []brew.Service{
		{Name: "redis", Label: "homebrew.mxcl.redis", Loaded: true, Status: "started", File: file("homebrew.mxcl.redis")},
		{Name: "postgresql@16", Label: "homebrew.mxcl.postgresql@16", Loaded: true, Status: "error", File: file("homebrew.mxcl.postgresql@16")},
		{Name: "mysql", Label: "homebrew.mxcl.mysql", Status: "stopped", File: file("homebrew.mxcl.mysql")},
		{Name: "ghost", Label: "homebrew.mxcl.ghost", Loaded: true, Status: "started", File: file("homebrew.mxcl.ghost")},
	}
	files := []string{file("homebrew.mxcl.redis"), file("homebrew.mxcl.postgresql@16"), file("homebrew.mxcl.mysql"), file("sh.brew.old")}
	remove, stop := unusedServices(files, svcs)
	if want := []string{file("homebrew.mxcl.mysql"), file("sh.brew.old")}; !slices.Equal(remove, want) {
		t.Errorf("remove %q, want %q (a loaded service keeps its file, even in error)", remove, want)
	}
	if want := []string{"ghost"}; !slices.Equal(stop, want) {
		t.Errorf("stop %q, want %q (loaded, but its file is gone)", stop, want)
	}
}

func TestTidyQueriesFollowDoctor(t *testing.T) {
	src := newCountingSource(t)
	a := newTestApp(t, src)
	drain(t, a, a.Init())
	if src.calls["autoremove"] != 1 || src.calls["service files"] != 1 {
		t.Errorf("a full refresh checks autoremove and the service files once each: %v", src.calls)
	}
	if !a.loaded[qAutoremove] || !a.loaded[qServiceFiles] {
		t.Error("both are loaded after the refresh")
	}
}

func TestTidyQueriesAfterActions(t *testing.T) {
	for _, tt := range []struct {
		name  string
		entry string
		keys  []string
		want  string
	}{
		{"uninstall checks autoremove", "Top-level", []string{"j", "x", "y"}, "autoremove"},
		{"uninstall checks the service files", "Top-level", []string{"j", "x", "y"}, "service files"},
		{"a service start checks the service files", "Services", []string{"s"}, "service files"},
	} {
		src := newCountingSource(t)
		a := New(src, (&fakeStarter{}).start, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
		a.now = func() time.Time { return testNow }
		a.tickEvery = 0
		a.Update(tea.WindowSizeMsg{Width: 140, Height: 40})
		show(t, a, tt.entry)
		keys(t, a, tt.keys...) // the fake job finishes at once
		if src.calls[tt.want] != 1 {
			t.Errorf("%s: calls %v", tt.name, src.calls)
		}
	}
}

func TestTidyQueryErrorsKeepTheLastResult(t *testing.T) {
	src := newFakeSource(t)
	src.autoremoveErr, src.serviceFilesErr = errors.New("boom"), errors.New("boom")
	a := New(src, nil, fixtureSnapshot(t), filepath.Join(t.TempDir(), "s.json"))
	a.now = func() time.Time { return testNow }
	before := slices.Clone(a.snap.Autoremove)
	drain(t, a, a.Init())
	if a.errs[qAutoremove] == nil || a.errs[qServiceFiles] == nil || !slices.Equal(a.snap.Autoremove, before) {
		t.Errorf("errors are kept, data isn't wiped: %v %v %q", a.errs[qAutoremove], a.errs[qServiceFiles], a.snap.Autoremove)
	}
}
