package ui

import "strings"

// scroller lets a preview scroll once it has the focus.
type scroller struct {
	focus bool // keys scroll the preview rather than move through the list
	off   int  // lines scrolled past
}

// scroll moves by a scrolling key, page lines for ctrl+d and ctrl+u. ok
// is false for any other key.
func (s *scroller) scroll(k string, page int) (ok bool) {
	switch k {
	case "j", "down":
		s.off++
	case "k", "up":
		s.off--
	case "ctrl+d":
		s.off += page
	case "ctrl+u":
		s.off -= page
	case "home":
		s.off = 0
	case "end", "G":
		s.off = 1 << 30 // window clamps it to the last screenful
	default:
		return false
	}
	s.off = max(s.off, 0)
	return true
}

// window is body from the scroll offset, for a pane h lines tall. It
// clamps the offset so the last line can't scroll out of view.
func (s *scroller) window(body string, h int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	s.off = min(s.off, max(len(lines)-h, 0))
	return strings.Join(lines[s.off:], "\n")
}
