package ui

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/state"
)

// fakeReleases answers release and tag requests from tables, by
// owner/repo, and records what it was asked.
type fakeReleases struct {
	mu    sync.Mutex
	asked []string
	rels  map[string][]github.Release
	tags  map[string][]string
	err   error // every answer's, when set
}

func (f *fakeReleases) Releases(_ context.Context, owner, repo string) ([]github.Release, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, "releases "+owner+"/"+repo)
	return f.rels[owner+"/"+repo], f.err
}

func (f *fakeReleases) Tags(_ context.Context, owner, repo string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.asked = append(f.asked, "tags "+owner+"/"+repo)
	return f.tags[owner+"/"+repo], f.err
}

// luaRepo is where the fixture's lua-language-server (3.19.1, outdated by
// 3.19.2) comes from, by its homepage.
const luaRepo = "LuaLS/lua-language-server"

// luaReleases are its releases: one in range, one past it, and the one
// installed.
var luaReleases = []github.Release{
	{Tag: "3.19.3", Body: "past the upgrade"},
	{Tag: "3.19.2", URL: "https://github.com/LuaLS/lua-language-server/releases/tag/3.19.2",
		Published: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC),
		Body:      "## Changes\n* **Breaking**: removed `--old`\n* fixed a crash"},
	{Tag: "3.19.1", Body: "already installed"},
}

// notesApp is the fixture app on Outdated, wide enough for the details,
// with lua-language-server selected and f answering for GitHub.
func notesApp(t *testing.T, f *fakeReleases) *App {
	t.Helper()
	a := newJobApp(t, &fakeStarter{})
	if f != nil {
		a.UseReleases(f)
	}
	a.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	selectOutdated(t, a, "lua-language-server")
	return a
}

func mustShow(t *testing.T, a *App, wants ...string) {
	t.Helper()
	out := stripANSI(a.render())
	for _, w := range wants {
		if !strings.Contains(out, w) {
			t.Errorf("the screen lacks %q:\n%s", w, out)
		}
	}
}

func mustNotShow(t *testing.T, a *App, unwanted ...string) {
	t.Helper()
	out := stripANSI(a.render())
	for _, w := range unwanted {
		if strings.Contains(out, w) {
			t.Errorf("the screen shows %q:\n%s", w, out)
		}
	}
}

func TestWhatsNewBeforeFetching(t *testing.T) {
	a := notesApp(t, &fakeReleases{})
	mustShow(t, a, "lua-language-server 3.19.1 → 3.19.2", luaRepo, "w shows what's new, from GitHub")
}

func TestWhatsNewFetches(t *testing.T) {
	f := &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}}
	a := notesApp(t, f)
	keys(t, a, "w")
	mustShow(t, a, "1 release since 3.19.1", "fetched just now", "mentions breaking changes",
		"3.19.2", "2026-10-10", "• Breaking: removed --old", "• fixed a crash")
	mustNotShow(t, a, "past the upgrade", "already installed")
	keys(t, a, "w")
	if len(f.asked) != 1 || f.asked[0] != "releases "+luaRepo {
		t.Errorf("notes kept for these versions aren't fetched again: %v", f.asked)
	}
	saved, err := state.Load(a.cachePath)
	if n, ok := saved.ReleaseNotes["formula/lua-language-server"]; err != nil || !ok || len(n.Releases) != 1 {
		t.Errorf("the notes should be saved: %+v, %v", saved.ReleaseNotes, err)
	}
}

func TestWhatsNewNewerVersion(t *testing.T) {
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: luaReleases}})
	keys(t, a, "w")
	a.snap.Outdated[0].Current = "3.19.3"
	a.sync()
	mustShow(t, a, "lua-language-server 3.19.1 → 3.19.3", "w shows what's new, from GitHub")
	if _, ok := a.snap.ReleaseNotes["formula/lua-language-server"]; !ok {
		t.Error("notes for a package still outdated are kept")
	}
	a.snap.Outdated = a.snap.Outdated[1:]
	a.sync()
	if _, ok := a.snap.ReleaseNotes["formula/lua-language-server"]; ok {
		t.Error("notes for a package no longer outdated are dropped")
	}
}

func TestWhatsNewNoReleasesInRange(t *testing.T) {
	f := &fakeReleases{tags: map[string][]string{luaRepo: {"3.19.2", "3.19.1"}}}
	a := notesApp(t, f)
	keys(t, a, "w")
	mustShow(t, a, "no release notes between these versions", "gx opens it")
	want := "https://github.com/LuaLS/lua-language-server/compare/3.19.1...3.19.2"
	if got := a.snap.ReleaseNotes["formula/lua-language-server"].Link; got != want {
		t.Errorf("link %q, want %q", got, want)
	}
}

func TestWhatsNewFailure(t *testing.T) {
	f := &fakeReleases{err: github.ErrRateLimited}
	a := notesApp(t, f)
	keys(t, a, "w")
	mustShow(t, a, "couldn't fetch: rate limit reached", "w tries again")
	if _, ok := a.snap.ReleaseNotes["formula/lua-language-server"]; ok {
		t.Error("a failure isn't kept")
	}
	f.err, f.rels = nil, map[string][]github.Release{luaRepo: luaReleases}
	keys(t, a, "w")
	mustShow(t, a, "1 release since 3.19.1")
	mustNotShow(t, a, "couldn't fetch")
}

func TestWhatsNewNotOnGitHub(t *testing.T) {
	f := &fakeReleases{}
	a := notesApp(t, f)
	keys(t, a, "j") // docker-desktop, from www.docker.com
	mustShow(t, a, "release notes come from GitHub", "www.docker.com", "gx opens the homepage")
	keys(t, a, "w")
	if len(f.asked) != 0 || !strings.Contains(stripANSI(a.statusBar()), "docker-desktop's release notes aren't on GitHub") {
		t.Errorf("w on a package not on GitHub only notes: %v %q", f.asked, stripANSI(a.statusBar()))
	}
}

func TestWhatsNewOnlyNotes(t *testing.T) {
	a := notesApp(t, nil)
	keys(t, a, "w")
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "release notes aren't available") {
		t.Errorf("no fetcher: %q", bar)
	}

	f := &fakeReleases{}
	a = notesApp(t, f)
	keys(t, a, "space", "w")
	if bar := stripANSI(a.statusBar()); len(f.asked) != 0 || !strings.Contains(bar, "w shows one package at a time") {
		t.Errorf("with marks: %v %q", f.asked, bar)
	}

	a = notesApp(t, f)
	a.fetchingNotes, a.fetchingName = "formula/fd", "fd"
	keys(t, a, "w")
	if bar := stripANSI(a.statusBar()); len(f.asked) != 0 || !strings.Contains(bar, "already asking GitHub about fd") {
		t.Errorf("while fetching: %v %q", f.asked, bar)
	}
	mustShow(t, a, "w shows what's new")
	a.fetchingNotes = "formula/lua-language-server"
	if !regexp.MustCompile(`asking GitHub…`).MatchString(stripANSI(a.render())) {
		t.Errorf("the pane says it's asking:\n%s", stripANSI(a.render()))
	}
}

func TestWhatsNewLongNotes(t *testing.T) {
	body := strings.Repeat("* a line\n", noteLines+5)
	rels := []github.Release{{Tag: "3.19.2", Body: body, URL: "https://example.com/r"}}
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: rels}})
	keys(t, a, "w")
	lines := a.releaseLines(rels[0], 80)
	if len(lines) != noteLines+2 || !strings.Contains(stripANSI(lines[noteLines]), "… 5 more lines") || stripANSI(lines[noteLines+1]) != "https://example.com/r" {
		t.Errorf("long notes end after %d lines with how many more and where: %q", noteLines, lines[noteLines:])
	}
	if got := a.releaseLines(github.Release{Tag: "x"}, 80); len(got) != 1 || stripANSI(got[0]) != "no notes" {
		t.Errorf("empty notes: %q", got)
	}
}

func TestWhatsNewScrolls(t *testing.T) {
	body := strings.Repeat("* a line\n", 30)
	rels := []github.Release{{Tag: "3.19.2", Body: body}}
	a := notesApp(t, &fakeReleases{rels: map[string][]github.Release{luaRepo: rels}})
	keys(t, a, "w", "l", "ctrl+d")
	if a.outdated.pv.off == 0 {
		t.Error("j/k and ctrl+d scroll the details once they have the focus")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "j/k scroll") {
		t.Errorf("the hints say so: %q", bar)
	}
}
