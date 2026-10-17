package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
)

// loadedHome is a Home tab whose queries have all succeeded.
func loadedHome() homeTab {
	return homeTab{st: testStyles, errs: map[query]error{},
		loaded: map[query]bool{qInstalled: true, qOutdated: true, qServices: true, qTaps: true, qDoctor: true, qCleanup: true}}
}

func TestHomeTiles(t *testing.T) {
	h := loadedHome()
	tiles := h.tiles(fixtureSnapshot(t))
	got := []string{}
	for _, tl := range tiles {
		got = append(got, tl.title+"="+tl.value+" ("+tl.detail+")")
	}
	want := []string{
		"Installed=7 (5 formulae · 2 casks)",
		"Outdated=2 (lua-language-server, docker-desktop)",
		"Services=1 error (postgresql@16)",
		"Doctor=2 warnings (see below)",
		"Cleanup=86.7MB (c to reclaim)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("tiles:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestHomeTilesHealthy(t *testing.T) {
	h := loadedHome()
	snap := state.Snapshot{
		Services:  []brew.Service{{Name: "redis", Status: "started"}},
		DoctorRan: true,
	}
	tiles := h.tiles(snap)
	if tiles[1].value != "✓ 0" || tiles[2].value != "1 running" || tiles[3].value != "✓ clean" {
		t.Errorf("healthy tiles: %q %q %q", tiles[1].value, tiles[2].value, tiles[3].value)
	}
	if tiles[3].tab != -1 {
		t.Error("doctor tile has no tab to open")
	}
	if got := h.tiles(state.Snapshot{}); got[3].value != "…" {
		t.Errorf("doctor tile before doctor runs = %q", got[3].value)
	}
}

func TestHomeTilesLoading(t *testing.T) {
	h := homeTab{st: testStyles}
	for _, tl := range h.tiles(state.Snapshot{})[:3] {
		if tl.value != "…" || tl.detail != "loading" {
			t.Errorf("%s tile before any result = %q (%s)", tl.title, tl.value, tl.detail)
		}
	}
}

func TestHomeTilesFailed(t *testing.T) {
	h := loadedHome()
	h.errs[qOutdated] = errors.New("brew outdated: exit status 1")
	tiles := h.tiles(fixtureSnapshot(t))
	if got := tiles[1]; got.value != "✗ failed" || got.detail != "see tab" || got.tab != tabOutdated {
		t.Errorf("outdated tile = %q (%s) -> %d", got.value, got.detail, got.tab)
	}
	if tiles[0].value != "7" || tiles[2].value != "1 error" {
		t.Errorf("other tiles should be unaffected: %q %q", tiles[0].value, tiles[2].value)
	}
	h = homeTab{st: testStyles, errs: map[query]error{qServices: errors.New("boom")}} // failed before ever loading
	if got := h.tiles(state.Snapshot{})[2]; got.value != "✗ failed" {
		t.Errorf("services tile = %q", got.value)
	}
}

func TestHomeCleanupTile(t *testing.T) {
	cleanupTile := func(h homeTab, snap state.Snapshot) string {
		tl := h.tiles(snap)[4]
		return tl.value + " (" + tl.detail + ")"
	}
	h := loadedHome()
	if got := cleanupTile(h, state.Snapshot{CleanupRan: true}); got != "✓ tidy (all clean)" {
		t.Errorf("nothing to clean: %q", got)
	}
	only := state.Snapshot{CleanupRan: true, Cleanup: brew.Cleanup{Standard: brew.CleanupEstimate{Items: 1, Size: "3.9KB"}}}
	if got := cleanupTile(h, only); got != "3.9KB (c to reclaim)" {
		t.Errorf("standard cleanup only: %q", got)
	}
	if got := cleanupTile(homeTab{st: testStyles}, state.Snapshot{}); got != "… (measuring)" {
		t.Errorf("before the dry-runs: %q", got)
	}
	h.errs[qCleanup] = errors.New("brew cleanup --dry-run: exit status 1")
	if got := cleanupTile(h, fixtureSnapshot(t)); got != "✗ failed (r retries)" {
		t.Errorf("failed dry-run: %q", got)
	}
}

func TestHomeNavigation(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := fixtureSnapshot(t)
	if tab := h.update(press("enter"), snap); tab != tabInstalled {
		t.Errorf("enter on first tile -> %d", tab)
	}
	h.update(press("l"), snap)
	h.update(press("right"), snap)
	if tab := h.update(press("enter"), snap); tab != tabServices {
		t.Errorf("enter on third tile -> %d", tab)
	}
	for range 5 {
		h.update(press("l"), snap)
	}
	if h.sel != 4 {
		t.Errorf("selection should stop at the last tile, got %d", h.sel)
	}
	if tab := h.update(press("enter"), snap); tab != openCleanup {
		t.Errorf("enter on cleanup tile -> %d, want openCleanup", tab)
	}
	h.update(press("h"), snap)
	if tab := h.update(press("enter"), snap); tab != homeStay {
		t.Errorf("enter on doctor tile -> %d, want homeStay", tab)
	}
	for range 3 {
		h.update(press("left"), snap)
	}
	if got := h.update(press("h"), snap); got != leaveHome || h.sel != 0 {
		t.Errorf("h on the first tile -> %d (sel %d), want leaveHome", got, h.sel)
	}
}

func TestHomePanesUnderTheTiles(t *testing.T) {
	h := homeTab{st: testStyles}
	snap := fixtureSnapshot(t)
	h.update(press("l"), snap)
	h.update(press("l"), snap) // Services
	h.update(press("j"), snap)
	if h.area != areaDoctor {
		t.Fatalf("j from a tile -> area %d, want Doctor", h.area)
	}
	h.update(press("l"), snap)
	if h.area != areaJobs {
		t.Fatalf("l from Doctor -> area %d, want Recent jobs", h.area)
	}
	if got := h.update(press("enter"), snap); got != showJobs {
		t.Errorf("enter on Recent jobs -> %d, want showJobs", got)
	}
	h.update(press("up"), snap)
	if h.area != areaTiles || h.sel != 2 {
		t.Errorf("k from Recent jobs -> area %d, tile %d; want the Services tile", h.area, h.sel)
	}
	h.update(press("down"), snap)
	h.update(press("right"), snap)
	h.update(press("h"), snap)
	if h.area != areaDoctor {
		t.Errorf("h from Recent jobs -> area %d, want Doctor", h.area)
	}
	if got := h.update(press("h"), snap); got != leaveHome {
		t.Errorf("h from Doctor -> %d, want leaveHome", got)
	}
	h.update(press("k"), snap)
	if h.area != areaTiles || h.sel != 2 {
		t.Errorf("k from Doctor -> area %d, tile %d; want the Services tile", h.area, h.sel)
	}
}

func TestHomeDoctorScrollsBeforeGoingUp(t *testing.T) {
	h := loadedHome()
	snap := fixtureSnapshot(t)
	h.update(press("j"), snap)
	h.view(snap, nil, testNow, 140, 12) // a short pane: doctor's warnings don't fit
	h.update(press("j"), snap)
	if h.doc.off != 1 || h.area != areaDoctor {
		t.Fatalf("j in Doctor should scroll it: off %d, area %d", h.doc.off, h.area)
	}
	h.update(press("k"), snap)
	if h.doc.off != 0 || h.area != areaDoctor {
		t.Fatalf("k should scroll back first: off %d, area %d", h.doc.off, h.area)
	}
	h.update(press("k"), snap)
	if h.area != areaTiles {
		t.Errorf("k at the top of Doctor should go up to the tiles, area %d", h.area)
	}
}

func TestHomeView(t *testing.T) {
	h := loadedHome()
	view := h.view(fixtureSnapshot(t), nil, testNow, 140, 30)
	out := stripANSI(view)
	for _, want := range []string{"Installed", "5 formulae · 2 casks", "lua-language-server,", "1 error", "86.7MB",
		"⚠ The following taps are not trusted:", "brew untap oven-sh/bun", "⚠ Some installed casks are deprecated"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if lipgloss.Height(view) != 30 {
		t.Errorf("home view height = %d, want 30", lipgloss.Height(view))
	}
	clean := stripANSI(h.view(state.Snapshot{DoctorRan: true}, nil, testNow, 140, 30))
	if !strings.Contains(clean, "Your system is ready to brew.") {
		t.Error("clean doctor message missing")
	}
}

func TestHomeDoctorFailed(t *testing.T) {
	h := homeTab{st: testStyles, doctorErr: errors.New("brew doctor: timed out after 30s")}
	if got := h.tiles(state.Snapshot{})[3]; got.value != "✗ failed" || got.detail != "see below" || got.tab != -1 {
		t.Errorf("doctor tile = %+v", got)
	}
	out := stripANSI(h.view(state.Snapshot{}, nil, testNow, 140, 30))
	for _, want := range []string{"brew doctor failed", "timed out after 30s", "press r to retry"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Contains(out, "brew doctor is running") {
		t.Error("failed doctor should not claim to be running")
	}

	out = stripANSI(h.view(fixtureSnapshot(t), nil, testNow, 140, 30))
	for _, want := range []string{"Warnings from the last successful check:", "The following taps are not trusted"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q with earlier warnings", want)
		}
	}
}

func TestHomeShowsRecentJobs(t *testing.T) {
	h := loadedHome()
	for _, size := range [][2]int{{140, 30}, {80, 24}} {
		view := h.view(fixtureSnapshot(t), pastJobs(), testNow, size[0], size[1])
		if lipgloss.Width(view) != size[0] || lipgloss.Height(view) != size[1] {
			t.Errorf("at %v the view is %dx%d", size, lipgloss.Width(view), lipgloss.Height(view))
		}
		out := stripANSI(view)
		for _, want := range []string{"Recent jobs", "✗ upgrade fd", "1h ago", "✓ pin git", "yesterday", "enter shows them all"} {
			if !strings.Contains(out, want) {
				t.Errorf("at %v missing %q:\n%s", size, want, out)
			}
		}
	}
	out := stripANSI(h.view(fixtureSnapshot(t), nil, testNow, 140, 30))
	if !strings.Contains(out, "no jobs yet") {
		t.Errorf("empty recent jobs:\n%s", out)
	}
}

func TestShortDoctorLinesAreNotCutOff(t *testing.T) {
	h := loadedHome()
	snap := state.Snapshot{DoctorRan: true, Doctor: []brew.DoctorWarning{{
		Title: "The following taps are not trusted:",
		Body:  "oven-sh/bun\nHomebrew is currently ignoring formulae, casks and commands from these taps because tap trust is required.",
	}}}
	out := stripANSI(h.view(snap, nil, testNow, 100, 30))
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "oven-sh/bun") && strings.Contains(line, "…") {
			t.Errorf("a short line is padded to the long one and cut off: %q", line)
		}
	}
}

func TestAgo(t *testing.T) {
	for _, tt := range []struct {
		d    time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{30 * time.Hour, "yesterday"},
		{72 * time.Hour, "3d ago"},
	} {
		if got := ago(testNow, testNow.Add(-tt.d)); got != tt.want {
			t.Errorf("ago(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestPlural(t *testing.T) {
	if plural(1, "warning") != "1 warning" || plural(3, "error") != "3 errors" {
		t.Error("plural wrong")
	}
}

func TestHomeOutdatedTileCountsPinned(t *testing.T) {
	h := loadedHome()
	snap := fixtureSnapshot(t)
	snap.Outdated[0].Pinned = true // lua-language-server: U skips it
	if got := h.tiles(snap)[1]; got.value != "2" || got.detail != "1 pinned · docker-desktop" || got.style.GetForeground() != testStyles.Warn.GetForeground() {
		t.Errorf("tile %q (%q)", got.value, got.detail)
	}
	snap.Outdated[1].Pinned = true // nothing left that U would upgrade
	if got := h.tiles(snap)[1]; got.value != "2" || got.detail != "2 pinned · nothing to upgrade" || got.style.GetForeground() != testStyles.Muted.GetForeground() {
		t.Errorf("tile %q (%q)", got.value, got.detail)
	}
}
