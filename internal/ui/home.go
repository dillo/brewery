package ui

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/jobs"
	"github.com/dillo/brewery/internal/state"
	"github.com/dillo/brewery/internal/theme"
)

// tile is one health summary on the Home tab.
type tile struct {
	title  string
	value  string
	style  lipgloss.Style
	detail string
	entry  string // the sidebar entry Enter opens; "" for none
	row    int    // the Maintenance row Enter selects; -1 for none
}

// What a key on Home asks the app to do, besides opening a tile's entry
// (a tile's index, 0 or more).
const (
	homeStay    = -1 // nothing: the key moved within Home, or did nothing
	leaveHome   = -3 // h past the left edge: back to the sidebar
	showHistory = -4 // Enter on Recent jobs: History, with them all
)

// homeArea is the part of Home that has the focus.
type homeArea int

const (
	areaTiles homeArea = iota
	areaDoctor
	areaJobs
)

// homeTab shows health tiles and doctor's warnings.
type homeTab struct {
	st        theme.Styles
	sel       int      // the tile with the focus, and the one k goes back up to
	area      homeArea // the tiles, or one of the panes under them
	doc       scroller // the Doctor pane, which j scrolls once it has the focus
	docPage   int      // half the Doctor pane's height, for ctrl+d and ctrl+u
	doctorErr error    // last doctor failure; nil once a run succeeds
	errs      map[query]error
	loaded    map[query]bool
}

func (h *homeTab) tiles(snap state.Snapshot) []tile {
	st := h.st
	formulae, casks := 0, 0
	for _, p := range snap.Packages {
		if p.Kind == brew.KindCask {
			casks++
		} else {
			formulae++
		}
	}
	installed := tile{"Installed", fmt.Sprint(len(snap.Packages)), st.Title,
		fmt.Sprintf("%d formulae · %d casks", formulae, casks), "All", -1}

	outdated := tile{"Outdated", "✓ 0", st.OK, "everything is up to date", "Outdated", -1}
	if n := len(snap.Outdated); n > 0 {
		// Pinned packages are outdated too, but U skips them: count them
		// first, where a narrow tile still shows it, then name the rest.
		names, pinned := make([]string, 0, n), 0
		for _, o := range snap.Outdated {
			if o.Pinned {
				pinned++
			} else {
				names = append(names, o.Name)
			}
		}
		outdated.value, outdated.style, outdated.detail = fmt.Sprint(n), st.Warn, strings.Join(names, ", ")
		switch {
		case pinned == n:
			outdated.style, outdated.detail = st.Muted, fmt.Sprintf("%d pinned · nothing to upgrade", n)
		case pinned > 0:
			outdated.detail = fmt.Sprintf("%d pinned · %s", pinned, outdated.detail)
		}
	}

	running, failed := 0, []string{}
	for _, s := range snap.Services {
		switch s.Status {
		case "started", "scheduled":
			running++
		case "error":
			failed = append(failed, s.Name)
		}
	}
	services := tile{"Services", fmt.Sprintf("%d running", running), st.OK,
		fmt.Sprintf("%d registered", len(snap.Services)), "Services", -1}
	if len(failed) > 0 {
		services.value, services.style, services.detail = plural(len(failed), "error"), st.Err, strings.Join(failed, ", ")
	}

	h.markUnknown(&installed, qInstalled)
	h.markUnknown(&outdated, qOutdated)
	h.markUnknown(&services, qServices)

	doctor := tile{"Doctor", "…", st.Muted, "checking", "Maintenance", maintDoctor}
	switch {
	case h.doctorErr != nil:
		doctor.value, doctor.style, doctor.detail = "✗ failed", st.Err, "see below"
	case snap.DoctorRan:
		doctor.value, doctor.style, doctor.detail = "✓ clean", st.OK, "no warnings"
		if n := len(snap.Doctor); n > 0 {
			doctor.value, doctor.style, doctor.detail = plural(n, "warning"), st.Warn, "see below"
		}
	}
	cleanup := tile{"Cleanup", "✓ tidy", st.OK, "all clean", "Maintenance", maintCleanup}
	switch size := cmp.Or(snap.Cleanup.All.Size, snap.Cleanup.Standard.Size); {
	case h.errs[qCleanup] != nil:
		cleanup.value, cleanup.style, cleanup.detail = "✗ failed", st.Err, "r retries"
	case !h.loaded[qCleanup]:
		cleanup.value, cleanup.style, cleanup.detail = "…", st.Muted, "measuring"
	case size != "":
		cleanup.value, cleanup.style, cleanup.detail = size, st.Title, "c to reclaim"
	}
	return []tile{installed, outdated, services, doctor, cleanup}
}

// markUnknown replaces a tile's numbers when its query has failed or has
// not returned yet, so zeros are never shown as a healthy result.
func (h *homeTab) markUnknown(t *tile, q query) {
	switch {
	case h.errs[q] != nil:
		t.value, t.style, t.detail = "✗ failed", h.st.Err, "see tab"
	case !h.loaded[q]:
		t.value, t.style, t.detail = "…", h.st.Muted, "loading"
	}
}

// enter puts the focus on the first tile, as l from the sidebar does.
func (h *homeTab) enter() {
	h.sel, h.area = 0, areaTiles
}

// update moves the focus around Home: h and l along the tiles and across
// the panes under them, j down from the tiles to Doctor, k back up to the
// tile you came from. It returns the index of a tile whose entry to open,
// or one of homeStay, leaveHome and showHistory.
func (h *homeTab) update(msg tea.KeyPressMsg, snap state.Snapshot) int {
	k := msg.String()
	switch h.area {
	case areaTiles:
		return h.tileKey(k, snap)
	case areaDoctor:
		switch k {
		case "l", "right":
			h.area = areaJobs
		case "h", "left":
			return leaveHome
		case "k", "up":
			if h.doc.off == 0 {
				h.area = areaTiles // at the top of the pane: up to the tiles
			} else {
				h.doc.scroll(k, h.docPage)
			}
		default:
			h.doc.scroll(k, h.docPage)
		}
	case areaJobs:
		switch k {
		case "h", "left":
			h.area = areaDoctor
		case "k", "up":
			h.area = areaTiles
		case "enter":
			return showHistory
		}
	}
	return homeStay
}

// tileKey handles a key while a tile has the focus.
func (h *homeTab) tileKey(k string, snap state.Snapshot) int {
	tiles := h.tiles(snap)
	switch k {
	case "l", "right":
		h.sel = min(h.sel+1, len(tiles)-1)
	case "h", "left":
		if h.sel == 0 {
			return leaveHome
		}
		h.sel--
	case "j", "down":
		h.area = areaDoctor
	case "enter":
		if tiles[h.sel].entry != "" {
			return h.sel
		}
	}
	return homeStay
}

// view renders the tiles over doctor's findings and the recent jobs
// (newest first).
func (h *homeTab) view(snap state.Snapshot, recent []jobs.Record, now time.Time, w, height int, focus column) string {
	st := h.st
	tiles := h.tiles(snap)
	tileH := 5
	tileW := w / len(tiles)
	parts := make([]string, len(tiles))
	for i, t := range tiles {
		tw := tileW
		if i == len(tiles)-1 {
			tw = w - tileW*(len(tiles)-1)
		}
		iw, _ := paneInner(tw, tileH)
		body := t.style.Bold(true).Render(t.value) + "\n" + st.Muted.Render(ansi.Truncate(t.detail, iw, "…"))
		parts[i] = pane(st, t.title, body, tw, tileH, i == h.sel && h.area == areaTiles && focus != colSidebar)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	lowH := max(height-tileH, 3)
	doctorW := w * 3 / 5
	_, docH := paneInner(doctorW, lowH)
	h.docPage = max(docH/2, 1)
	doctor := pane(st, "Doctor", h.doc.window(doctorBody(st, snap, h.doctorErr), docH), doctorW, lowH, h.area == areaDoctor && focus != colSidebar)
	jobsPane := pane(st, "Recent jobs", h.recentJobs(recent, now, w-doctorW, lowH), w-doctorW, lowH, h.area == areaJobs && focus != colSidebar)
	return lipgloss.JoinVertical(lipgloss.Left, row, lipgloss.JoinHorizontal(lipgloss.Top, doctor, jobsPane))
}

// recentJobs lists as many recent jobs as fit a w×h pane, each with how
// long ago it started, above a hint for the full list.
func (h *homeTab) recentJobs(recent []jobs.Record, now time.Time, w, height int) string {
	st := h.st
	iw, ih := paneInner(w, height)
	hint := st.Muted.Render("5 History has them all")
	if len(recent) == 0 {
		return st.Muted.Render("no jobs yet") + "\n" + hint
	}
	var lines []string
	for _, r := range recent[:min(len(recent), max(ih-1, 0))] {
		icon := st.OK.Render("✓")
		switch r.State {
		case jobs.Failed:
			icon = st.Err.Render("✗")
		case jobs.Canceled:
			icon = st.Warn.Render("⊘")
		}
		when := st.Muted.Render(ago(now, r.Started))
		title := ansi.Truncate(r.Title, max(iw-4-lipgloss.Width(when), 1), "…")
		gap := max(iw-2-lipgloss.Width(title)-lipgloss.Width(when), 1)
		lines = append(lines, icon+" "+title+strings.Repeat(" ", gap)+when)
	}
	return strings.Join(append(lines, hint), "\n")
}

// ago says roughly how long before now t was.
func ago(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

// doctorBody is what doctor found, as Home's Doctor pane and Maintenance's
// details show it: its warnings, that it is running, or that it failed
// (err), with the warnings from the last check that worked.
func doctorBody(st theme.Styles, snap state.Snapshot, err error) string {
	var b strings.Builder
	switch {
	case err != nil:
		b.WriteString(st.Err.Render("✗ brew doctor failed") + "\n\n")
		b.WriteString(st.Muted.Render(err.Error()) + "\n\n")
		b.WriteString(st.Muted.Render("press r to retry") + "\n")
		if snap.DoctorRan && len(snap.Doctor) > 0 {
			b.WriteString("\n" + st.Muted.Render("Warnings from the last successful check:") + "\n\n")
			writeWarnings(st, &b, snap.Doctor)
		}
	case !snap.DoctorRan:
		b.WriteString(st.Muted.Render("brew doctor is running…"))
	case len(snap.Doctor) == 0:
		b.WriteString(st.OK.Render("✓ Your system is ready to brew."))
	default:
		writeWarnings(st, &b, snap.Doctor)
	}
	return b.String()
}

// writeWarnings renders each doctor warning as a title and indented body.
func writeWarnings(st theme.Styles, b *strings.Builder, warnings []brew.DoctorWarning) {
	for _, d := range warnings {
		b.WriteString(st.Warn.Render("⚠ "+d.Title) + "\n")
		if d.Body != "" {
			// Style each line alone: as one block, every line would be
			// padded to the longest, and a narrow pane cuts the padding off.
			for line := range strings.SplitSeq(d.Body, "\n") {
				b.WriteString(st.Muted.Render("  "+line) + "\n")
			}
		}
		b.WriteString("\n")
	}
}

// plural formats a count with its noun: "1 warning", "2 warnings".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
