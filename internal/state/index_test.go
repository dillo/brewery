package state

import (
	"reflect"
	"slices"
	"testing"

	"brewery/internal/brew"
)

// git -> pcre2 -> json-c, git also depends on json-c indirectly; fd has no deps.
var testPkgs = []brew.Package{
	{Name: "git", FullName: "git", Kind: brew.KindFormula, OnRequest: true, Deps: []string{"pcre2", "json-c"}, DirectDeps: []string{"pcre2"}},
	{Name: "pcre2", FullName: "pcre2", Kind: brew.KindFormula, Deps: []string{"json-c"}, DirectDeps: []string{"json-c"}},
	{Name: "json-c", FullName: "json-c", Kind: brew.KindFormula},
	{Name: "fd", FullName: "fd", Kind: brew.KindFormula, OnRequest: true},
	{Name: "docker-desktop", FullName: "docker-desktop", Kind: brew.KindCask, OnRequest: true},
}

func TestUsedBy(t *testing.T) {
	ix := NewIndex(testPkgs)
	tests := map[string][]string{
		"json-c": {"git", "pcre2"},
		"pcre2":  {"git"},
		"fd":     nil,
	}
	for name, want := range tests {
		if got := ix.UsedBy(name); !reflect.DeepEqual(got, want) {
			t.Errorf("UsedBy(%s) = %v, want %v", name, got, want)
		}
	}
}

func TestIsLeaf(t *testing.T) {
	ix := NewIndex(testPkgs)
	want := map[string]bool{"git": true, "pcre2": false, "json-c": false, "fd": true, "docker-desktop": false}
	for _, p := range testPkgs {
		if got := ix.IsLeaf(p); got != want[p.Name] {
			t.Errorf("IsLeaf(%s) = %v, want %v", p.Name, got, want[p.Name])
		}
	}
}

func TestTree(t *testing.T) {
	ix := NewIndex(testPkgs)
	want := []string{"git", "└─ pcre2", "   └─ json-c"}
	if got := ix.Tree("git"); !reflect.DeepEqual(got, want) {
		t.Errorf("Tree(git) = %q, want %q", got, want)
	}
	if got := ix.Tree("fd"); !reflect.DeepEqual(got, []string{"fd"}) {
		t.Errorf("Tree(fd) = %q", got)
	}
}

func TestTreeMarksRepeats(t *testing.T) {
	pkgs := []brew.Package{
		{FullName: "a", DirectDeps: []string{"b", "c"}},
		{FullName: "b", DirectDeps: []string{"c"}},
		{FullName: "c"},
	}
	want := []string{"a", "├─ b", "│  └─ c", "└─ c …"}
	if got := NewIndex(pkgs).Tree("a"); !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestOrphans(t *testing.T) {
	ix := NewIndex(testPkgs)
	if got := ix.Orphans("git"); !reflect.DeepEqual(got, []string{"json-c", "pcre2"}) {
		t.Errorf("Orphans(git) = %v", got)
	}
	if got := ix.Orphans("fd"); len(got) != 0 {
		t.Errorf("fd has no deps, got %v", got)
	}
	if got := ix.Orphans("nope"); got != nil {
		t.Errorf("unknown package: %v", got)
	}
}

func TestOrphansKeepsWhatOthersNeed(t *testing.T) {
	pkgs := append(slices.Clone(testPkgs),
		brew.Package{Name: "jq", FullName: "jq", Kind: brew.KindFormula, OnRequest: true, Deps: []string{"json-c"}, DirectDeps: []string{"json-c"}})
	if got := NewIndex(pkgs).Orphans("git"); !reflect.DeepEqual(got, []string{"pcre2"}) {
		t.Errorf("json-c is still used by jq; got %v", got)
	}
}

func TestOrphansSkipsPackagesInstalledOnRequest(t *testing.T) {
	pkgs := slices.Clone(testPkgs)
	pkgs[1].OnRequest = true // pcre2 was installed by hand
	if got := NewIndex(pkgs).Orphans("git"); len(got) != 0 {
		t.Errorf("pcre2 was installed on request, and json-c is still used by it; got %v", got)
	}
}

func TestUsedByReturnsACopy(t *testing.T) {
	ix := NewIndex(testPkgs)
	ix.UsedBy("json-c")[0] = "mangled"
	if got := ix.UsedBy("json-c"); got[0] != "git" {
		t.Errorf("changing the result changed the index: %v", got)
	}
}

func TestOrphansTakesSeveralRounds(t *testing.T) {
	// a needs b and c; b needs c. x, installed by hand, still needs b, so
	// b stays, and then c stays too because b needs it: c only turns out
	// to be needed once b has been dropped.
	pkgs := []brew.Package{
		{FullName: "a", OnRequest: true, Deps: []string{"b", "c"}},
		{FullName: "b", Deps: []string{"c"}},
		{FullName: "c"},
		{FullName: "x", OnRequest: true, Deps: []string{"b"}},
	}
	for range 20 { // map order varies: c may be looked at before or after b
		if got := NewIndex(pkgs).Orphans("a"); len(got) != 0 {
			t.Fatalf("x still needs b, and b needs c; got %v", got)
		}
	}
}

func TestCaskDependencies(t *testing.T) {
	// The cask a needs libusb, which needs libffi; both came in for it.
	pkgs := []brew.Package{
		{Name: "a", FullName: "a", Kind: brew.KindCask, OnRequest: true, Deps: []string{"libusb", "libffi"}, DirectDeps: []string{"libusb"}},
		{Name: "libusb", FullName: "libusb", Kind: brew.KindFormula, Deps: []string{"libffi"}, DirectDeps: []string{"libffi"}},
		{Name: "libffi", FullName: "libffi", Kind: brew.KindFormula},
	}
	ix := NewIndex(pkgs)
	if got := ix.UsedBy("libusb"); !reflect.DeepEqual(got, []string{"a"}) {
		t.Errorf("UsedBy(libusb) = %v: the cask needs it", got)
	}
	if got := ix.Orphans("a"); !reflect.DeepEqual(got, []string{"libffi", "libusb"}) {
		t.Errorf("Orphans(a) = %v", got)
	}
	if ix.IsLeaf(pkgs[1]) {
		t.Error("libusb isn't top-level while a cask needs it")
	}
}
