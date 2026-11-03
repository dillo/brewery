package ui

import (
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/theme"
)

// ghNotes is how GitHub's generated release notes look.
const ghNotes = "<!-- Release notes generated using configuration in .github/release.yml -->\r\n" +
	"## What's Changed\r\n" +
	"### Features\r\n" +
	"* Add `gh pr revert` by @octocat in https://github.com/cli/cli/pull/1\r\n" +
	"  - nested [link](https://example.com)\r\n" +
	"![screenshot](https://example.com/a.png)\r\n" +
	"<img src=\"x.png\" width=\"10\">\r\n" +
	"\r\n" +
	"**BREAKING**: removed the old flag\r\n" +
	"\r\n" +
	"\r\n" +
	"```sh\r\n" +
	"gh pr revert 1\r\n" +
	"```\r\n"

func plainLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = ansi.Strip(l)
	}
	return out
}

func TestRenderNotes(t *testing.T) {
	st := theme.New(theme.Macchiato)
	got := renderNotes(st, ghNotes, 100)
	want := []string{
		"What's Changed",
		"Features",
		"• Add gh pr revert by @octocat in https://github.com/cli/cli/pull/1",
		"  • nested link",
		"",
		"BREAKING: removed the old flag",
		"",
		"gh pr revert 1",
	}
	if p := plainLines(got); !slices.Equal(p, want) {
		t.Errorf("rendered\n%q\nwant\n%q", p, want)
	}
	if !strings.Contains(got[2], st.Accent.Render("gh pr revert")) {
		t.Errorf("code is in the accent colour: %q", got[2])
	}
	if !strings.Contains(got[5], st.Warn.Bold(true).Render("BREAKING")) || !strings.Contains(got[5], st.Warn.Render("removed")) {
		t.Errorf("breaking words are in the warning colour, bold inside **: %q", got[5])
	}
	if !strings.Contains(got[7], st.Muted.Render("gh pr revert 1")) {
		t.Errorf("code blocks are muted: %q", got[7])
	}
}

func TestRenderNotesDetails(t *testing.T) {
	st := theme.New(theme.Macchiato)
	for _, tt := range []struct {
		body string
		want []string
	}{
		{"", nil},
		{"\n\n<!-- nothing -->\n\n", nil},
		{"# Title #", []string{"Title"}},
		{"+ plus\n\t- tab", []string{"• plus", "• tab"}},
		{"      - deep", []string{"  • deep"}}, // two levels at most
		{"an `unclosed tick", []string{"an `unclosed tick"}},
		{"__bold__ and *em* and _em_", []string{"bold and *em* and _em_"}},
		{"---", []string{"---"}},
	} {
		if got := plainLines(renderNotes(st, tt.body, 80)); !slices.Equal(got, tt.want) {
			t.Errorf("%q: %q, want %q", tt.body, got, tt.want)
		}
	}
	for _, l := range renderNotes(st, strings.Repeat("word ", 30)+strings.Repeat("x", 50), 20) {
		if w := ansi.StringWidth(l); w > 20 {
			t.Errorf("a line %d wide, past 20: %q", w, ansi.Strip(l))
		}
	}
}
