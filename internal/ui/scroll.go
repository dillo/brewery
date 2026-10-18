package ui

import "strings"

// scroller is how far a preview has scrolled.
type scroller struct {
	off   int  // lines scrolled past
	last  int  // the furthest off can go, as of the last window
	sized bool // a window has set last
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
	if s.sized {
		s.off = min(s.off, s.last) // so k right after G moves up at once
	}
	return true
}

// window is body from the scroll offset, for a pane h lines tall. It
// clamps the offset so the last line can't scroll out of view.
func (s *scroller) window(body string, h int) string {
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	s.last, s.sized = max(len(lines)-h, 0), true
	s.off = min(s.off, s.last)
	return strings.Join(lines[s.off:], "\n")
}
