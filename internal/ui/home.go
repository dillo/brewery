package ui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"brewery/internal/brew"
	"brewery/internal/state"
	"brewery/internal/theme"
)

// tile is one health summary on the Home tab.
type tile struct {
	title  string
	value  string
	style  lipgloss.Style
	detail string
	tab    int // tab to open on Enter; -1 for none
}

// homeTab shows health tiles and doctor's warnings.
type homeTab struct {
	st  theme.Styles
	sel int
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

	doctor := tile{"Doctor", "…", st.Muted, "checking", -1}
	if snap.DoctorRan {
		doctor.value, doctor.style, doctor.detail = "✓ clean", st.OK, "no warnings"
		if n := len(snap.Doctor); n > 0 {
			doctor.value, doctor.style, doctor.detail = plural(n, "warning"), st.Warn, "see below"
		}
	}
	return []tile{installed, outdated, services, doctor}
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

func (h *homeTab) view(snap state.Snapshot, w, height int) string {
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
	case !snap.DoctorRan:
		b.WriteString(st.Muted.Render("brew doctor is running…"))
	case len(snap.Doctor) == 0:
		b.WriteString(st.OK.Render("✓ Your system is ready to brew."))
	default:
		for _, d := range snap.Doctor {
			b.WriteString(st.Warn.Render("⚠ "+d.Title) + "\n")
			if d.Body != "" {
				b.WriteString(st.Muted.Render(indent(d.Body, "  ")) + "\n")
			}
			b.WriteString("\n")
		}
	}
	doctor := pane(st, "Doctor", b.String(), w, max(height-tileH, 3), false)
	return lipgloss.JoinVertical(lipgloss.Left, row, doctor)
}

// plural formats a count with its noun: "1 warning", "2 warnings".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func indent(s, prefix string) string {
	return prefix + strings.ReplaceAll(s, "\n", "\n"+prefix)
}

// Temporary: moved to app.go in Task 9.
const (
	tabHome = iota
	tabInstalled
	tabOutdated
	tabServices
	tabTaps
)
