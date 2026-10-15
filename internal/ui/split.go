package ui

import "charm.land/lipgloss/v2"

// Widths at which a list and its preview change layout.
const (
	sideBySideWidth = 100 // and wider: the preview sits beside the list
	previewMinWidth = 60  // narrower: one at a time, the preview only when asked
	minPreviewH     = 5   // a stacked preview shorter than this isn't shown
)

// split is how a list and its preview share a w×h area. A zero width
// means that pane isn't shown.
type split struct {
	listW, listH int
	pvW, pvH     int
	stacked      bool // the preview is under the list
}

// splitFor lays out a list and its preview in w×h: side by side from 100
// columns, the preview under the list from 60, and below that (or when a
// stacked preview would be too short) the list alone, or the preview alone
// when showPreview is set.
func splitFor(w, h int, showPreview bool) split {
	if w >= sideBySideWidth {
		lw := w * 55 / 100
		return split{listW: lw, listH: h, pvW: w - lw, pvH: h}
	}
	if lh := h - h*45/100; w >= previewMinWidth && h-lh >= minPreviewH {
		return split{listW: w, listH: lh, pvW: w, pvH: h - lh, stacked: true}
	}
	if showPreview {
		return split{pvW: w, pvH: h}
	}
	return split{listW: w, listH: h}
}

// join puts the list and preview panes together as s lays them out.
func (s split) join(list, preview string) string {
	switch {
	case s.listW == 0:
		return preview
	case s.pvW == 0:
		return list
	case s.stacked:
		return lipgloss.JoinVertical(lipgloss.Left, list, preview)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, list, preview)
}
