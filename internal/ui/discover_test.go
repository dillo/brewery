package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dillo/brewery/internal/brew"
)

// fakeCatalog serves the catalogue fixture; added is what brew's last
// update brought in. calls counts catalogue reads. details answers
// questions about one package by name (a bare entry if it isn't there),
// detailsErr makes them fail, and asked records each package asked about.
type fakeCatalog struct {
	entries, added []brew.Entry
	err            error
	calls          int
	details        map[string]brew.Details
	detailsErr     error
	asked          []string
}

func newFakeCatalog(t *testing.T) *fakeCatalog {
	return &fakeCatalog{
		entries: brew.ParseCatalogue(string(readFixture(t, "catalogue.txt"))),
		added:   []brew.Entry{{Name: "python@3.15", Kind: brew.KindFormula}, {Name: "codex", Kind: brew.KindCask}},
	}
}

func (f *fakeCatalog) Catalogue(context.Context) ([]brew.Entry, error) {
	f.calls++
	return f.entries, f.err
}

func (f *fakeCatalog) NewSinceUpdate(context.Context) ([]brew.Entry, error) { return f.added, nil }

func (f *fakeCatalog) Details(_ context.Context, name string, kind brew.Kind) (brew.Details, error) {
	f.asked = append(f.asked, name)
	if f.detailsErr != nil {
		return brew.Details{}, f.detailsErr
	}
	if d, ok := f.details[name]; ok {
		return d, nil
	}
	return brew.Details{Package: brew.Package{Name: name, Kind: kind}}, nil
}

// newDiscoverApp is an app with packages loaded and a catalogue to read,
// still on Home.
func newDiscoverApp(t *testing.T) (*App, *fakeCatalog) {
	t.Helper()
	a := newTestApp(t, newFakeSource(t))
	cat := newFakeCatalog(t)
	a.UseCatalog(cat)
	a.detailsDelay = 0
	drain(t, a, a.Init())
	return a, cat
}

// discoverRows are the names in the Discover table, as shown.
func discoverRows(a *App) []string {
	var names []string
	for _, r := range a.discover.table.Rows() {
		names = append(names, stripANSI(r[0]))
	}
	return names
}

func TestDiscoverReadsTheCatalogueOnce(t *testing.T) {
	a, cat := newDiscoverApp(t)
	if cat.calls != 0 {
		t.Fatalf("read the catalogue %d times before Discover opened", cat.calls)
	}
	keys(t, a, "6")
	if a.tab != tabDiscover || cat.calls != 1 || !a.discover.loaded {
		t.Fatalf("tab %d, %d reads, loaded %v", a.tab, cat.calls, a.discover.loaded)
	}
	keys(t, a, "1", "6", "]", "[")
	if cat.calls != 1 {
		t.Errorf("read the catalogue %d times, want once", cat.calls)
	}
	// brew update can change the catalogue: a refresh reads it again.
	keys(t, a, "r")
	if cat.calls != 2 {
		t.Errorf("after r: %d reads, want 2", cat.calls)
	}
}

func TestDiscoverViews(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6", "f", "f", "f") // to New
	out := stripANSI(a.render())
	for _, want := range []string{"6 Discover", "New 2", "All 17", "Added to Homebrew since brew last updated."} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
	// New takes descriptions from the catalogue.
	if got := strings.Join(discoverRows(a), ","); got != "python@3.15,codex" {
		t.Errorf("New shows %s", got)
	}
	if !strings.Contains(out, "OpenAI's coding agent") {
		t.Errorf("codex's description is missing:\n%s", out)
	}

	keys(t, a, "f", "f")
	rows := discoverRows(a)
	if len(rows) != 17 || rows[0] != "abcl" {
		t.Fatalf("All shows %v", rows)
	}
	// Installed packages stay in the list, marked.
	if !strings.Contains(strings.Join(rows, ","), "✓ git") || !strings.Contains(strings.Join(rows, ","), "✓ docker-desktop") {
		t.Errorf("installed packages aren't marked: %v", rows)
	}
	if strings.Contains(strings.Join(rows, ","), "✓ docker,") {
		t.Errorf("the docker formula isn't installed, only the cask: %v", rows)
	}
	keys(t, a, "F")
	if a.discover.showing != discFonts {
		t.Errorf("F goes back to Fonts, got %d", a.discover.showing)
	}
}

func TestDiscoverFilterSearchesDescriptions(t *testing.T) {
	a, _ := newDiscoverApp(t)
	keys(t, a, "6", "F", "/", "r", "e", "t", "r", "i", "e", "v", "e", "r", "enter")
	if got := discoverRows(a); len(got) != 1 || got[0] != "wget" {
		t.Errorf("/retriever shows %v", got)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Discover · /retriever") || !strings.Contains(out, "Discover › wget") {
		t.Errorf("the title or status bar doesn't say:\n%s", out)
	}
	keys(t, a, "/", "x", "x", "x", "enter")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing matches /retrieverxxx") {
		t.Errorf("no match isn't explained:\n%s", out)
	}
}

func TestDiscoverEmptyAndFailed(t *testing.T) {
	a, cat := newDiscoverApp(t)
	cat.added = nil
	keys(t, a, "6", "f", "f", "f")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing new since brew last updated") {
		t.Errorf("an empty New isn't explained:\n%s", out)
	}

	a, cat = newDiscoverApp(t)
	cat.err = errors.New("brew search --desc : exit status 1")
	keys(t, a, "6")
	if out := stripANSI(a.render()); !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") {
		t.Errorf("the failure isn't shown:\n%s", out)
	}
	cat.err = nil
	keys(t, a, "r")
	if !a.discover.loaded || a.discover.err != nil {
		t.Errorf("r didn't retry: loaded %v, err %v", a.discover.loaded, a.discover.err)
	}
}

func TestDiscoverBeforeTheCatalogue(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.UseCatalog(newFakeCatalog(t))
	a.Update(press("6")) // the read hasn't come back
	if out := stripANSI(a.render()); !strings.Contains(out, "loading…") {
		t.Errorf("want a loading placeholder:\n%s", out)
	}
}

func TestRefreshDuringReadIsNotDropped(t *testing.T) {
	a, cat := newDiscoverApp(t)
	_, first := a.Update(press("6")) // the first read is in flight
	keys(t, a, "r")
	drain(t, a, first)
	if cat.calls != 2 {
		t.Errorf("read the catalogue %d times, want 2", cat.calls)
	}
}

func TestOpeningDuringReadReadsOnce(t *testing.T) {
	a, cat := newDiscoverApp(t)
	_, first := a.Update(press("6"))
	keys(t, a, "1", "6", ":", "esc") // the : line wants the catalogue too
	drain(t, a, first)
	if cat.calls != 1 {
		t.Errorf("read the catalogue %d times, want 1", cat.calls)
	}
}

func TestRefreshBeforeDiscoverOpens(t *testing.T) {
	a, cat := newDiscoverApp(t)
	st := newFakeStats(t)
	a.UseAnalytics(st)
	keys(t, a, "r")
	if cat.calls != 0 || len(st.fetched) != 0 {
		t.Errorf("r on Home read the catalogue %d times and fetched %v: Discover hasn't opened", cat.calls, st.fetched)
	}
	keys(t, a, "6")
	if cat.calls != 1 || len(st.fetched) != 5 {
		t.Errorf("opening Discover read the catalogue %d times and fetched %v", cat.calls, st.fetched)
	}
}
