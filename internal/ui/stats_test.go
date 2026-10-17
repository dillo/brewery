package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/analytics"
)

// fakeStats serves the analytics fixtures. cached are the copies kept from
// earlier downloads, fetched at cachedAt; err makes downloads fail. tried
// is the day a fetch last started.
type fakeStats struct {
	fresh    map[string]analytics.List
	cached   map[string]analytics.List
	cachedAt time.Time
	err      error
	fetched  []string // lists downloaded, in order
	mu       sync.Mutex
	tried    string
}

func statsFixture(t *testing.T, list string) analytics.List {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "analytics", "testdata", strings.ReplaceAll(list, "/", "-")+".json"))
	if err != nil {
		t.Fatal(err)
	}
	l, err := analytics.Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func newFakeStats(t *testing.T) *fakeStats {
	f := &fakeStats{fresh: map[string]analytics.List{}, cached: map[string]analytics.List{}}
	for _, l := range analytics.Lists {
		f.fresh[l] = statsFixture(t, l)
	}
	return f
}

func (f *fakeStats) Cached(list string) (analytics.List, time.Time, error) {
	if l, ok := f.cached[list]; ok {
		return l, f.cachedAt, nil
	}
	return analytics.List{}, time.Time{}, os.ErrNotExist
}

func (f *fakeStats) Fetch(_ context.Context, list string) (analytics.List, error) {
	f.fetched = append(f.fetched, list)
	if f.err != nil {
		return analytics.List{}, f.err
	}
	return f.fresh[list], nil
}

func (f *fakeStats) Tried() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.tried
}

func (f *fakeStats) SetTried(day string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tried = day
	return nil
}

// newStatsApp is a Discover app with analytics to read, still on Home.
func newStatsApp(t *testing.T) (*App, *fakeStats) {
	t.Helper()
	a, _ := newDiscoverApp(t)
	st := newFakeStats(t)
	a.UseAnalytics(st)
	return a, st
}

// discoverCells are the Discover table's rows, as shown.
func discoverCells(a *App) []string {
	var rows []string
	for _, r := range a.discover.table.Rows() {
		cells := make([]string, len(r))
		for i, c := range r {
			cells[i] = stripANSI(c)
		}
		rows = append(rows, strings.Join(cells, "|"))
	}
	return rows
}

func TestDiscoverPopular(t *testing.T) {
	a, st := newStatsApp(t)
	show(t, a, "Popular tools")
	if !slices.Equal(st.fetched, analytics.Lists) {
		t.Errorf("fetched %v, want every list in order", st.fetched)
	}
	// Packages brew doesn't know (from other taps) are left out, and the
	// ranks count what's shown.
	want := []string{
		"1|gh|GitHub command-line tool|217k",
		"2|✓ git|Distributed revision control system|105k",
		"3|wget|Internet file retriever|54k",
		"4|ripgrep|Search tool like grep and The Silver Searcher|30k",
		"5|✓ fd|Simple, fast and user-friendly alternative to find|16k",
		"6|python@3.15|Interpreted, interactive, object-oriented programming language|5.6k",
	}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Popular tools:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	out := stripANSI(a.render())
	for _, s := range []string{"Popular tools 6", "Popular apps 4", "Rising 4", "New 2", "Fonts 2", "All 17", "The most-installed tools of the last 30 days."} {
		if !strings.Contains(out, s) {
			t.Errorf("missing %q in\n%s", s, out)
		}
	}
	if strings.Contains(out, "analytics from") {
		t.Errorf("today's analytics need no date:\n%s", out)
	}

	show(t, a, "Popular apps") // without fonts
	want = []string{
		"1|codex|OpenAI's coding agent that runs in your terminal|135k",
		"2|✓ docker-desktop|App to build and share containerised applications and microservices|32k",
		"3|firefox|Web browser|10k",
		"4|0-ad|Real-time strategy game|249",
	}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Popular apps:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscoverRising(t *testing.T) {
	a, _ := newStatsApp(t)
	show(t, a, "Rising")
	want := []string{
		"1|python@3.15|formula|new|Interpreted, interactive, object-oriented programming language",
		"2|wget|formula|×4.0|Internet file retriever",
		"3|✓ git|formula|×1.4|Distributed revision control system",
		"4|codex|cask|×1.3|OpenAI's coding agent that runs in your terminal",
	}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Rising:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscoverFonts(t *testing.T) {
	a, st := newStatsApp(t)
	// font-fira-code has no installs: it follows the ranked fonts, by name.
	apps := st.fresh[analytics.Apps30]
	apps.Items = slices.DeleteFunc(slices.Clone(apps.Items), func(it analytics.Item) bool { return it.Name == "font-fira-code" })
	st.fresh[analytics.Apps30] = apps
	show(t, a, "Fonts")
	want := []string{"1|font-jetbrains-mono|JetBrains Mono|2.8k", "|font-fira-code|Fira Code|"}
	if got := discoverCells(a); !slices.Equal(got, want) {
		t.Errorf("Fonts:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestDiscoverUsesTheCacheOffline(t *testing.T) {
	a, st := newStatsApp(t)
	st.cached[analytics.Tools30] = st.fresh[analytics.Tools30]
	st.cachedAt = testNow.Add(-24 * time.Hour)
	st.err = errors.New("dial tcp: no route to host")
	show(t, a, "Popular tools")
	if len(st.fetched) != 1 {
		t.Errorf("fetched %v: a failure stops the rest", st.fetched)
	}
	out := stripANSI(a.render())
	if !strings.Contains(out, "gh") || !strings.Contains(out, "analytics from Oct 12") {
		t.Errorf("want yesterday's copy, dated:\n%s", out)
	}
	show(t, a, "Popular apps") // nothing kept for apps
	if out := stripANSI(a.render()); !strings.Contains(out, "couldn't reach formulae.brew.sh · r retries") {
		t.Errorf("the failure isn't explained:\n%s", out)
	}
	// New and All don't need the network.
	show(t, a, "New")
	if got := discoverRows(a); len(got) != 2 {
		t.Errorf("New shows %v", got)
	}

	st.err = nil
	show(t, a, "Popular apps")
	keys(t, a, "r")
	if out := stripANSI(a.render()); !strings.Contains(out, "codex") || strings.Contains(out, "analytics from") {
		t.Errorf("after r, want today's apps:\n%s", out)
	}
}

func TestDiscoverFetchesOnceADay(t *testing.T) {
	a, st := newStatsApp(t)
	for _, l := range analytics.Lists {
		st.cached[l] = st.fresh[l]
	}
	st.cachedAt = testNow.Add(-time.Hour) // already today
	show(t, a, "Popular tools")
	keys(t, a, "1", "4")
	if len(st.fetched) != 0 {
		t.Errorf("fetched %v, though the copies are from today", st.fetched)
	}
	keys(t, a, "r")
	if len(st.fetched) != 5 {
		t.Errorf("r fetched %v, want every list", st.fetched)
	}

	a, st = newStatsApp(t)
	st.err = errors.New("offline")
	show(t, a, "Popular tools")
	keys(t, a, "1", "4")
	if len(st.fetched) != 1 {
		t.Errorf("fetched %v: a failed fetch isn't retried by opening Discover again the same day", st.fetched)
	}
	if st.tried != "2026-10-13" {
		t.Errorf("tried %q: the day is kept for the next start", st.tried)
	}

	// The next start the same day remembers the failed try.
	a, st2 := newStatsApp(t)
	st2.tried, st2.err = st.tried, st.err
	show(t, a, "Popular tools")
	if len(st2.fetched) != 0 {
		t.Errorf("fetched %v after a restart, though a fetch failed earlier today", st2.fetched)
	}
	keys(t, a, "r")
	if len(st2.fetched) != 1 {
		t.Errorf("r fetched %v: it fetches anyway", st2.fetched)
	}

	// A try on an earlier day doesn't count.
	a, st3 := newStatsApp(t)
	st3.tried = "2026-10-12"
	show(t, a, "Popular tools")
	if len(st3.fetched) != 5 {
		t.Errorf("fetched %v, want every list", st3.fetched)
	}
}

func TestDiscoverSaysWhyTheFetchFailed(t *testing.T) {
	for _, c := range []struct {
		err  error
		want string
	}{
		{errors.New("dial tcp: no route to host"), "couldn't reach formulae.brew.sh · r retries"},
		{&analytics.StatusError{List: analytics.Tools30, Code: 404}, "formulae.brew.sh answered 404 · r retries"},
		{fmt.Errorf("%s: %w: eof", analytics.Tools30, analytics.ErrBadList), "couldn't read formulae.brew.sh's lists · r retries"},
	} {
		a, st := newStatsApp(t)
		st.err = c.err
		show(t, a, "Popular tools")
		if out := stripANSI(a.render()); !strings.Contains(out, c.want) {
			t.Errorf("%v: want %q in\n%s", c.err, c.want, out)
		}
	}
}

func TestDiscoverWhileFetching(t *testing.T) {
	a, _ := newStatsApp(t)
	a.selectEntry(entryIndex("Popular tools")) // nothing comes back yet
	if a.fetchStats(true) == nil {
		t.Fatal("the first fetch didn't start")
	}
	if a.fetchStats(true) != nil {
		t.Error("a second fetch started while one is running")
	}
	if !strings.Contains(stripANSI(a.render()), "fetching analytics") {
		t.Errorf("want a note that analytics are coming:\n%s", stripANSI(a.render()))
	}
}

func TestShortCount(t *testing.T) {
	for n, want := range map[int]string{0: "", 249: "249", 2810: "2.8k", 54269: "54k", 217104: "217k", 2994048: "3.0M"} {
		if got := shortCount(n); got != want {
			t.Errorf("shortCount(%d) = %q, want %q", n, got, want)
		}
	}
}
