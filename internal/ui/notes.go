package ui

import (
	"fmt"
	"maps"
	"net/url"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/notes"
)

// ReleaseFetcher asks GitHub for a repository's releases and tags.
// *github.Client satisfies it; tests use a fake.
type ReleaseFetcher = notes.Fetcher

// UseReleases sets how Outdated fetches release notes from GitHub.
func (a *App) UseReleases(f ReleaseFetcher) { a.releases = f }

// notesMsg carries one fetch's notes, or why it failed.
type notesMsg struct {
	key   string
	notes notes.Notes
	err   error
}

// noteLines is the most of one release's notes the details show.
const noteLines = 40

// outdatedSelected is the selected Outdated row and its package. ok is
// false when no row is shown.
func (a *App) outdatedSelected() (brew.Outdated, brew.Package, bool) {
	i := a.outdated.cursor()
	if i < 0 {
		return brew.Outdated{}, brew.Package{}, false
	}
	o := a.snap.Outdated[i]
	return o, a.packageNamed(o.Name, o.Kind), true
}

// notesKey is the key o's notes are kept under: its package's key, or
// its kind and name when the package isn't known.
func (a *App) notesKey(o brew.Outdated) string {
	if p := a.packageNamed(o.Name, o.Kind); p.FullName != "" {
		return p.Key()
	}
	return o.Kind.String() + "/" + o.Name
}

// notesRepo is the GitHub repository p's release notes come from: the
// first of its download addresses, then its homepage, that is on GitHub.
func notesRepo(p brew.Package) (owner, repo string, ok bool) {
	for _, addr := range append(slices.Clone(p.Sources), p.Homepage) {
		if owner, repo, ok = github.SourceRepo(addr); ok {
			return owner, repo, true
		}
	}
	return "", "", false
}

// keptNotes are the notes fetched for o's upgrade, when they're for the
// versions it goes between now.
func (a *App) keptNotes(o brew.Outdated) (notes.Notes, bool) {
	n, ok := a.snap.ReleaseNotes[a.notesKey(o)]
	return n, ok && n.From == o.Installed && n.To == o.Current
}

// hasBreaking reports whether any of n's releases mentions a breaking
// change.
func hasBreaking(n notes.Notes) bool {
	return slices.ContainsFunc(n.Releases, func(r github.Release) bool { return notes.Breaking(r.Body) })
}

// whatsNew fetches the selected upgrade's release notes from GitHub,
// unless they're kept already.
func (a *App) whatsNew() tea.Cmd {
	if a.marks() > 0 {
		return a.note("w shows one package at a time")
	}
	o, p, ok := a.outdatedSelected()
	if !ok {
		return nil
	}
	owner, repo, onGitHub := notesRepo(p)
	switch {
	case a.releases == nil:
		return a.note("release notes aren't available")
	case a.fetchingNotes != "":
		return a.note("already asking GitHub about " + a.fetchingName)
	case !onGitHub:
		return a.note(o.Name + "'s release notes aren't on GitHub")
	}
	if _, kept := a.keptNotes(o); kept {
		return nil // already showing
	}
	key := a.notesKey(o)
	a.fetchingNotes, a.fetchingName = key, o.Name
	delete(a.notesErr, key)
	ctx, f, now := a.ctx, a.releases, a.now
	name, from, to := o.Name, o.Installed, o.Current
	return func() tea.Msg {
		n, err := notes.Fetch(ctx, f, owner, repo, name, from, to, now())
		return notesMsg{key: key, notes: n, err: err}
	}
}

// gotNotes keeps a fetch's notes and saves them, or remembers why it
// failed until the next try. The details start again at the top when the
// notes are for the selected row. The kept notes are copied before they
// change, as a save may be reading the ones it was given.
func (a *App) gotNotes(m notesMsg) tea.Cmd {
	a.fetchingNotes, a.fetchingName = "", ""
	if o, _, ok := a.outdatedSelected(); ok && a.notesKey(o) == m.key {
		a.outdated.pv.off = 0
	}
	if m.err != nil {
		a.notesErr[m.key] = m.err.Error()
		return nil
	}
	kept := maps.Clone(a.snap.ReleaseNotes)
	if kept == nil {
		kept = map[string]notes.Notes{}
	}
	kept[m.key] = m.notes
	a.snap.ReleaseNotes = kept
	a.pruneNotes()
	return save(a.cachePath, a.snap)
}

// pruneNotes forgets the notes of packages no longer outdated, in a new
// map, as a save may be reading the one it was given.
func (a *App) pruneNotes() {
	if len(a.snap.ReleaseNotes) == 0 {
		return
	}
	outdated := map[string]bool{}
	for _, o := range a.snap.Outdated {
		outdated[a.notesKey(o)] = true
	}
	kept := map[string]notes.Notes{}
	for k, n := range a.snap.ReleaseNotes {
		if outdated[k] {
			kept[k] = n
		}
	}
	if len(kept) < len(a.snap.ReleaseNotes) {
		a.snap.ReleaseNotes = kept
	}
}

// notesPreview is Outdated's details: what's new in the selected upgrade,
// or how to see it.
func (a *App) notesPreview(w, h int) (title, body string) {
	o, p, ok := a.outdatedSelected()
	if !ok {
		return "Details", ""
	}
	iw, _ := paneInner(w, h)
	return o.Name + " " + o.Installed + " → " + o.Current, ansi.Wrap(a.notesBody(o, p, iw), iw, "")
}

// notesBody is the details' text for o, w wide.
func (a *App) notesBody(o brew.Outdated, p brew.Package, w int) string {
	st := a.st
	key := a.notesKey(o)
	owner, repo, onGitHub := notesRepo(p)
	n, kept := a.keptNotes(o)
	switch {
	case !onGitHub:
		return a.notOnGitHub(o, p)
	case a.fetchingNotes == key:
		return "asking GitHub…"
	case a.notesErr[key] != "":
		return st.Warn.Render("couldn't fetch: "+ansi.Truncate(a.notesErr[key], reasonW, "…")) + "\n" + st.Muted.Render("w tries again")
	case !kept:
		return st.Muted.Render(owner+"/"+repo) + "\n\n" + "w shows what's new, from GitHub"
	case len(n.Releases) == 0:
		return "no release notes between these versions\n" + st.Muted.Render(n.Link) + "\n" + st.Muted.Render("gx opens it")
	}
	return a.releasesBody(n, o, w)
}

// notOnGitHub says why a package has no release notes to show.
func (a *App) notOnGitHub(o brew.Outdated, p brew.Package) string {
	st := a.st
	from := "brew doesn't say where " + o.Name + " comes from"
	addr := p.Homepage
	if len(p.Sources) > 0 {
		addr = p.Sources[0]
	}
	if u, err := url.Parse(addr); err == nil && u.Host != "" {
		from = o.Name + " comes from " + u.Host
	}
	body := st.Muted.Render("release notes come from GitHub, and " + from)
	if webPage(p.Homepage) {
		body += "\n" + st.Muted.Render("gx opens the homepage")
	}
	return body
}

// releasesBody shows each release in n, newest first, after how many
// there are and whether any mentions a breaking change.
func (a *App) releasesBody(n notes.Notes, o brew.Outdated, w int) string {
	st := a.st
	lines := []string{plural(len(n.Releases), "release") + " since " + o.Installed + " · fetched " + ago(a.now(), n.At)}
	if hasBreaking(n) {
		lines = append(lines, st.Warn.Render("mentions breaking changes"))
	}
	for _, r := range n.Releases {
		lines = append(lines, "", st.Title.Render(r.Tag))
		if !r.Published.IsZero() {
			lines = append(lines, st.Muted.Render(r.Published.In(a.now().Location()).Format("2006-01-02")))
		}
		lines = append(lines, a.releaseLines(r, w)...)
	}
	return strings.Join(lines, "\n")
}

// releaseLines are one release's notes, w wide: at most noteLines of
// them, then how many more there are and where to read them.
func (a *App) releaseLines(r github.Release, w int) []string {
	st := a.st
	body := renderNotes(st, r.Body, w)
	switch {
	case len(body) == 0:
		return []string{st.Muted.Render("no notes")}
	case len(body) <= noteLines:
		return body
	}
	more := st.Muted.Render(fmt.Sprintf("… %d more lines", len(body)-noteLines))
	return append(body[:noteLines:noteLines], more, st.Muted.Render(r.URL))
}
