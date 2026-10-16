package brew

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
)

// The oldest Homebrew brewery supports: the version it was built and tested
// against.
const (
	MinMajor = 7
	MinMinor = 0
)

// Version is a Homebrew version: Major and Minor to compare, and Text as
// brew printed it, up to any "-" suffix ("7.0.9").
type Version struct {
	Major, Minor int
	Text         string
}

// TooOld reports whether v is older than MinMajor.MinMinor.
func (v Version) TooOld() bool {
	return v.Major < MinMajor || (v.Major == MinMajor && v.Minor < MinMinor)
}

var versionRe = regexp.MustCompile(`(?m)^Homebrew (\d+)\.(\d+)(\.\d+)?`)

// ParseVersion reads the first "Homebrew <major>.<minor>[.<patch>]" line of
// brew --version's output.
func ParseVersion(out string) (Version, bool) {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return Version{}, false
	}
	major, _ := strconv.Atoi(m[1])
	minor, _ := strconv.Atoi(m[2])
	return Version{Major: major, Minor: minor, Text: m[1] + "." + m[2] + m[3]}, true
}

// BrewVersion runs brew --version and parses it.
func (c *Client) BrewVersion(ctx context.Context) (Version, error) {
	out, err := c.query(ctx, "--version")
	if err != nil {
		return Version{}, err
	}
	v, ok := ParseVersion(string(out))
	if !ok {
		return Version{}, fmt.Errorf("can't read the version in %q", out)
	}
	return v, nil
}
