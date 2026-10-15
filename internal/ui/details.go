package ui

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/analytics"
	"brewery/internal/brew"
)

// detailsDelay is how long the cursor rests on a package before Discover
// asks brew about it, so holding j doesn't start a brew for every row.
const detailsDelay = 250 * time.Millisecond

// Messages from waiting on the cursor and asking brew about a package.
type (
	detailsDueMsg struct{ key string }
	detailsMsg    struct {
		key string
		d   brew.Details
		err error
	}
)

// wantDetails waits for the cursor to rest, then asks brew about the
// selected package if the preview doesn't know it yet.
func (a *App) wantDetails() tea.Cmd {
	d := &a.discover
	key := d.selectedKey()
	if a.catalog == nil || key == "" || key == d.waiting || key == d.asking {
		return nil
	}
	if _, ok := d.details[key]; ok {
		return nil
	}
	if _, ok := d.detailsErr[key]; ok {
		return nil
	}
	d.waiting = key
	return tea.Tick(a.detailsDelay, func(time.Time) tea.Msg { return detailsDueMsg{key} })
}

// detailsDue asks brew about the package once the cursor has rested on
// it, unless it has since moved on.
func (a *App) detailsDue(m detailsDueMsg) tea.Cmd {
	d := &a.discover
	if m.key != d.waiting {
		return nil // the cursor moved on, and a newer wait is running
	}
	d.waiting = ""
	r, ok := d.selected()
	if !ok || entryKey(r.Name, r.Kind) != m.key {
		return nil
	}
	d.asking = m.key
	cat, ctx := a.catalog, a.ctx
	return func() tea.Msg {
		det, err := cat.Details(ctx, r.Name, r.Kind)
		return detailsMsg{m.key, det, err}
	}
}

func (a *App) gotDetails(m detailsMsg) {
	d := &a.discover
	if d.asking == m.key {
		d.asking = ""
	}
	switch {
	case m.err == nil:
		d.details[m.key] = m.d
	case a.ctx.Err() == nil: // quitting cancels the question: that's no failure
		d.detailsErr[m.key] = m.err
	}
}

// missing is what installing the package with key would add: what it
// needs that isn't installed. ok is false until brew has said.
func (t *discoverTab) missing(key string) (names []string, ok bool) {
	det, ok := t.details[key]
	if !ok {
		return nil, false
	}
	for _, n := range det.Needs {
		if !t.have[entryKey(n, brew.KindFormula)] && !t.have[entryKey(n, brew.KindCask)] {
			names = append(names, n)
		}
	}
	return names, true
}

func (t *discoverTab) preview(r discRow) string {
	st := t.st
	key := entryKey(r.Name, r.Kind)
	det, known := t.details[key]
	var b strings.Builder
	b.WriteString(st.Title.Render(strings.TrimSpace(r.Name+" "+det.Latest)) + "\n")
	if r.Desc != "" {
		b.WriteString(st.Muted.Render(r.Desc) + "\n")
	}
	b.WriteString("\n")
	installed := "no"
	if t.have[key] {
		installed = st.OK.Render("yes")
	}
	b.WriteString(kv(st, "type", r.Kind.String(), "installed", installed,
		"tap", det.Tap, "licence", det.License, "homepage", det.Homepage))

	b.WriteString("\n" + st.Accent.Render("Would also install") + "\n")
	switch missing, ok := t.missing(key); {
	case ok && len(missing) == 0:
		b.WriteString(st.Muted.Render("nothing else") + "\n")
	case ok:
		b.WriteString(strings.Join(missing, ", ") + "\n")
	case t.detailsErr[key] != nil:
		first, _, _ := strings.Cut(t.detailsErr[key].Error(), "\n")
		b.WriteString(st.Err.Render("couldn't ask brew: "+first) + "\n")
	case !known:
		b.WriteString(st.Muted.Render("asking brew…") + "\n")
	}

	lists := []struct{ label, list string }{{"30 days", analytics.Tools30}, {"90 days", analytics.Tools90}, {"365 days", analytics.Tools365}}
	if r.Kind == brew.KindCask {
		lists = []struct{ label, list string }{{"30 days", analytics.Apps30}, {"90 days", analytics.Apps90}}
	}
	var pairs []string
	for _, l := range lists {
		if n, ok := t.counts[l.list][r.Name]; ok {
			pairs = append(pairs, l.label, withCommas(n))
		}
	}
	if len(pairs) > 0 {
		b.WriteString("\n" + st.Accent.Render("Installs") + "\n" + kv(st, pairs...))
	}
	return b.String()
}

// withCommas writes n with thousands separators, as Homebrew does:
// 2,994,048.
func withCommas(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
