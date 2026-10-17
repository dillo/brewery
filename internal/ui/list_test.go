package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/theme"
)

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(theme.Icons{}, snap.Outdated))
	out := stripANSI(lt.view(nil))
	for _, want := range []string{"Outdated · 2", "NAME", "lua-language-server", "3.19.1", "3.19.2", "4.95.0,242831", "cask"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if h, w := lipgloss.Height(lt.view(nil)), lipgloss.Width(lt.view(nil)); h != 10 || w != 120 {
		t.Errorf("rendered %dx%d, want 120x10", w, h)
	}
}

func TestListTabEmptyAndError(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "✓ everything is up to date")
	lt.resize(80, 8)
	if out := stripANSI(lt.view(nil)); !strings.Contains(out, "loading…") || strings.Contains(out, "up to date") {
		t.Errorf("before its first result the tab should say it is loading:\n%s", out)
	}
	lt.loaded = true
	if out := stripANSI(lt.view(nil)); !strings.Contains(out, "everything is up to date") {
		t.Errorf("empty message missing:\n%s", out)
	}
	out := stripANSI(lt.view(errors.New("brew outdated --json=v2: exit status 1")))
	if !strings.Contains(out, "couldn't load this view") || !strings.Contains(out, "exit status 1") || !strings.Contains(out, "press r to retry") {
		t.Errorf("error body missing:\n%s", out)
	}
}

func TestServiceAndTapRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	rows := serviceRows(testStyles, snap.Services)
	if got := rows[0]; got[0] != "postgresql@16" || stripANSI(got[1]) != "error (exit 1)" {
		t.Errorf("service row = %q", got)
	}
	if got := rows[1][1]; got != theme.Tint(testStyles.P.Green, "started") {
		t.Errorf("a started service should be green, without an exit code: %q", got)
	}
	if got := rows[0][1]; got != theme.Tint(testStyles.P.Red, "error (exit 1)") {
		t.Errorf("a failed service should be red: %q", got)
	}
	if got := tapRows(snap.Taps)[0]; got[1] != "2" || got[2] != "1" || got[3] != "" {
		t.Errorf("tap row = %v", got)
	}
}

func TestSetRowsRecoversCursorAfterEmpty(t *testing.T) {
	tbl := newTable(testStyles, outdatedCols)
	setRows(&tbl, nil) // an empty table parks the cursor at -1
	setRows(&tbl, []table.Row{{"a", "", "", "", ""}, {"b", "", "", "", ""}})
	if tbl.Cursor() != 0 {
		t.Errorf("cursor = %d, want 0", tbl.Cursor())
	}
	tbl.SetCursor(1)
	setRows(&tbl, []table.Row{{"a", "", "", "", ""}})
	if tbl.Cursor() != 0 {
		t.Errorf("cursor = %d after shrinking, want 0", tbl.Cursor())
	}
}

func TestSizeTableSplitsFlexWidth(t *testing.T) {
	tbl := newTable(testStyles, outdatedCols)
	sizeTable(&tbl, outdatedCols, 100, 10)
	// fixed: 18+18+8+9 = 53, plus 2 padding per column (10) = 63; flex gets 100-63.
	if got := tbl.Columns()[0].Width; got != 37 {
		t.Errorf("flex column width = %d, want 37", got)
	}
}

// tableWidth is the rendered width of t's columns: each width plus 2 padding.
func tableWidth(tbl table.Model) int {
	n := 0
	for _, c := range tbl.Columns() {
		if c.Width > 0 {
			n += c.Width + 2
		}
	}
	return n
}

func TestSizeTableShrinksTheRoomiestColumnsFirst(t *testing.T) {
	widths := func(w int) string {
		tbl := newTable(testStyles, installedCols)
		sizeTable(&tbl, installedCols, w, 10)
		got := []int{}
		for _, c := range tbl.Columns() {
			got = append(got, c.Width)
		}
		if tw := tableWidth(tbl); tw > w {
			t.Errorf("at %d the table is %d wide", w, tw)
		}
		return fmt.Sprint(got)
	}
	// NAME wants 20; VERSION 16, TYPE 9, STATUS 10 and SIZE 8 (optional).
	for _, tt := range []struct {
		w    int
		want string
		why  string
	}{
		{62, "[20 10 6 8 8]", "VERSION gives way first, then STATUS and TYPE"},
		{50, "[24 8 4 6 0]", "fixed columns at their floors, then SIZE goes before NAME gives way"},
		{40, "[14 8 4 6 0]", "and only then does NAME shrink"},
	} {
		if got := widths(tt.w); got != tt.want {
			t.Errorf("at %d: widths %s, want %s: %s", tt.w, got, tt.want, tt.why)
		}
	}
}

func TestSizeTableDropsTrailingColumns(t *testing.T) {
	tbl := newTable(testStyles, outdatedCols)
	sizeTable(&tbl, outdatedCols, 30, 10)
	// NAME 8 + INSTALLED 8 + LATEST 8 + padding 6 = 30; TYPE and PINNED don't fit.
	if w := tableWidth(tbl); w > 30 {
		t.Errorf("table is %d wide, want <= 30", w)
	}
	if tbl.Columns()[0].Width < 8 {
		t.Errorf("NAME width = %d, want >= 8", tbl.Columns()[0].Width)
	}
	setRows(&tbl, outdatedRows(theme.Icons{}, fixtureSnapshot(t).Outdated)) // rows still carry every cell
	out := stripANSI(tbl.View())
	if !strings.Contains(out, "lua-lan") || strings.Contains(out, "PINNED") || strings.Contains(out, "TYPE") {
		t.Errorf("dropped columns should not render, rows should:\n%s", out)
	}
	for line := range strings.SplitSeq(tbl.View(), "\n") {
		if lipgloss.Width(line) > 30 {
			t.Errorf("line is %d wide: %q", lipgloss.Width(line), stripANSI(line))
		}
	}
}

func TestPaneClipsLongBodyAboveTheBorder(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d with a tail far too wide for this pane to hold", i)
	}
	out := pane(testStyles, "Title", strings.Join(lines, "\n"), 40, 10, false)
	rows := strings.Split(stripANSI(out), "\n")
	if lipgloss.Height(out) != 10 || lipgloss.Width(out) != 40 {
		t.Errorf("pane is %dx%d, want 40x10", lipgloss.Width(out), lipgloss.Height(out))
	}
	if last := rows[len(rows)-1]; !strings.HasPrefix(last, "╰") {
		t.Errorf("bottom border missing, last line %q", last)
	}
	if !strings.Contains(stripANSI(out), "… 44 more") || !strings.Contains(stripANSI(out), "line 5 ") {
		t.Errorf("want 6 body lines then a count of the rest:\n%s", stripANSI(out))
	}
}

func TestListTabErrorWithRowsShowsBanner(t *testing.T) {
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(theme.Icons{}, fixtureSnapshot(t).Outdated))
	view := lt.view(errors.New("brew outdated --json=v2: exit status 1\nError: second line"))
	out := stripANSI(view)
	for _, want := range []string{"refresh failed · r to retry", "exit status 1", "lua-language-server", "docker-desktop"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "second line") || strings.Contains(out, "couldn't load") {
		t.Errorf("banner should be one line of error above the rows:\n%s", out)
	}
	if h := lipgloss.Height(view); h != 10 {
		t.Errorf("height = %d, want 10", h)
	}
	if rows := strings.Split(out, "\n"); !strings.HasPrefix(rows[len(rows)-1], "╰") {
		t.Errorf("bottom border missing:\n%s", out)
	}
}

func TestListTabKeepsSelectionAcrossRefresh(t *testing.T) {
	lt := newListTab(testStyles, "Services", serviceCols, firstCell, "none")
	lt.resize(120, 10)
	lt.setRows([]table.Row{{"mysql", ""}, {"postgresql@16", ""}, {"redis", ""}})
	lt.update(press("j"))
	lt.update(press("j")) // redis
	lt.setRows([]table.Row{{"nginx", ""}, {"postgresql@16", ""}, {"redis", ""}, {"valkey", ""}})
	if got := lt.table.SelectedRow()[0]; got != "redis" {
		t.Errorf("selection moved to %q after refresh, want redis", got)
	}
	if lt.cursor() != 2 {
		t.Errorf("cursor() = %d, want 2", lt.cursor())
	}
	lt.setRows([]table.Row{{"nginx", ""}})
	if lt.cursor() != 0 {
		t.Errorf("when the item is gone the cursor stays in range, got %d", lt.cursor())
	}
	lt.setRows(nil)
	if lt.cursor() != -1 {
		t.Errorf("cursor() on an empty table = %d, want -1", lt.cursor())
	}
}

func TestSkeletonRowsBeforeTheFirstResult(t *testing.T) {
	lt := newListTab(testStyles, "Taps", tapCols, firstCell, "none")
	lt.resize(60, 20)
	out := stripANSI(lt.view(nil))
	n := 0
	for l := range strings.SplitSeq(out, "\n") {
		if strings.Contains(l, "░") {
			n++
		}
	}
	if n != skeletonRows {
		t.Errorf("want %d placeholder rows, got %d:\n%s", skeletonRows, n, out)
	}
	it := newInstalledTab(testStyles)
	it.resize(140, 6)
	it.setData(nil, state.NewIndex(nil))
	out = stripANSI(it.view(nil))
	if !strings.Contains(out, "░░░░") || lipgloss.Height(it.view(nil)) != 6 {
		t.Errorf("Installed shows placeholders too, within its height:\n%s", out)
	}
	if got := skeleton(testStyles, 10, 3); lipgloss.Width(got) > 10 || lipgloss.Height(got) != 3 {
		t.Errorf("skeleton is %dx%d, want at most 10 wide and 3 tall", lipgloss.Width(got), lipgloss.Height(got))
	}
}

func TestIconsInRows(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.UseIcons(true)
	if got := a.outdated.all[1][4]; got != theme.Nerd.Cask+" cask" {
		t.Errorf("outdated TYPE with icons = %q", got)
	}
	if got := stripANSI(a.services.all[0][1]); got != theme.Nerd.Failed+" error (exit 1)" {
		t.Errorf("service status with icons = %q", got)
	}
	show(t, a, "Top-level")
	keys(t, a, "j", "j", "j") // git, pinned
	if p, _ := a.installed.selected(); p.Name != "git" {
		t.Fatalf("selected %s", p.Name)
	}
	row := a.installed.table.SelectedRow()
	if row[2] != theme.Nerd.Formula+" formula" || row[3] != theme.Nerd.Pinned+" pinned" {
		t.Errorf("installed row with icons = %q", row)
	}
	a.UseIcons(false)
	if got := a.outdated.all[1][4]; got != "cask" {
		t.Errorf("icons off: %q", got)
	}
}

func TestSelectedRowStaysHighlighted(t *testing.T) {
	lt := newListTab(testStyles, "Services", serviceCols, firstCell, "none")
	lt.resize(60, 10)
	lt.setRows(serviceRows(testStyles, fixtureSnapshot(t).Services))
	found := false
	for line := range strings.SplitSeq(lt.table.View(), "\n") {
		if !strings.Contains(line, "postgresql@16") {
			continue
		}
		found = true
		// One full reset, at the very end: anything earlier would end the
		// highlight partway along the row.
		if n := strings.Count(line, "\x1b[m"); n != 1 || !strings.HasSuffix(line, "\x1b[m") {
			t.Errorf("the selected row resets %d times: %q", n, line)
		}
	}
	if !found {
		t.Errorf("the selected row isn't shown:\n%s", lt.table.View())
	}
}

func TestSkeletonOnTinyPanes(t *testing.T) {
	for _, n := range []int{-1, 0} {
		if got := skeleton(testStyles, 10, n); got != "" {
			t.Errorf("skeleton with %d rows = %q, want empty", n, got)
		}
	}
	lt := newListTab(testStyles, "Taps", tapCols, firstCell, "none")
	lt.resize(40, 3)
	_ = lt.view(nil)
	it := newInstalledTab(testStyles)
	it.resize(40, 4)
	it.setData(nil, state.NewIndex(nil))
	_ = it.view(nil)
}

func TestServiceStatusFitsWithAnIcon(t *testing.T) {
	widest := theme.With(theme.Nerd.Failed, "error (exit 127)")
	if w := lipgloss.Width(widest); w > serviceCols[1].width {
		t.Errorf("%q is %d wide; STATUS has %d", widest, w, serviceCols[1].width)
	}
}

func TestOutdatedKeepsPinnedLongerThanType(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.Update(tea.WindowSizeMsg{Width: 46, Height: 24})
	show(t, a, "Outdated")
	if out := stripANSI(a.render()); !strings.Contains(out, "PINNED") || strings.Contains(out, "TYPE") {
		t.Errorf("a narrow Outdated should drop TYPE before PINNED:\n%s", out)
	}
}

func TestOutdatedCellsFitWithIcons(t *testing.T) {
	ic := theme.Nerd
	for _, c := range []struct {
		col  int
		cell string
	}{{3, theme.With(ic.Pinned, "pinned")}, {4, kindCell(ic, brew.KindFormula)}} {
		if w := lipgloss.Width(c.cell); w > outdatedCols[c.col].width {
			t.Errorf("%q is %d wide; %s has %d", c.cell, w, outdatedCols[c.col].title, outdatedCols[c.col].width)
		}
	}
}
