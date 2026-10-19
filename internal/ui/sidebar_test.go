package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
)

func TestSidebarHasEveryViewOnce(t *testing.T) {
	seen := map[[2]int]int{}
	for _, e := range entries {
		seen[[2]int{e.tab, e.view}]++
	}
	for f := range views {
		if n := seen[[2]int{tabInstalled, f}]; n != 1 {
			t.Errorf("Installed view %s is in the sidebar %d times", views[f].name, n)
		}
	}
	for v := range discoverViews {
		if n := seen[[2]int{tabDiscover, v}]; n != 1 {
			t.Errorf("Discover view %s is in the sidebar %d times", discoverViews[v].name, n)
		}
	}
	for _, tab := range []int{tabHome, tabOutdated, tabServices, tabTaps} {
		if n := seen[[2]int{tab, 0}]; n != 1 {
			t.Errorf("tab %d is in the sidebar %d times", tab, n)
		}
	}
}

func TestSelectEntryShowsItsTabAndView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, tt := range []struct {
		label string
		tab   int
		check func() bool
	}{
		{"Casks", tabInstalled, func() bool { return a.installed.filter == filterCasks }},
		{"Pinned", tabInstalled, func() bool { return a.installed.filter == filterPinned }},
		{"Rising", tabDiscover, func() bool { return a.discover.showing == discRising }},
		{"Search all", tabDiscover, func() bool { return a.discover.showing == discAll }},
		{"Taps", tabTaps, func() bool { return true }},
	} {
		settle(t, a, a.selectEntry(entryIndex(tt.label)))
		if a.tab != tt.tab || !tt.check() || entries[a.sel].label != tt.label {
			t.Errorf("%s: tab %d, sel %s", tt.label, a.tab, entries[a.sel].label)
		}
	}
}

func TestSelectSectionReturnsToItsLastEntry(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.selectSection(secDiscover))
	if got := entries[a.sel].label; got != "Popular tools" {
		t.Errorf("first visit to Discover: %s, want its first entry", got)
	}
	settle(t, a, a.selectEntry(entryIndex("Taps")))
	settle(t, a, a.selectSection(secHome))
	settle(t, a, a.selectSection(secManage))
	if got := entries[a.sel].label; got != "Taps" {
		t.Errorf("back to Manage: %s, want Taps", got)
	}
}

func TestStepEntryWraps(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	settle(t, a, a.stepEntry(-1))
	if got, last := entries[a.sel].label, entries[len(entries)-1].label; got != last {
		t.Errorf("before Home: %s, want the last entry, %s", got, last)
	}
	settle(t, a, a.stepEntry(1))
	if got := entries[a.sel].label; got != "Home" {
		t.Errorf("after the last entry: %s, want Home", got)
	}
}

func TestSidebarShowsCountsAndChips(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	out := stripANSI(a.sidebarView(sidebarW, 40))
	for _, want := range []string{
		"All 7", "Top-level 5", "Formulae 5", "Casks 2", "Dependencies 2", "Pinned 1",
		fmt.Sprintf("Outdated ↑%d", len(a.snap.Outdated)),
		fmt.Sprintf("Taps %d", len(a.snap.Taps)),
		"PACKAGES", "MANAGE", "DISCOVER", "Search all",
	} {
		if !strings.Contains(squash(out), want) {
			t.Errorf("sidebar lacks %q:\n%s", want, out)
		}
	}
	for i, l := range strings.Split(out, "\n") {
		if w := lipgloss.Width(l); w > sidebarW {
			t.Errorf("line %d is %d wide, want <= %d: %q", i, w, sidebarW, l)
		}
	}
	raw := strings.Split(a.sidebarView(sidebarW, 40), "\n")
	open, _, _ := strings.Cut(a.st.TabActive.Render("x"), "x") // the block's opening escape
	for _, l := range raw {
		if got, want := strings.Contains(l, open), strings.Contains(l, "Home"); got != want {
			t.Errorf("only the selected entry, Home, is a TabActive block: %q", l)
		}
	}
	failing := 1 // the fixtures have one already
	a.snap.Services = append(a.snap.Services, brew.Service{Name: "broken", Status: "error"})
	if out := squash(stripANSI(a.sidebarView(sidebarW, 40))); !strings.Contains(out, fmt.Sprintf("Services ✗%d", failing+1)) {
		t.Errorf("failing services should show a chip: %s", out)
	}
	a.errs[qTaps] = errors.New("offline")
	if out := squash(stripANSI(a.sidebarView(sidebarW, 40))); !strings.Contains(out, "Taps ⚠") {
		t.Errorf("a failed refresh should mark its entry: %s", out)
	}
}

// TestSidebarBadgesLineUp checks that chips end in the same column as the
// plain counts, and that the selected entry's count isn't bold.
func TestSidebarBadgesLineUp(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	end := func(entry string) int {
		for l := range strings.SplitSeq(stripANSI(a.sidebarView(sidebarW, 40)), "\n") {
			if strings.HasPrefix(strings.TrimLeft(l, "│ "), entry+" ") {
				return lipgloss.Width(strings.TrimRight(strings.TrimSuffix(l, "│"), " "))
			}
		}
		t.Fatalf("no %s entry", entry)
		return 0
	}
	if chip, count := end("Outdated"), end("Pinned"); chip != count {
		t.Errorf("the Outdated chip ends in column %d, the Pinned count in %d", chip, count)
	}

	show(t, a, "Dependencies")
	bold := a.st.TabActive.Render(" Dependencies")
	plain := a.st.TabActive.Bold(false).Render(strings.Repeat(" ", sidebarW-4-len(" Dependencies")-2) + "2 ")
	if raw := a.sidebarView(sidebarW, 40); !strings.Contains(raw, bold+plain) {
		t.Errorf("the selected label should be bold and its count not:\n%q", raw)
	}
}

func label(a *App) string { return entries[a.sel].label }

func TestSidebarKeysMoveThroughEntries(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if label(a) != "Home" || a.focus != colSidebar {
		t.Fatalf("brewery should open on Home with the sidebar focused: %s %v", label(a), a.focus == colSidebar)
	}
	keys(t, a, "k")
	if label(a) != "Home" {
		t.Errorf("k at the top: %s", label(a))
	}
	keys(t, a, "j", "j", "j", "j")
	if label(a) != "Casks" || a.tab != tabInstalled || a.installed.filter != filterCasks || a.focus != colSidebar {
		t.Errorf("4×j: %s tab %d filter %d focus %v", label(a), a.tab, a.installed.filter, a.focus == colSidebar)
	}
	keys(t, a, "G")
	if last := entries[len(entries)-1].label; label(a) != last {
		t.Errorf("G: %s, want the last entry, %s", label(a), last)
	}
	keys(t, a, "g", "g")
	if label(a) != "Home" {
		t.Errorf("gg: %s", label(a))
	}
}

func TestFocusMovesAcrossColumns(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "l")
	if a.focus != colList {
		t.Fatalf("l from the sidebar should focus the list")
	}
	keys(t, a, "l")
	if a.focus != colDetails {
		t.Fatalf("l from the list should focus the details")
	}
	keys(t, a, "h")
	if a.focus != colList {
		t.Fatalf("h from the details should focus the list")
	}
	keys(t, a, "h")
	if a.focus != colSidebar {
		t.Fatalf("h from the list should focus the sidebar")
	}
	keys(t, a, "enter")
	if a.focus == colSidebar {
		t.Errorf("enter in the sidebar should focus the list")
	}
}

func TestNumberKeysJumpToSections(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "4", "j")
	if label(a) != "Popular apps" || a.focus != colSidebar {
		t.Fatalf("4 then j: %s, focus %v", label(a), a.focus == colSidebar)
	}
	keys(t, a, "1", "4")
	if label(a) != "Popular apps" {
		t.Errorf("4 should return to Discover's last entry: %s", label(a))
	}
	keys(t, a, "3")
	if label(a) != "Services" {
		t.Errorf("3: %s", label(a))
	}
}

func TestBracketsStepEntriesAndKeepTheFocus(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Taps")
	keys(t, a, "]")
	if label(a) != "Popular tools" || a.focus == colSidebar {
		t.Errorf("] from Taps: %s, focus %v", label(a), a.focus == colSidebar)
	}
	keys(t, a, "[")
	if label(a) != "Taps" {
		t.Errorf("[: %s", label(a))
	}
}

func TestRowKeysDoNothingInTheSidebar(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	keys(t, a, "j", "u", "x", "p", "space", "y")
	if len(fs.calls) != 0 || a.confirm != nil || a.marks() != 0 {
		t.Errorf("row keys in the sidebar acted: %v %+v %d", fs.calls, a.confirm, a.marks())
	}
	keys(t, a, "U")
	if got := lastCall(fs); len(got) == 0 || got[0] != "update" {
		t.Errorf("U should work from the sidebar: %v", got)
	}
}

func TestSlashInTheSidebarJumps(t *testing.T) {
	a := newJobApp(t, &fakeStarter{}) // the focus starts on the sidebar, on Home
	keys(t, a, "/")
	if a.mode != modeFilter || !a.jumping || a.focus != colSidebar {
		t.Fatalf("/ in the sidebar should open the jump: mode %d, jumping %v, focus %v", a.mode, a.jumping, a.focus)
	}
	typeText(t, a, "maint")
	if label(a) != "Maintenance" {
		t.Errorf("/maint selects %q, want Maintenance", label(a))
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "JUMP") || !strings.Contains(bar, "enter open · esc back") {
		t.Errorf("status bar while jumping: %q", bar)
	}
	keys(t, a, "enter")
	if a.mode != modeNormal || a.jumping || a.focus != colList || label(a) != "Maintenance" {
		t.Errorf("enter opens the entry: mode %d, jumping %v, focus %v, entry %q", a.mode, a.jumping, a.focus, label(a))
	}
}

func TestJumpMatchesTheFirstEntryIgnoringCase(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	typeText(t, a, "S")
	if label(a) != "Casks" {
		t.Errorf("/S selects %q, want Casks, the first label with an s", label(a))
	}
	typeText(t, a, "zz")
	if label(a) != "Casks" || !strings.Contains(stripANSI(a.statusBar()), "no match · esc back") {
		t.Errorf("no match keeps the last match and says so: %q, %q", label(a), stripANSI(a.statusBar()))
	}
	keys(t, a, "enter")
	if a.mode != modeNormal || a.focus != colSidebar {
		t.Errorf("enter with no match only closes the prompt: mode %d, focus %v", a.mode, a.focus)
	}
}

func TestJumpGoesBack(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "j", "j") // Top-level
	keys(t, a, "/")
	typeText(t, a, "hist")
	keys(t, a, "esc")
	if label(a) != "Top-level" || a.mode != modeNormal || a.jumping || a.focus != colSidebar {
		t.Errorf("esc goes back to where the jump started: %q, mode %d, focus %v", label(a), a.mode, a.focus)
	}
	keys(t, a, "/")
	typeText(t, a, "taps")
	keys(t, a, "backspace", "backspace", "backspace", "backspace")
	if label(a) != "Top-level" {
		t.Errorf("clearing the text goes back: %q", label(a))
	}
	keys(t, a, "esc")
}

func TestJumpTakesAPaste(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "/")
	_, cmd := a.Update(tea.PasteMsg{Content: "history"})
	settle(t, a, cmd)
	if label(a) != "History" {
		t.Errorf("a pasted jump selects %q, want History", label(a))
	}
}

func TestSidebarHelpAndHintMentionTheJump(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "j/k choose · / jump · l open · 1-5 sections") {
		t.Errorf("sidebar hint: %q", bar)
	}
	keys(t, a, "?")
	if out := stripANSI(a.render()); !strings.Contains(out, "jump to an entry") {
		t.Errorf("help lacks the jump:\n%s", out)
	}
}

func TestSidebarKeepsItsSelectedEntryInView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 120, Height: 20})
	for _, l := range []string{"Search all", "Home", "Pinned", "Fonts"} {
		settle(t, a, a.selectEntry(entryIndex(l)))
		out := a.sidebarView(sidebarW, 20-4)
		if !strings.Contains(squash(stripANSI(out)), l) {
			t.Errorf("%s is out of view:\n%s", l, stripANSI(out))
		}
		if n := lipgloss.Height(out); n != 20-4 {
			t.Errorf("%s: sidebar is %d lines, want %d", l, n, 20-4)
		}
		if strings.Contains(stripANSI(out), "more") {
			t.Errorf("%s: the pane cut the sidebar:\n%s", l, stripANSI(out))
		}
		if full := stripANSI(a.render()); !strings.Contains(squash(full), l) {
			t.Errorf("%s is out of view in the 120×20 screen:\n%s", l, full)
		}
	}
}

func TestHomeTileOpensItsEntry(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "enter") // Home's first tile, then the Outdated tile
	if label(a) != "Outdated" || a.focus == colSidebar {
		t.Errorf("enter on the Outdated tile: %s, focus %v", label(a), a.focus == colSidebar)
	}
}

func TestHomeKeysWalkTheTilesAndBackToTheSidebar(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	keys(t, a, "l", "l", "l")
	if a.focus == colSidebar || a.home.sel != 2 {
		t.Fatalf("l from the sidebar, then l twice: focus %v, tile %d; want the Services tile", a.focus == colSidebar, a.home.sel)
	}
	keys(t, a, "h", "h", "h")
	if a.focus != colSidebar || label(a) != "Home" {
		t.Fatalf("h past the first tile should reach the sidebar: focus %v, %s", a.focus == colSidebar, label(a))
	}
	keys(t, a, "l")
	if a.home.sel != 0 || a.home.area != areaTiles {
		t.Errorf("l from the sidebar should start on Installed: tile %d, area %d", a.home.sel, a.home.area)
	}
	keys(t, a, "j", "h")
	if a.focus != colSidebar {
		t.Errorf("h from Doctor should reach the sidebar")
	}
}

func TestEntryKeysKeepTheFocusInItsColumn(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "All")
	keys(t, a, "l")
	if a.focus != colDetails {
		t.Fatalf("l from the list: focus %d, want details", a.focus)
	}
	keys(t, a, "]") // Top-level: Installed again, with details
	if label(a) != "Top-level" || a.focus != colDetails {
		t.Errorf("] keeps the details focus: %s, focus %d", label(a), a.focus)
	}
	keys(t, a, "3") // Services
	if a.focus != colDetails {
		t.Errorf("3 keeps the details focus on Services: %d", a.focus)
	}
	keys(t, a, "4") // Popular tools: no catalogue here, so no details yet
	if a.tab != tabDiscover || a.focus != colList {
		t.Errorf("4 onto an unloaded Discover: tab %d, focus %d, want the list", a.tab, a.focus)
	}
	keys(t, a, "3", "l")
	keys(t, a, "]") // Taps has no details: the focus is its list's
	if label(a) != "Taps" || a.focus != colList {
		t.Errorf("] onto Taps: %s, focus %d, want the list", label(a), a.focus)
	}
	keys(t, a, "h")
	if a.focus != colSidebar {
		t.Errorf("h from the list: focus %d, want the sidebar", a.focus)
	}
	keys(t, a, "2")
	if a.focus != colSidebar {
		t.Errorf("2 from the sidebar keeps the sidebar focus: %d", a.focus)
	}
}

func TestStatusTrailNamesTheSelectedService(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	show(t, a, "Services")
	s, ok := a.selectedService()
	if !ok {
		t.Fatal("no service selected")
	}
	if bar := stripANSI(a.statusBar()); !strings.Contains(bar, "Services › "+s.Name) {
		t.Errorf("status bar lacks %q:\n%s", "Services › "+s.Name, bar)
	}
}
