package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/notes"
	"github.com/dillo/brewery/internal/theme"
)

// Release notes are GitHub markdown. This draws the little of it they use,
// in the theme's styles, without a markdown library: headings, lists,
// bold, code and links, and drops images, HTML and comments.
var (
	mdComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	mdImage   = regexp.MustCompile(`!\[[^\]]*\]\([^)]*\)`)
	mdLink    = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	mdAutolnk = regexp.MustCompile(`<(https?://[^>\s]+)>`)
	mdTag     = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	mdHeading = regexp.MustCompile(`^#{1,6}\s+(.*?)[\s#]*$`)
	mdItem    = regexp.MustCompile(`^(\s*)[-*+]\s+(.*)$`)
	mdBold    = regexp.MustCompile(`\*\*(.+?)\*\*|__(.+?)__`)
)

// renderNotes draws a release's notes as lines at most w wide. Runs of
// blank lines become one, and none starts or ends it.
func renderNotes(st theme.Styles, body string, w int) []string {
	body = mdComment.ReplaceAllString(strings.ReplaceAll(body, "\r\n", "\n"), "")
	var out []string
	fenced, blank := false, true // blank: the last line kept is blank, or there is none yet
	for l := range strings.SplitSeq(body, "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			fenced = !fenced
			continue
		}
		line := st.Muted.Render(l)
		if !fenced {
			line = mdLine(st, l)
		}
		if strings.TrimSpace(ansi.Strip(line)) == "" {
			if !blank {
				out = append(out, "")
			}
			blank = true
			continue
		}
		blank = false
		out = append(out, strings.Split(ansi.Wrap(line, w, ""), "\n")...)
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// mdLine draws one line outside a code block: a heading in bold, a list
// item after a bullet, indented one level at most, and anything else as
// it is.
func mdLine(st theme.Styles, l string) string {
	if m := mdHeading.FindStringSubmatch(l); m != nil {
		return mdInline(st, m[1], true)
	}
	if m := mdItem.FindStringSubmatch(l); m != nil {
		indent := strings.Repeat("  ", min(len(m[1])/2, 1))
		return indent + "• " + mdInline(st, m[2], false)
	}
	return mdInline(st, strings.TrimRight(l, " \t"), false)
}

// mdClean drops the images, link addresses and HTML tags from text that is
// not code, and keeps the address of an <https://…> autolink.
func mdClean(s string) string {
	s = mdAutolnk.ReplaceAllString(mdImage.ReplaceAllString(s, ""), "$1")
	return mdTag.ReplaceAllString(mdLink.ReplaceAllString(s, "$1"), "")
}

// mdInline styles a line's text: `code` in the accent colour, **bold** in
// bold, and the words that flag a breaking change in the warning colour.
// bold draws all of it in bold, for headings.
func mdInline(st theme.Styles, s string, bold bool) string {
	parts := strings.Split(s, "`")
	if len(parts)%2 == 0 { // an unclosed `: the text after it is plain
		last := len(parts) - 1
		parts[last-1] += "`" + parts[last]
		parts = parts[:last]
	}
	var b strings.Builder
	for i, p := range parts {
		if i%2 == 1 {
			b.WriteString(st.Accent.Render(p))
			continue
		}
		b.WriteString(mdBoldRuns(st, mdClean(p), bold))
	}
	return b.String()
}

// mdBoldRuns draws text with its **bold** and __bold__ runs in bold,
// without the markers.
func mdBoldRuns(st theme.Styles, s string, bold bool) string {
	var b strings.Builder
	at := 0
	for _, m := range mdBold.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(mdWords(st, s[at:m[0]], bold))
		start, end := m[2], m[3]
		if start < 0 { // __bold__
			start, end = m[4], m[5]
		}
		b.WriteString(mdWords(st, s[start:end], true))
		at = m[1]
	}
	b.WriteString(mdWords(st, s[at:], bold))
	return b.String()
}

// mdWords draws plain text, with the words that flag a breaking change in
// the warning colour.
func mdWords(st theme.Styles, s string, bold bool) string {
	if s == "" {
		return ""
	}
	text, warn := st.Text, st.Warn
	if bold {
		text, warn = text.Bold(true), warn.Bold(true)
	}
	var b strings.Builder
	at := 0
	for _, m := range notes.BreakingSpans(s) {
		if m[0] > at {
			b.WriteString(text.Render(s[at:m[0]]))
		}
		b.WriteString(warn.Render(s[m[0]:m[1]]))
		at = m[1]
	}
	if at < len(s) {
		b.WriteString(text.Render(s[at:]))
	}
	return b.String()
}
