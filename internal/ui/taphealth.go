package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
	"github.com/dillo/brewery/internal/github"
	"github.com/dillo/brewery/internal/theme"
)

// TapChecker asks GitHub how a tap's repository stands. *github.Client
// satisfies it; tests use a fake.
type TapChecker interface {
	Repo(ctx context.Context, owner, repo string) (github.TapCheck, error)
}

var _ TapChecker = (*github.Client)(nil)

// UseTapChecker sets how Maintenance checks taps on GitHub.
func (a *App) UseTapChecker(c TapChecker) { a.tapChecker = c }

// tapsCheckedMsg carries a whole tap check's results, and when it ended.
type tapsCheckedMsg struct {
	checks []github.TapCheck
	at     time.Time
}

// reasonW is how much of a failed check's reason the details show.
const reasonW = 40

// thirdPartyTaps are the installed taps that aren't brew's own.
func (a *App) thirdPartyTaps() []brew.Tap {
	var out []brew.Tap
	for _, t := range a.snap.Taps {
		if !t.Official {
			out = append(out, t)
		}
	}
	return out
}

// tapsToCheck are the third-party taps on GitHub.
func (a *App) tapsToCheck() []brew.Tap {
	var out []brew.Tap
	for _, t := range a.thirdPartyTaps() {
		if _, _, ok := github.RepoOf(t.Remote); ok {
			out = append(out, t)
		}
	}
	return out
}

// checkTaps starts asking GitHub about each tap, unless a check is already
// running or there's nothing to check.
func (a *App) checkTaps() tea.Cmd {
	taps := a.tapsToCheck()
	switch {
	case a.tapChecker == nil:
		return a.note("checking taps isn't available")
	case a.checkingTaps:
		return a.note("already checking taps")
	case len(taps) == 0:
		return a.note("no taps to check")
	}
	a.checkingTaps = true
	ctx, checker, now := a.ctx, a.tapChecker, a.now
	return tea.Batch(a.note("checking taps on GitHub…"), func() tea.Msg {
		return tapsCheckedMsg{checks: runTapChecks(ctx, checker, taps), at: now()}
	})
}

// runTapChecks checks each tap in turn. Once GitHub's rate limit is
// reached or ctx is done, the rest aren't asked about and say why.
func runTapChecks(ctx context.Context, c TapChecker, taps []brew.Tap) []github.TapCheck {
	checks := make([]github.TapCheck, len(taps))
	limited := false
	for i, t := range taps {
		checks[i] = github.TapCheck{Tap: t.Name, State: github.Unknown, Reason: github.ErrRateLimited.Error()}
		if err := ctx.Err(); err != nil {
			checks[i].Reason = err.Error()
			continue
		}
		if limited {
			continue
		}
		owner, repo, _ := github.RepoOf(t.Remote)
		res, err := c.Repo(ctx, owner, repo)
		switch {
		case errors.Is(err, github.ErrRateLimited):
			limited = true
		case err != nil:
			checks[i].Reason = err.Error()
		default:
			res.Tap = t.Name
			checks[i] = res
		}
	}
	return checks
}

// gotTapChecks keeps a finished check's results and saves them.
func (a *App) gotTapChecks(m tapsCheckedMsg) tea.Cmd {
	a.checkingTaps = false
	a.snap.TapChecks, a.snap.TapsCheckedAt = m.checks, m.at
	a.sync()
	return save(a.cachePath, a.snap)
}

// tapCheck is the last check's result for the tap named name.
func (a *App) tapCheck(name string) (github.TapCheck, bool) {
	for _, c := range a.snap.TapChecks {
		if c.Tap == name {
			return c, true
		}
	}
	return github.TapCheck{}, false
}

// checkText says what c found, and whether that's a problem.
func checkText(c github.TapCheck) (text string, problem bool) {
	switch c.State {
	case github.Fine:
		return "fine", false
	case github.Moved:
		return "moved to " + c.MovedTo, true
	case github.Unknown:
		return "couldn't check: " + c.Reason, true
	}
	return string(c.State), true // archived or gone
}

// tapsStatus is the Check taps row's status.
func (a *App) tapsStatus() string {
	st := a.st
	taps := a.tapsToCheck()
	switch {
	case len(taps) == 0:
		return theme.Tint(st.P.Overlay0, "no taps to check")
	case a.checkingTaps:
		return theme.Tint(st.P.Overlay0, "checking…")
	case a.snap.TapsCheckedAt.IsZero():
		return theme.Tint(st.P.Overlay0, "not checked yet")
	}
	n := map[github.State]int{}
	unchecked := 0
	for _, t := range taps {
		if c, ok := a.tapCheck(t.Name); ok {
			n[c.State]++
		} else {
			unchecked++
		}
	}
	if unchecked == len(taps) {
		return theme.Tint(st.P.Overlay0, "not checked yet")
	}
	var parts []string
	for _, s := range []github.State{github.Archived, github.Moved, github.Gone} {
		if n[s] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n[s], s))
		}
	}
	color := st.P.Yellow
	if len(parts) == 0 && n[github.Fine] > 0 {
		parts, color = []string{fmt.Sprintf("%d fine", n[github.Fine])}, st.P.Green
	}
	if n[github.Unknown] > 0 {
		parts = append(parts, fmt.Sprintf("%d couldn't check", n[github.Unknown]))
	}
	if unchecked > 0 {
		parts = append(parts, fmt.Sprintf("%d not checked yet", unchecked))
	}
	return theme.Tint(color, strings.Join(parts, " · "))
}

// tapsPreview is the Check taps row's details: when the check last ran,
// and what it found for each third-party tap.
func (a *App) tapsPreview() string {
	st := a.st
	taps := a.thirdPartyTaps()
	if len(taps) == 0 {
		return st.Muted.Render("no third-party taps to check")
	}
	when := "not checked yet"
	switch {
	case a.checkingTaps:
		when = "checking…"
	case !a.snap.TapsCheckedAt.IsZero():
		when = "checked " + ago(a.now(), a.snap.TapsCheckedAt)
	}
	var pairs []string
	for _, t := range taps {
		pairs = append(pairs, t.Name, a.tapLine(t))
	}
	return st.Muted.Render(when) + "\n\n" + kv(st, pairs...) + "\n" + st.Muted.Render("enter checks them on GitHub")
}

// tapLine is one tap's line in the details, coloured by what was found.
func (a *App) tapLine(t brew.Tap) string {
	st := a.st
	if _, _, ok := github.RepoOf(t.Remote); !ok {
		return st.Muted.Render("not on GitHub")
	}
	c, ok := a.tapCheck(t.Name)
	if !ok {
		return st.Muted.Render("not checked yet")
	}
	text, problem := checkText(c)
	text = ansi.Truncate(text, reasonW, "…")
	if problem {
		return st.Warn.Render(text)
	}
	return st.OK.Render(text)
}

// tapColumn is a tap's GITHUB cell: empty unless it's on GitHub and has
// been checked.
func (a *App) tapColumn(t brew.Tap) string {
	c, ok := a.tapCheck(t.Name)
	if _, _, onGitHub := github.RepoOf(t.Remote); t.Official || !onGitHub || !ok {
		return ""
	}
	text, problem := checkText(c)
	if problem {
		return theme.Tint(a.st.P.Yellow, text)
	}
	return text
}
