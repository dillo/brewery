package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, outdatedKey, "all good")
	lt.resize(120, 10)
	lt.setRows(outdatedRows(snap.Outdated))
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
	if got := serviceRows(snap.Services)[0]; got[0] != "postgresql@16" || got[1] != "error (exit 1)" {
		t.Errorf("service row = %v", got)
	}
	if got := serviceRows(snap.Services)[1][1]; got != "none" {
		t.Errorf("status without exit code = %q", got)
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
	// fixed: 18+18+7+6 = 49, plus 2 padding per column (10) = 59; flex gets 100-59.
	if got := tbl.Columns()[0].Width; got != 41 {
		t.Errorf("flex column width = %d, want 41", got)
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

func TestSizeTableShrinksFixedColumnsFromTheEnd(t *testing.T) {
	tbl := newTable(testStyles, installedCols)
	sizeTable(&tbl, installedCols, 40, 10)
	// fixed 16+7+10 plus NAME's minimum 8, plus 8 padding = 49: 9 too many.
	// STATUS shrinks to its title (6), TYPE to its title (4), VERSION by 2.
	got := []int{}
	for _, c := range tbl.Columns() {
		got = append(got, c.Width)
	}
	if want := []int{8, 14, 4, 6}; fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("widths = %v, want %v", got, want)
	}
	if w := tableWidth(tbl); w > 40 {
		t.Errorf("table is %d wide, want <= 40", w)
	}
}

func TestSizeTableDropsTrailingColumns(t *testing.T) {
	tbl := newTable(testStyles, outdatedCols)
	sizeTable(&tbl, outdatedCols, 30, 10)
	// NAME 8 + INSTALLED 9 + LATEST 6 + padding 6 = 29; TYPE and PINNED don't fit.
	if w := tableWidth(tbl); w > 30 {
		t.Errorf("table is %d wide, want <= 30", w)
	}
	if tbl.Columns()[0].Width < 8 {
		t.Errorf("NAME width = %d, want >= 8", tbl.Columns()[0].Width)
	}
	setRows(&tbl, outdatedRows(fixtureSnapshot(t).Outdated)) // rows still carry every cell
	out := stripANSI(tbl.View())
	if !strings.Contains(out, "lua-lan") || strings.Contains(out, "PINNED") || strings.Contains(out, "TYPE") {
		t.Errorf("dropped columns should not render, rows should:\n%s", out)
	}
	for _, line := range strings.Split(tbl.View(), "\n") {
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
	lt.setRows(outdatedRows(fixtureSnapshot(t).Outdated))
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
	lt.setRows([]table.Row{{"mysql", "", "", ""}, {"postgresql@16", "", "", ""}, {"redis", "", "", ""}})
	lt.update(press("j"))
	lt.update(press("j")) // redis
	lt.setRows([]table.Row{{"nginx", "", "", ""}, {"postgresql@16", "", "", ""}, {"redis", "", "", ""}, {"valkey", "", "", ""}})
	if got := lt.table.SelectedRow()[0]; got != "redis" {
		t.Errorf("selection moved to %q after refresh, want redis", got)
	}
	if lt.cursor() != 2 {
		t.Errorf("cursor() = %d, want 2", lt.cursor())
	}
	lt.setRows([]table.Row{{"nginx", "", "", ""}})
	if lt.cursor() != 0 {
		t.Errorf("when the item is gone the cursor stays in range, got %d", lt.cursor())
	}
	lt.setRows(nil)
	if lt.cursor() != -1 {
		t.Errorf("cursor() on an empty table = %d, want -1", lt.cursor())
	}
}
