// Package state holds brewery's picture of the Homebrew installation and the
// facts derived from it that brew does not report directly.
package state

import (
	"maps"
	"slices"
	"sort"

	"brewery/internal/brew"
)

// Index answers dependency questions about a set of installed packages.
type Index struct {
	byFull map[string]brew.Package
	usedBy map[string][]string // full name -> full names of installed packages that depend on it
}

func NewIndex(pkgs []brew.Package) *Index {
	ix := &Index{byFull: make(map[string]brew.Package, len(pkgs)), usedBy: map[string][]string{}}
	for _, p := range pkgs {
		ix.byFull[p.FullName] = p
	}
	for _, p := range pkgs {
		for _, d := range p.Deps {
			ix.usedBy[d] = append(ix.usedBy[d], p.FullName)
		}
	}
	for k := range ix.usedBy {
		sort.Strings(ix.usedBy[k])
	}
	return ix
}

// Get returns the package with the given full name.
func (ix *Index) Get(fullName string) (brew.Package, bool) {
	p, ok := ix.byFull[fullName]
	return p, ok
}

// UsedBy lists installed packages that depend on fullName, directly or not.
// The slice is a copy the caller may keep or change.
func (ix *Index) UsedBy(fullName string) []string {
	return slices.Clone(ix.usedBy[fullName])
}

// Orphans lists the dependencies that removing the given packages would
// leave unneeded: packages installed as dependencies (not on request) that
// are used only by those packages or by other such orphans. Sorted; nil
// when there are none.
func (ix *Index) Orphans(fullNames ...string) []string {
	gone := map[string]bool{}
	for _, n := range fullNames {
		gone[n] = true
	}
	cand := map[string]bool{}
	for _, n := range fullNames {
		for _, d := range ix.byFull[n].Deps {
			if dp, ok := ix.byFull[d]; ok && !dp.OnRequest && !gone[d] {
				cand[d] = true
			}
		}
	}
	// Drop any candidate something else still needs, until nothing changes.
	for changed := true; changed; {
		changed = false
		for c := range cand {
			for _, u := range ix.usedBy[c] {
				if !gone[u] && !cand[u] {
					delete(cand, c)
					changed = true
					break
				}
			}
		}
	}
	if len(cand) == 0 {
		return nil
	}
	return slices.Sorted(maps.Keys(cand))
}

// IsLeaf reports whether p is a formula that no other installed package
// depends on (the same definition as `brew leaves`).
func (ix *Index) IsLeaf(p brew.Package) bool {
	return p.Kind == brew.KindFormula && len(ix.usedBy[p.FullName]) == 0
}

// Tree renders p's direct dependencies recursively, one line per node,
// using box-drawing prefixes. Repeated subtrees are shown once.
func (ix *Index) Tree(fullName string) []string {
	lines := []string{fullName}
	seen := map[string]bool{fullName: true}
	var walk func(name, prefix string)
	walk = func(name, prefix string) {
		p := ix.byFull[name]
		for i, d := range p.DirectDeps {
			last := i == len(p.DirectDeps)-1
			branch, next := "├─ ", "│  "
			if last {
				branch, next = "└─ ", "   "
			}
			if seen[d] {
				lines = append(lines, prefix+branch+d+" …")
				continue
			}
			seen[d] = true
			lines = append(lines, prefix+branch+d)
			walk(d, prefix+next)
		}
	}
	walk(fullName, "")
	return lines
}
