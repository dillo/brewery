package ui

import (
	"cmp"
	"time"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/theme"
)

// formulaReasons and caskReasons are brew's phrases for its deprecation
// and disable keywords (Library/Homebrew/deprecate_disable.rb). A few long
// ones are shortened to fit the details.
var (
	formulaReasons = map[string]string{
		"does_not_build":      "does not build",
		"no_license":          "has no license",
		"repo_archived":       "has an archived upstream repository",
		"repo_removed":        "has a removed upstream repository",
		"unmaintained":        "is not maintained upstream",
		"unreachable":         "is no longer reliably reachable upstream",
		"unsupported":         "is not supported upstream",
		"deprecated_upstream": "is deprecated upstream",
		"versioned_formula":   "is a versioned formula",
		"checksum_mismatch":   "has a source file whose checksum changed upstream",
	}
	caskReasons = map[string]string{
		"discontinued":             "is discontinued upstream",
		"moved_to_mas":             "is now only on the Mac App Store",
		"no_longer_available":      "is no longer available upstream",
		"no_longer_meets_criteria": "no longer meets the criteria for casks",
		"unmaintained":             "is not maintained upstream",
		"fails_gatekeeper_check":   "does not pass the macOS Gatekeeper check",
		"unreachable":              "is no longer reliably reachable upstream",
	}
)

// retireState is "deprecated" or "disabled", as brew counts p, or "".
// Like brew, it takes deprecated first.
func retireState(p brew.Package) string {
	switch {
	case p.Deprecated:
		return "deprecated"
	case p.Disabled:
		return "disabled"
	}
	return ""
}

// reasonText is why p is deprecated or disabled, in brew's words: its
// phrase for a known keyword, or the reason as it is.
func reasonText(p brew.Package) string {
	phrases := formulaReasons
	if p.Kind == brew.KindCask {
		phrases = caskReasons
	}
	if s, ok := phrases[p.Reason]; ok {
		return s
	}
	return p.Reason
}

// disableLine says when p was or will be disabled: its disable date, or
// else a year after it was deprecated, as brew reckons it. It's "" when
// there's no date.
func disableLine(p brew.Package, now time.Time) string {
	day, err := time.Parse(time.DateOnly, p.DisableDate)
	if err != nil {
		deprecated, err := time.Parse(time.DateOnly, p.DeprecationDate)
		if err != nil {
			return ""
		}
		day = deprecated.AddDate(0, 12, 0)
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if day.Before(today) {
		return "disabled on " + day.Format(time.DateOnly)
	}
	return "will be disabled on " + day.Format(time.DateOnly)
}

// retireRows are the details' rows for a deprecated or disabled package,
// as kv pairs: the state and why, when it's disabled, and what to install
// instead. None for any other package.
func retireRows(st theme.Styles, p brew.Package, now time.Time) []string {
	state := retireState(p)
	if state == "" {
		return nil
	}
	style := st.Warn
	if state == "disabled" {
		style = st.Err
	}
	rows := []string{state, style.Render(cmp.Or(reasonText(p), "yes"))}
	if line := disableLine(p, now); line != "" {
		rows = append(rows, " ", st.Muted.Render(line)) // a blank key continues the row above
	}
	if p.Replacement != "" {
		rows = append(rows, "replacement", "brew install "+p.Replacement)
	}
	return rows
}
