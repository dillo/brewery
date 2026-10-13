package brew

import (
	"os"
	"reflect"
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
