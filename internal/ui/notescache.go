package ui

import (
	"maps"
	"slices"
	"strings"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/notes"
)

// The release notes brewery has fetched are kept in the snapshot, by
// package, for the versions they were fetched for. A save may be reading
// the map it was given, so the kept notes are never changed in place: each
// change makes a new map.

// notesKey is the key o's notes are kept under: its package's key, or
// its kind and name when the package isn't known.
func (a *App) notesKey(o brew.Outdated) string {
	if p := a.outdatedPackage(o); p.FullName != "" {
		return p.Key()
	}
	return o.Kind.String() + "/" + o.Name
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

// isReleasesPage reports whether n has nothing to show but GitHub's
// releases page, which is worth asking about again.
func isReleasesPage(n notes.Notes) bool {
	return len(n.Releases) == 0 && strings.HasSuffix(n.Link, "/releases")
}

// keepNotes keeps n under key, in a new map, and forgets the notes of
// packages no longer outdated.
func (a *App) keepNotes(key string, n notes.Notes) {
	kept := maps.Clone(a.snap.ReleaseNotes)
	if kept == nil {
		kept = map[string]notes.Notes{}
	}
	kept[key] = n
	a.snap.ReleaseNotes = kept
	a.pruneNotes()
}

// pruneNotes forgets the notes of packages no longer outdated, in a new
// map.
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
