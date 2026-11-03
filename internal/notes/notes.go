// Package notes picks a package's release notes on GitHub: the releases
// between the version installed and the one brew would install.
package notes

import (
	"cmp"
	"context"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/dillo/brewery/internal/github"
)

// Notes are what was fetched about one package's upgrade.
type Notes struct {
	Repo     string           `json:"repo"`     // owner/repo
	From     string           `json:"from"`     // the installed version, as brew writes it
	To       string           `json:"to"`       // the version brew would install
	Releases []github.Release `json:"releases"` // in range, newest first
	Link     string           `json:"link"`     // the compare or releases page, when Releases is empty
	At       time.Time        `json:"at"`       // when they were fetched
}

// Fetcher asks GitHub for a repository's releases and tags.
type Fetcher interface {
	Releases(ctx context.Context, owner, repo string) ([]github.Release, error)
	Tags(ctx context.Context, owner, repo string) ([]string, error)
}

var _ Fetcher = (*github.Client)(nil)

// maxBody is the most of one release's notes that is kept, in bytes.
const maxBody = 16 << 10

// Fetch asks GitHub what's new in owner/repo after from, up to to. When no
// release falls in between, it asks for the tags too, for a page to read
// instead: the compare page, or the releases page when the tags don't say
// (or can't be had). name is the package's, for reading tags. now stamps
// the result.
func Fetch(ctx context.Context, f Fetcher, owner, repo, name, from, to string, now time.Time) (Notes, error) {
	rels, err := f.Releases(ctx, owner, repo)
	if err != nil {
		return Notes{}, err
	}
	n := Notes{Repo: owner + "/" + repo, From: from, To: to, At: now, Releases: Pick(rels, repo, name, from, to)}
	for i := range n.Releases {
		n.Releases[i].Body = trimBody(n.Releases[i].Body)
	}
	if len(n.Releases) > 0 {
		return n, nil
	}
	tags, err := f.Tags(ctx, owner, repo)
	if ctx.Err() != nil {
		return Notes{}, ctx.Err() // cancelled: nothing to keep
	}
	if err != nil {
		tags = nil // the releases page needs no tags
	}
	n.Link = Link(owner, repo, name, tags, from, to)
	return n, nil
}

// trimBody keeps at most maxBody bytes of a release's notes, without
// cutting a character in two.
func trimBody(s string) string {
	if len(s) <= maxBody {
		return s
	}
	return strings.ToValidUTF8(s[:maxBody], "")
}

// Clean is brew's version as upstream writes it: without a formula's
// revision (2.101.0_1) or what follows a cask's first comma (4.39.0,184744).
func Clean(v string) string {
	v, _, _ = strings.Cut(v, ",")
	if i := strings.LastIndexByte(v, '_'); i > 0 && isNumber(v[i+1:]) {
		v = v[:i]
	}
	return v
}

// isNumber reports whether s is one or more digits.
func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil && !strings.ContainsAny(s, "+-")
}

// compare orders versions a and b as -1, 0 or 1. Parts split on . and -;
// numeric parts compare as numbers, others as text, and a version that
// runs out of parts first is the lower.
func compare(a, b string) int {
	pa, pb := parts(a), parts(b)
	for i := range min(len(pa), len(pb)) {
		if c := comparePart(pa[i], pb[i]); c != 0 {
			return c
		}
	}
	return cmp.Compare(len(pa), len(pb))
}

// parts splits a version on . and -.
func parts(v string) []string {
	return strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' })
}

// comparePart compares two parts of a version: as numbers when both are,
// otherwise as text.
func comparePart(a, b string) int {
	if isNumber(a) && isNumber(b) {
		na, _ := strconv.Atoi(a)
		nb, _ := strconv.Atoi(b)
		return cmp.Compare(na, nb)
	}
	return strings.Compare(a, b)
}

// TagVersion is the version a tag (or a release's name) stands for: with
// the repository's or package's name and a dash, then release-, then v,
// taken off the front. ok is false when what's left doesn't start with a
// digit.
func TagVersion(tag, repo, name string) (string, bool) {
	t := strings.TrimSpace(tag)
	for _, p := range []string{repo + "-", name + "-", "release-", "v"} {
		if len(t) > len(p) && strings.EqualFold(t[:len(p)], p) {
			t = t[len(p):]
		}
	}
	if t == "" || t[0] < '0' || t[0] > '9' {
		return "", false
	}
	return t, true
}

// releaseVersion is the version r stands for: its tag's, or else its
// name's.
func releaseVersion(r github.Release, repo, name string) (string, bool) {
	if v, ok := TagVersion(r.Tag, repo, name); ok {
		return v, true
	}
	return TagVersion(r.Name, repo, name)
}

// isPre reports whether version v is a pre-release: it has a letter in it.
func isPre(v string) bool {
	return strings.ContainsAny(strings.ToLower(v), "abcdefghijklmnopqrstuvwxyz")
}

// Pick is the releases after from, up to and including to, newest first.
// from and to are brew's versions. A release whose tag and name both fail
// to read as versions is left out, and so are pre-releases, unless to is
// one.
func Pick(rels []github.Release, repo, name, from, to string) []github.Release {
	from, to = Clean(from), Clean(to)
	type found struct {
		rel github.Release
		v   string
	}
	var in []found
	for _, r := range rels {
		v, ok := releaseVersion(r, repo, name)
		if !ok || (r.Prerelease && !isPre(to)) {
			continue
		}
		if compare(v, from) > 0 && compare(v, to) <= 0 {
			in = append(in, found{r, v})
		}
	}
	slices.SortStableFunc(in, func(a, b found) int { return compare(b.v, a.v) })
	out := make([]github.Release, len(in))
	for i, f := range in {
		out[i] = f.rel
	}
	return out
}

// Link is where to read about the change from from to to on GitHub: the
// compare page when both versions have a tag among tags, otherwise the
// releases page.
func Link(owner, repo, name string, tags []string, from, to string) string {
	from, to = Clean(from), Clean(to)
	var old, cur string
	for _, t := range tags {
		v, ok := TagVersion(t, repo, name)
		switch {
		case !ok:
		case old == "" && compare(v, from) == 0:
			old = t
		case cur == "" && compare(v, to) == 0:
			cur = t
		}
	}
	base := "https://github.com/" + owner + "/" + repo
	if old == "" || cur == "" {
		return base + "/releases"
	}
	return base + "/compare/" + url.PathEscape(old) + "..." + url.PathEscape(cur)
}

// breakingWords are the words that flag a breaking change, whole and in
// any case.
var breakingWords = regexp.MustCompile(`(?i)\b(breaking|removed|deprecated|incompatible|migration)\b`)

// Breaking reports whether release notes mention a breaking change.
func Breaking(body string) bool { return breakingWords.MatchString(body) }

// BreakingSpans are where in s the words that flag a breaking change are,
// as start and end byte offsets.
func BreakingSpans(s string) [][]int { return breakingWords.FindAllStringIndex(s, -1) }
