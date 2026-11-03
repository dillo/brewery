package notes

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dillo/brewery/internal/github"
)

func TestClean(t *testing.T) {
	for in, want := range map[string]string{
		"2.101.0":       "2.101.0",
		"2.101.0_1":     "2.101.0",
		"4.39.0,184744": "4.39.0",
		"1.4.2,abc_1":   "1.4.2",
		"1.0_rc":        "1.0_rc", // only a numeric revision goes
		"20260817.0_12": "20260817.0",
		"":              "",
	} {
		if got := Clean(in); got != want {
			t.Errorf("Clean(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLess(t *testing.T) {
	for _, tt := range []struct {
		a, b string
		less bool
	}{
		{"2.9", "2.10", true},
		{"2.10", "2.9", false},
		{"2.1", "2.1.1", true}, // a missing part is lower
		{"2.1.1", "2.1", false},
		{"2.1", "2.1", false},
		{"1.0-beta", "1.0-rc", true}, // text compares as text
		{"3.19.1", "3.19.2", true},
		{"20260817.0", "20261001.0", true},
	} {
		if got := Less(tt.a, tt.b); got != tt.less {
			t.Errorf("Less(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.less)
		}
	}
}

func TestTagVersion(t *testing.T) {
	for _, tt := range []struct {
		tag, want string
		ok        bool
	}{
		{"v2.104.0", "2.104.0", true},
		{"V2.104.0", "2.104.0", true},
		{"2.104.0", "2.104.0", true},
		{"release-1.2", "1.2", true},
		{"cli-2.1", "2.1", true}, // the repository's name
		{"gh-v2.1", "2.1", true}, // the package's name, then v
		{"nightly", "", false},
		{"v", "", false},
		{"", "", false},
	} {
		got, ok := TagVersion(tt.tag, "cli", "gh")
		if got != tt.want || ok != tt.ok {
			t.Errorf("TagVersion(%q) = %q, %v; want %q, %v", tt.tag, got, ok, tt.want, tt.ok)
		}
	}
}

func tags(rels []github.Release) []string {
	out := make([]string, len(rels))
	for i, r := range rels {
		out[i] = r.Tag
	}
	return out
}

func TestPick(t *testing.T) {
	rels := []github.Release{
		{Tag: "v2.105.0"}, // after to
		{Tag: "v2.104.0"}, // to: in
		{Tag: "v2.104.0-rc1", Prerelease: true},
		{Tag: "nightly", Name: "2.103.5"}, // the name stands in for the tag
		{Tag: "v2.103.0"},                 // in
		{Tag: "weekly", Name: "Weekly"},   // neither parses: out
		{Tag: "v2.101.0"},                 // from: out
		{Tag: "v2.100.0"},
	}
	got := tags(Pick(rels, "cli", "gh", "2.101.0_1", "2.104.0"))
	if want := []string{"v2.104.0", "nightly", "v2.103.0"}; !slices.Equal(got, want) {
		t.Errorf("picked %v, want %v", got, want)
	}
	pre := []github.Release{{Tag: "v3.0.0-rc2", Prerelease: true}, {Tag: "v3.0.0-rc1", Prerelease: true}, {Tag: "v2.9.0"}}
	if got := tags(Pick(pre, "cli", "gh", "2.9.0", "3.0.0-rc2")); !slices.Equal(got, []string{"v3.0.0-rc2", "v3.0.0-rc1"}) {
		t.Errorf("to a pre-release, pre-releases count: %v", got)
	}
	if got := tags(Pick(pre, "cli", "gh", "2.9.0", "3.0.0")); len(got) != 0 {
		t.Errorf("to a release, pre-releases don't: %v", got)
	}
	shuffled := []github.Release{{Tag: "v1.1"}, {Tag: "v1.3"}, {Tag: "v1.2"}}
	if got := tags(Pick(shuffled, "r", "n", "1.0", "1.3")); !slices.Equal(got, []string{"v1.3", "v1.2", "v1.1"}) {
		t.Errorf("newest first: %v", got)
	}
}

func TestLink(t *testing.T) {
	base := "https://github.com/cli/cli"
	for _, tt := range []struct {
		tags []string
		want string
	}{
		{[]string{"v2.104.0", "v2.101.0"}, base + "/compare/v2.101.0...v2.104.0"},
		{[]string{"v2.104.0"}, base + "/releases"},
		{nil, base + "/releases"},
	} {
		if got := Link("cli", "cli", "gh", tt.tags, "2.101.0_1", "2.104.0"); got != tt.want {
			t.Errorf("tags %v: %q, want %q", tt.tags, got, tt.want)
		}
	}
}

func TestBreaking(t *testing.T) {
	for body, want := range map[string]bool{
		"BREAKING: the flag changed":  true,
		"`--old` was removed":         true,
		"Deprecated the old API":      true,
		"an incompatible change":      true,
		"see the migration guide":     true,
		"an irremovable bug is fixed": false,
		"fixed a crash":               false,
	} {
		if got := Breaking(body); got != want {
			t.Errorf("Breaking(%q) = %v, want %v", body, got, want)
		}
	}
	if got := BreakingSpans("x removed y"); len(got) != 1 || got[0][0] != 2 || got[0][1] != 9 {
		t.Errorf("spans %v, want [[2 9]]", got)
	}
}

// fakeFetcher answers from fields, and records what it was asked.
type fakeFetcher struct {
	rels           []github.Release
	tags           []string
	relErr, tagErr error
	asked          []string
}

func (f *fakeFetcher) Releases(_ context.Context, owner, repo string) ([]github.Release, error) {
	f.asked = append(f.asked, "releases "+owner+"/"+repo)
	return f.rels, f.relErr
}

func (f *fakeFetcher) Tags(_ context.Context, owner, repo string) ([]string, error) {
	f.asked = append(f.asked, "tags "+owner+"/"+repo)
	return f.tags, f.tagErr
}

func TestFetch(t *testing.T) {
	ctx, now := context.Background(), time.Date(2026, 11, 3, 9, 0, 0, 0, time.UTC)
	long := strings.Repeat("x", maxBody+10)
	f := &fakeFetcher{rels: []github.Release{{Tag: "v2", Body: long}, {Tag: "v1"}}}
	n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now)
	if err != nil || n.Repo != "o/r" || n.From != "1" || n.To != "2" || !n.At.Equal(now) || len(n.Releases) != 1 || n.Link != "" {
		t.Fatalf("releases found: %+v, %v", n, err)
	}
	if len(n.Releases[0].Body) != maxBody {
		t.Errorf("a long body is kept to %d bytes, got %d", maxBody, len(n.Releases[0].Body))
	}
	if !slices.Equal(f.asked, []string{"releases o/r"}) {
		t.Errorf("with releases found, only they are asked for: %v", f.asked)
	}

	f = &fakeFetcher{tags: []string{"v2", "v1"}}
	if n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now); err != nil || len(n.Releases) != 0 || n.Link != "https://github.com/o/r/compare/v1...v2" {
		t.Errorf("no releases: %+v, %v", n, err)
	}
	f = &fakeFetcher{tagErr: github.ErrRateLimited}
	if n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now); err != nil || n.Link != "https://github.com/o/r/releases" {
		t.Errorf("tags failing still gives the releases page: %+v, %v", n, err)
	}
	f = &fakeFetcher{relErr: github.ErrNotFound}
	if _, err := Fetch(ctx, f, "o", "r", "r", "1", "2", now); !errors.Is(err, github.ErrNotFound) {
		t.Errorf("releases failing is the error: %v", err)
	}
}

func TestFetchCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := &fakeFetcher{tagErr: ctx.Err()}
	n, err := Fetch(ctx, f, "o", "r", "r", "1", "2", time.Now())
	if !errors.Is(err, context.Canceled) || n.Link != "" || n.Repo != "" {
		t.Errorf("a cancelled fetch keeps nothing: %+v, %v", n, err)
	}
}
