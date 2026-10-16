package ui

import (
	"context"
	"fmt"
	"strconv"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/analytics"
)

// Analytics are Homebrew's install counts, downloaded and kept between
// runs. analytics.Store satisfies it; tests use a fake.
type Analytics interface {
	Cached(list string) (analytics.List, time.Time, error)
	Fetch(ctx context.Context, list string) (analytics.List, error)
	Tried() string // the day a fetch last started, as 2006-01-02
	SetTried(day string) error
}

var _ Analytics = analytics.Store{}

// UseAnalytics sets where Discover's install counts come from.
func (a *App) UseAnalytics(an Analytics) { a.analytics = an }

// Messages from reading the kept analytics and downloading them.
type (
	cachedStatsMsg struct {
		lists map[string]analytics.List
		at    map[string]time.Time
		tried string // the day a fetch last started, in this run or an earlier one
	}
	fetchedStatsMsg struct {
		i   int // the list's index in analytics.Lists
		l   analytics.List
		err error
	}
)

// wantStats reads the kept analytics the first time Discover opens; after
// that it fetches them if they aren't from today.
func (a *App) wantStats() tea.Cmd {
	if a.analytics == nil {
		return nil
	}
	if !a.discover.statsRead {
		a.discover.statsRead = true
		an := a.analytics
		return func() tea.Msg {
			m := cachedStatsMsg{lists: map[string]analytics.List{}, at: map[string]time.Time{}, tried: an.Tried()}
			for _, name := range analytics.Lists {
				if l, at, err := an.Cached(name); err == nil {
					m.lists[name], m.at[name] = l, at
				}
			}
			return m
		}
	}
	return a.fetchStats(false)
}

func (a *App) gotCachedStats(m cachedStatsMsg) tea.Cmd {
	d := &a.discover
	if d.triedOn == "" {
		d.triedOn = m.tried
	}
	for name, l := range m.lists {
		if _, ok := d.stats[name]; !ok { // a download that finished first is newer
			d.stats[name], d.statsAt[name] = l, m.at[name]
		}
	}
	d.rebuild()
	return a.fetchStats(false)
}

// fetchStats downloads every list, one after another, unless the copies
// are all from today or a fetch already started today, in this run or an
// earlier one. force (r) fetches anyway. Only one fetch runs at a time.
func (a *App) fetchStats(force bool) tea.Cmd {
	d := &a.discover
	today := a.now().Format(time.DateOnly)
	if a.analytics == nil || d.fetching || (!force && (d.triedOn == today || d.statsFrom(today))) {
		return nil
	}
	d.fetching, d.triedOn, d.fetchErr = true, today, nil
	an := a.analytics
	remember := func() tea.Msg {
		_ = an.SetTried(today) // without it, the next start may fetch again today
		return nil
	}
	return tea.Batch(a.fetchList(0), remember)
}

func (a *App) fetchList(i int) tea.Cmd {
	an, ctx := a.analytics, a.ctx
	return func() tea.Msg {
		l, err := an.Fetch(ctx, analytics.Lists[i])
		return fetchedStatsMsg{i: i, l: l, err: err}
	}
}

// gotStats takes one downloaded list and fetches the next. A failure
// stops the fetch: offline, every list would fail the same way.
func (a *App) gotStats(m fetchedStatsMsg) tea.Cmd {
	d := &a.discover
	if m.err != nil {
		d.fetching = false
		if a.ctx.Err() == nil { // quitting cancels a download: that's no failure
			d.fetchErr = m.err
		}
		return nil
	}
	name := analytics.Lists[m.i]
	d.stats[name], d.statsAt[name] = m.l, a.now()
	d.rebuild()
	if m.i+1 < len(analytics.Lists) {
		return a.fetchList(m.i + 1)
	}
	d.fetching = false
	return nil
}

// statsAge notes a fetch in progress, or how old the analytics are when
// they aren't from today.
func (a *App) statsAge() string {
	d := &a.discover
	if d.fetching {
		return "fetching analytics…"
	}
	var oldest time.Time
	for _, at := range d.statsAt {
		if oldest.IsZero() || at.Before(oldest) {
			oldest = at
		}
	}
	if oldest.IsZero() || oldest.Format(time.DateOnly) == a.now().Format(time.DateOnly) {
		return ""
	}
	return "analytics from " + oldest.Format("Jan 2")
}

// shortCount writes an install count in a few characters: 249, 2.8k,
// 217k, 1.2M. Zero, not known, is blank.
func shortCount(n int) string {
	switch {
	case n == 0:
		return ""
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return strconv.Itoa(n/1000) + "k"
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return strconv.Itoa(n)
}

// growthCell is a riser's GROWTH: "new", or how many times its installs
// grew, "×4.1".
func growthCell(r discRow) string {
	switch {
	case r.fresh:
		return "new"
	case r.growth < 10:
		return fmt.Sprintf("×%.1f", r.growth)
	}
	return fmt.Sprintf("×%.0f", r.growth)
}
