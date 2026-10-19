package brew

import (
	"os"
	"reflect"
	"slices"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func byName(pkgs []Package, name string) Package {
	for _, p := range pkgs {
		if p.Name == name {
			return p
		}
	}
	return Package{}
}

func TestParseInfo(t *testing.T) {
	pkgs, err := ParseInfo(fixture(t, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 7 {
		t.Fatalf("got %d packages, want 7", len(pkgs))
	}

	git := byName(pkgs, "git")
	if git.Kind != KindFormula || git.Version != "2.51.0" || !git.OnRequest || !git.Pinned {
		t.Errorf("git parsed wrong: %+v", git)
	}
	if !reflect.DeepEqual(git.Deps, []string{"pcre2", "json-c"}) {
		t.Errorf("git deps = %v", git.Deps)
	}
	if !reflect.DeepEqual(git.DirectDeps, []string{"pcre2"}) {
		t.Errorf("git direct deps = %v", git.DirectDeps)
	}
	if git.Caveats != "Bash completion has been installed." {
		t.Errorf("git caveats = %q", git.Caveats)
	}
	if !git.InstalledAt.Equal(time.Unix(1788000000, 0)) {
		t.Errorf("git installed at = %v", git.InstalledAt)
	}

	if pcre := byName(pkgs, "pcre2"); pcre.OnRequest {
		t.Error("pcre2 should be installed as a dependency")
	}
	if lls := byName(pkgs, "lua-language-server"); lls.Version != "3.19.1" || lls.Latest != "3.19.2" || !lls.Outdated {
		t.Errorf("multi-keg formula should use newest keg: %+v", lls)
	}

	docker := byName(pkgs, "docker-desktop")
	if docker.Kind != KindCask || docker.Version != "4.39.0,184744" || docker.Latest != "4.95.0,242831" || !docker.Outdated || !docker.OnRequest {
		t.Errorf("docker-desktop parsed wrong: %+v", docker)
	}
	if fb := byName(pkgs, "fontbase"); fb.Desc != "FontBase" || !fb.InstalledAt.IsZero() {
		t.Errorf("cask with null desc should fall back to its name: %+v", fb)
	}
}

func TestParseInfoCaskDependencies(t *testing.T) {
	// b needs the cask a, which needs the formula libusb, which needs
	// libffi. brew lists only what each cask names itself.
	in := `{"formulae": [
	  {"name": "libusb", "full_name": "libusb", "installed": [{"version": "1.0", "runtime_dependencies": [{"full_name": "libffi", "declared_directly": true}]}]},
	  {"name": "libffi", "full_name": "libffi", "installed": [{"version": "3.5"}]}
	], "casks": [
	  {"token": "a", "full_token": "a", "depends_on": {"formula": ["libusb"], "macos": {">=": ["14"]}},
	   "artifacts": [{"app": ["A.app"], "target": "/Applications/A.app"}, {"uninstall": [{"quit": "a"}]}, "odd", {"target": 7}, {"font": ["A.ttf"], "target": "/Users/x/Library/Fonts/A.ttf"}]},
	  {"token": "b", "full_token": "b", "depends_on": {"cask": ["a"]}},
	  {"token": "c", "full_token": "c", "depends_on": {}}
	]}`
	pkgs, err := ParseInfo([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name         string
		deps, direct []string
	}{
		{"a", []string{"libusb", "libffi"}, []string{"libusb"}},
		{"b", []string{"a", "libusb", "libffi"}, []string{"a"}},
		{"c", nil, nil},
	}
	if a := byName(pkgs, "a"); !reflect.DeepEqual(a.Targets, []string{"/Applications/A.app", "/Users/x/Library/Fonts/A.ttf"}) {
		t.Errorf("a's targets = %v: only artifacts with a target count", a.Targets)
	}
	for _, tt := range tests {
		p := byName(pkgs, tt.name)
		if !reflect.DeepEqual(p.Deps, tt.deps) || !reflect.DeepEqual(p.DirectDeps, tt.direct) {
			t.Errorf("%s: deps %v, direct %v; want %v, %v", tt.name, p.Deps, p.DirectDeps, tt.deps, tt.direct)
		}
	}
}

func TestParseInfoBadJSON(t *testing.T) {
	if _, err := ParseInfo([]byte("Error: not json")); err == nil {
		t.Fatal("want error for non-JSON output")
	}
}

func TestParseOutdatedSkipsLeadingNoise(t *testing.T) {
	got, err := ParseOutdated(fixture(t, "outdated.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Outdated{
		{Name: "lua-language-server", Kind: KindFormula, Installed: "3.19.1", Current: "3.19.2"},
		{Name: "docker-desktop", Kind: KindCask, Installed: "4.39.0,184744", Current: "4.95.0,242831"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseServices(t *testing.T) {
	got, err := ParseServices(fixture(t, "services.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Name != "postgresql@16" || got[0].Status != "error" || *got[0].ExitCode != 1 {
		t.Errorf("got %+v", got)
	}
	if got[1].ExitCode != nil {
		t.Errorf("null exit code should stay nil, got %v", *got[1].ExitCode)
	}
	redis := got[1]
	if redis.PID != 80476 || redis.Log != "/opt/homebrew/var/log/redis.log" || redis.ErrorLog != "" ||
		redis.Command != "/opt/homebrew/opt/redis/bin/redis-server /opt/homebrew/etc/redis.conf" {
		t.Errorf("redis details parsed wrong: %+v", redis)
	}
	if pg := got[0]; pg.PID != 0 || pg.ErrorLog != "/opt/homebrew/var/log/postgresql@16.log" {
		t.Errorf("a stopped service has no pid: %+v", pg)
	}
}

func TestParseCleanup(t *testing.T) {
	if got, want := ParseCleanup(string(fixture(t, "cleanup.txt"))), (CleanupEstimate{Items: 4, Size: "86.7MB"}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := ParseCleanup(""); got != (CleanupEstimate{}) {
		t.Errorf("nothing to clean up: got %+v", got)
	}
}

func TestParseTaps(t *testing.T) {
	got, err := ParseTaps(fixture(t, "taps.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []Tap{
		{Name: "anomalyco/tap", Remote: "https://github.com/anomalyco/homebrew-tap.git", Path: "/opt/homebrew/Library/Taps/anomalyco/homebrew-tap", Formulae: 2, Casks: 1},
		{Name: "homebrew/core", Path: "/opt/homebrew/Library/Taps/homebrew/homebrew-core", Official: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestParseDoctor(t *testing.T) {
	got := ParseDoctor(string(fixture(t, "doctor.txt")))
	want := []DoctorWarning{
		{Title: "The following taps are not trusted:", Body: "oven-sh/bun\n\nUntap them with:\n  brew untap oven-sh/bun"},
		{Title: "Some installed casks are deprecated or disabled.", Body: "fontbase"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v\nwant %#v", got, want)
	}
}

func TestParseDoctorClean(t *testing.T) {
	if got := ParseDoctor("Your system is ready to brew.\n"); len(got) != 0 {
		t.Errorf("want no warnings, got %+v", got)
	}
}

func TestParseAutoremove(t *testing.T) {
	for _, tt := range []struct {
		name, out string
		want      []string
	}{
		{"several", "==> Would autoremove 2 unneeded formulae:\nlibyaml\nm4\n", []string{"libyaml", "m4"}},
		{"one", "==> Would autoremove 1 unneeded formula:\nm4\n", []string{"m4"}},
		{"none", "", nil},
		{"stops at the next heading", "==> Would autoremove 1 unneeded formula:\nm4\n==> Something else\nnot-a-formula\n", []string{"m4"}},
		{"stops at a blank line", "==> Would autoremove 1 unneeded formula:\nm4\n\nWarning: hi\n", []string{"m4"}},
	} {
		if got := ParseAutoremove(tt.out); !slices.Equal(got, tt.want) {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestParseServicesLabelAndLoaded(t *testing.T) {
	svcs, err := ParseServices(fixture(t, "services.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s := svcs[0]; s.Label != "homebrew.mxcl.postgresql@16" || !s.Loaded {
		t.Errorf("got label %q, loaded %v", s.Label, s.Loaded)
	}
}

func TestParseInfoLinks(t *testing.T) {
	pkgs, err := ParseInfo(fixture(t, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name            string
		linked, kegOnly bool
	}{
		{"fd", true, false},
		{"json-c", false, true},
		{"docker-desktop", false, false}, // casks are never linked
	} {
		p := byName(pkgs, tt.name)
		if p.Linked != tt.linked || p.KegOnly != tt.kegOnly {
			t.Errorf("%s: linked %v, keg-only %v; want %v, %v", tt.name, p.Linked, p.KegOnly, tt.linked, tt.kegOnly)
		}
	}
}

func TestParseInfoRetirement(t *testing.T) {
	pkgs, err := ParseInfo(fixture(t, "info.json"))
	if err != nil {
		t.Fatal(err)
	}
	jsonc := byName(pkgs, "json-c")
	if !jsonc.Deprecated || jsonc.Disabled || jsonc.Reason != "unmaintained" ||
		jsonc.DeprecationDate != "2026-03-01" || jsonc.DisableDate != "" || jsonc.Replacement != "--formula json-c-ng" {
		t.Errorf("json-c: %+v", jsonc)
	}
	fontbase := byName(pkgs, "fontbase")
	if fontbase.Deprecated || !fontbase.Disabled || fontbase.Reason != "discontinued" ||
		fontbase.DisableDate != "2026-09-01" || fontbase.Replacement != "" {
		t.Errorf("fontbase: %+v", fontbase)
	}
	fd := byName(pkgs, "fd")
	if fd.Deprecated || fd.Disabled || fd.Reason != "" || fd.Replacement != "" {
		t.Errorf("fd is neither: %+v", fd)
	}
}

func TestReplacementIsWrittenAsBrewWritesIt(t *testing.T) {
	for _, tt := range []struct{ formula, cask, want string }{
		{"foo", "foo", "foo"},
		{"foo", "", "--formula foo"},
		{"", "bar", "--cask bar"},
		{"foo", "bar", "--formula foo"},
		{"", "", ""},
	} {
		if got := replacement(tt.formula, tt.cask); got != tt.want {
			t.Errorf("replacement(%q, %q) = %q, want %q", tt.formula, tt.cask, got, tt.want)
		}
	}
}

func TestReasonComesFromTheStateBrewReports(t *testing.T) {
	r := retireJSON{
		Deprecated: true, DeprecationReason: new("unmaintained"),
		Disabled: true, DisableReason: new("does_not_build"), DisableFormula: new("other"),
	}
	var p Package
	r.apply(&p)
	if p.Reason != "unmaintained" || p.Replacement != "" {
		t.Errorf("deprecated wins, as in brew: %+v", p)
	}
	r.Deprecated = false
	p = Package{}
	r.apply(&p)
	if p.Reason != "does_not_build" || p.Replacement != "--formula other" {
		t.Errorf("disabled: %+v", p)
	}
}
