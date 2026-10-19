package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/brew"
)

func TestReasonText(t *testing.T) {
	for _, tt := range []struct {
		p    brew.Package
		want string
	}{
		{brew.Package{Kind: brew.KindFormula, Reason: "unmaintained"}, "is not maintained upstream"},
		{brew.Package{Kind: brew.KindFormula, Reason: "repo_archived"}, "has an archived upstream repository"},
		{brew.Package{Kind: brew.KindCask, Reason: "discontinued"}, "is discontinued upstream"},
		{brew.Package{Kind: brew.KindCask, Reason: "repo_archived"}, "repo_archived"}, // a formula keyword on a cask stays as it is
		{brew.Package{Kind: brew.KindFormula, Reason: "it moved to a new name"}, "it moved to a new name"},
		{brew.Package{Kind: brew.KindFormula}, ""},
	} {
		if got := reasonText(tt.p); got != tt.want {
			t.Errorf("%v %q: %q, want %q", tt.p.Kind, tt.p.Reason, got, tt.want)
		}
	}
}

func TestDisableLine(t *testing.T) {
	now := time.Date(2026, 10, 13, 9, 30, 0, 0, time.UTC)
	for _, tt := range []struct {
		p    brew.Package
		want string
	}{
		{brew.Package{DisableDate: "2026-09-01"}, "disabled on 2026-09-01"},
		{brew.Package{DisableDate: "2026-10-13"}, "will be disabled on 2026-10-13"}, // today counts as to come, as in brew
		{brew.Package{DisableDate: "2026-12-01"}, "will be disabled on 2026-12-01"},
		{brew.Package{DeprecationDate: "2026-03-01"}, "will be disabled on 2027-03-01"},
		{brew.Package{DeprecationDate: "2025-01-15"}, "disabled on 2026-01-15"},
		{brew.Package{DeprecationDate: "2026-03-01", DisableDate: "2026-11-01"}, "will be disabled on 2026-11-01"},
		{brew.Package{}, ""},
		{brew.Package{DisableDate: "soon"}, ""},
	} {
		if got := disableLine(tt.p, now); got != tt.want {
			t.Errorf("%+v: %q, want %q", tt.p, got, tt.want)
		}
	}
}

func TestInstalledShowsRetirement(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "All")
	out := stripANSI(a.render())
	for _, want := range []string{`json-c\s.*⚠`, `fontbase\s.*✗`} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("STATUS lacks %s:\n%s", want, out)
		}
	}

	selectInstalled(t, a, "json-c")
	out = stripANSI(a.render())
	for _, want := range []string{
		`deprecated\s+is not maintained upstream`,
		`will be disabled on 2027-03-01`,
		`replacement\s+brew install --formula json-c-ng`,
	} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("json-c's details lack %s:\n%s", want, out)
		}
	}

	selectInstalled(t, a, "fontbase")
	out = stripANSI(a.render())
	for _, want := range []string{`disabled\s+is discontinued upstream`, `disabled on 2026-09-01`} {
		if !regexp.MustCompile(want).MatchString(out) {
			t.Errorf("fontbase's details lack %s:\n%s", want, out)
		}
	}

	for _, p := range a.snap.Packages {
		if p.Name == "fd" {
			if pv := stripANSI(a.installed.preview(p)); strings.Contains(pv, "deprecated") || strings.Contains(pv, "disabled") {
				t.Errorf("fd is neither:\n%s", pv)
			}
		}
	}
}

func TestDeprecatedView(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	show(t, a, "Deprecated")
	var names []string
	for _, p := range a.installed.rows {
		names = append(names, p.Name)
	}
	if strings.Join(names, ",") != "fontbase,json-c" && strings.Join(names, ",") != "json-c,fontbase" {
		t.Errorf("Deprecated lists %v, want json-c and fontbase", names)
	}
	if !regexp.MustCompile(`Deprecated\s+2`).MatchString(stripANSI(a.render())) {
		t.Errorf("the sidebar should count 2:\n%s", stripANSI(a.render()))
	}
	for i := range a.snap.Packages {
		a.snap.Packages[i].Deprecated, a.snap.Packages[i].Disabled = false, false
	}
	a.sync()
	if out := stripANSI(a.render()); !strings.Contains(out, "nothing installed is deprecated or disabled") {
		t.Errorf("empty Deprecated view:\n%s", out)
	}
}
