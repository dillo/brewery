package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
	show(t, a, "Popular tools")
	if a.tab != tabDiscover || cat.calls != 1 || !a.discover.loaded {
		t.Fatalf("tab %d, %d reads, loaded %v", a.tab, cat.calls, a.discover.loaded)
	}
	show(t, a, "Home")
	keys(t, a, "4", "]", "[")
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
	show(t, a, "New")
	out := stripANSI(a.render())
	for _, want := range []string{"New", "Added to Homebrew since brew last updated."} {
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

	show(t, a, "Search all")
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
	show(t, a, "Fonts")
	if a.discover.showing != discFonts {
		t.Errorf("Fonts shows view %d", a.discover.showing)
	}
}

func TestDiscoverFilterSearchesDescriptions(t *testing.T) {
	a, _ := newDiscoverApp(t)
	show(t, a, "Search all")
	keys(t, a, "/", "r", "e", "t", "r", "i", "e", "v", "e", "r", "enter")
	if got := discoverRows(a); len(got) != 1 || got[0] != "wget" {
		t.Errorf("/retriever shows %v", got)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "Search all · /retriever") || !strings.Contains(out, "Search all › wget") {
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
	show(t, a, "New")
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing new since brew last updated") {
		t.Errorf("an empty New isn't explained:\n%s", out)
	}

	a, cat = newDiscoverApp(t)
	cat.err = errors.New("brew search --desc : exit status 1")
	show(t, a, "Popular tools")
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
	a.selectEntry(entryIndex("Popular tools")) // the read hasn't come back
	if out := stripANSI(a.render()); !strings.Contains(out, "loading…") {
		t.Errorf("want a loading placeholder:\n%s", out)
	}
}

func TestRefreshDuringReadIsNotDropped(t *testing.T) {
	a, cat := newDiscoverApp(t)
	first := a.selectEntry(entryIndex("Popular tools")) // the first read is in flight
	keys(t, a, "r")
	drain(t, a, first)
	if cat.calls != 2 {
		t.Errorf("read the catalogue %d times, want 2", cat.calls)
	}
}

func TestOpeningDuringReadReadsOnce(t *testing.T) {
	a, cat := newDiscoverApp(t)
	first := a.selectEntry(entryIndex("Popular tools"))
	show(t, a, "Home")
	keys(t, a, "4", ":", "esc") // the : line wants the catalogue too
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
	show(t, a, "Popular tools")
	if cat.calls != 1 || len(st.fetched) != 5 {
		t.Errorf("opening Discover read the catalogue %d times and fetched %v", cat.calls, st.fetched)
	}
}

func TestDiscoverHasNoDetailsFocusBeforeTheCatalogue(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.UseCatalog(newFakeCatalog(t))
	a.Update(tea.WindowSizeMsg{Width: 90, Height: 30})
	read := a.selectEntry(entryIndex("Popular tools")) // the read hasn't come back
	keys(t, a, "l")
	if a.focus != colList {
		t.Fatalf("l on a loading Discover: focus %d, want the list", a.focus)
	}
	keys(t, a, "tab", "enter")
	if a.focus != colList {
		t.Fatalf("tab and enter on a loading Discover: focus %d, want the list", a.focus)
	}
	if c := a.columnsFor(tabDiscover); c.details != 0 {
		t.Errorf("a loading Discover has a details column: %+v", c)
	}
	if bar := stripANSI(a.statusBar()); strings.Contains(bar, "h back to the list") {
		t.Errorf("status bar speaks of details: %s", bar)
	}
	drain(t, a, read)
	if !a.discover.loaded || a.focus != colList {
		t.Errorf("after the catalogue: loaded %v, focus %d, want the list", a.discover.loaded, a.focus)
	}
}

func TestMovingToLoadingDiscoverLandsOnItsList(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	a.UseCatalog(newFakeCatalog(t))
	show(t, a, "All")
	keys(t, a, "l")
	if a.focus != colDetails {
		t.Fatalf("focus %d, want details", a.focus)
	}
	a.selectEntry(entryIndex("Popular tools"))
	if a.focus != colList {
		t.Errorf("onto a loading Discover: focus %d, want the list", a.focus)
	}
}
