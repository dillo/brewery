package ui

import (
	"errors"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
)

func TestListTabRendersRows(t *testing.T) {
	snap := fixtureSnapshot(t)
	lt := newListTab(testStyles, "Outdated", outdatedCols, "all good")
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
	lt := newListTab(testStyles, "Outdated", outdatedCols, "✓ everything is up to date")
	lt.resize(80, 8)
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
