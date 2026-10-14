package ui

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/jobs"
	"brewery/internal/state"
	"brewery/internal/theme"
)

// tile is one health summary on the Home tab.
type tile struct {
	title  string
	value  string
	style  lipgloss.Style
	detail string
	tab    int // tab to open on Enter; -1 for none, openCleanup for the cleanup dialog
}

// openCleanup is the Cleanup tile's Enter: it asks to clean up rather than
// opening a tab.
const openCleanup = -2

// homeTab shows health tiles and doctor's warnings.
type homeTab struct {
	st        theme.Styles
	sel       int
	doctorErr error // last doctor failure; nil once a run succeeds
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
		fmt.Sprintf("%d formulae · %d casks", formulae, casks), tabInstalled}

	outdated := tile{"Outdated", "✓ 0", st.OK, "everything is up to date", tabOutdated}
	if n := len(snap.Outdated); n > 0 {
		names := make([]string, 0, n)
		for _, o := range snap.Outdated {
			names = append(names, o.Name)
		}
		outdated.value, outdated.style, outdated.detail = fmt.Sprint(n), st.Warn, strings.Join(names, ", ")
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
		fmt.Sprintf("%d registered", len(snap.Services)), tabServices}
	if len(failed) > 0 {
		services.value, services.style, services.detail = plural(len(failed), "error"), st.Err, strings.Join(failed, ", ")
	}

	h.markUnknown(&installed, qInstalled)
	h.markUnknown(&outdated, qOutdated)
	h.markUnknown(&services, qServices)

	doctor := tile{"Doctor", "…", st.Muted, "checking", -1}
	switch {
	case h.doctorErr != nil:
		doctor.value, doctor.style, doctor.detail = "✗ failed", st.Err, "see below"
	case snap.DoctorRan:
		doctor.value, doctor.style, doctor.detail = "✓ clean", st.OK, "no warnings"
		if n := len(snap.Doctor); n > 0 {
			doctor.value, doctor.style, doctor.detail = plural(n, "warning"), st.Warn, "see below"
		}
	}
	cleanup := tile{"Cleanup", "✓ tidy", st.OK, "all clean", openCleanup}
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

func (h *homeTab) update(msg tea.KeyPressMsg, snap state.Snapshot) (openTab int) {
	n := len(h.tiles(snap))
	switch msg.String() {
	case "l", "right":
		h.sel = min(h.sel+1, n-1)
	case "h", "left":
		h.sel = max(h.sel-1, 0)
	case "enter":
		return h.tiles(snap)[h.sel].tab
	}
	return -1
}

// view renders the tiles over doctor's findings and the recent jobs
// (newest first).
func (h *homeTab) view(snap state.Snapshot, recent []jobs.Record, now time.Time, w, height int) string {
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
		parts[i] = pane(st, t.title, body, tw, tileH, i == h.sel)
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, parts...)

	var b strings.Builder
	switch {
	case h.doctorErr != nil:
		b.WriteString(st.Err.Render("✗ brew doctor failed") + "\n\n")
		b.WriteString(st.Muted.Render(h.doctorErr.Error()) + "\n\n")
		b.WriteString(st.Muted.Render("press r to retry") + "\n")
		if snap.DoctorRan && len(snap.Doctor) > 0 {
			b.WriteString("\n" + st.Muted.Render("Warnings from the last successful check:") + "\n\n")
			h.writeWarnings(&b, snap.Doctor)
		}
	case !snap.DoctorRan:
		b.WriteString(st.Muted.Render("brew doctor is running…"))
	case len(snap.Doctor) == 0:
		b.WriteString(st.OK.Render("✓ Your system is ready to brew."))
	default:
		h.writeWarnings(&b, snap.Doctor)
	}
	lowH := max(height-tileH, 3)
	doctorW := w * 3 / 5
	doctor := pane(st, "Doctor", b.String(), doctorW, lowH, false)
	jobsPane := pane(st, "Recent jobs", h.recentJobs(recent, now, w-doctorW, lowH), w-doctorW, lowH, false)
	return lipgloss.JoinVertical(lipgloss.Left, row, lipgloss.JoinHorizontal(lipgloss.Top, doctor, jobsPane))
}

// recentJobs lists as many recent jobs as fit a w×h pane, each with how
// long ago it started, above a hint for the full list.
func (h *homeTab) recentJobs(recent []jobs.Record, now time.Time, w, height int) string {
	st := h.st
	iw, ih := paneInner(w, height)
	hint := st.Muted.Render(":jobs shows them all")
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

// writeWarnings renders each doctor warning as a title and indented body.
func (h *homeTab) writeWarnings(b *strings.Builder, warnings []brew.DoctorWarning) {
	st := h.st
	for _, d := range warnings {
		b.WriteString(st.Warn.Render("⚠ "+d.Title) + "\n")
		if d.Body != "" {
			// Style each line alone: as one block, every line would be
			// padded to the longest, and a narrow pane cuts the padding off.
			for _, line := range strings.Split(d.Body, "\n") {
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
