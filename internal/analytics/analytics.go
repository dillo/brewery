// Package analytics reads Homebrew's public install counts from
// formulae.brew.sh: how often each formula and cask was installed.
package analytics

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// List is one of Homebrew's analytics lists, most installed first.
type List struct {
	Start, End string // the days it covers, as "2026-09-15"
	Items      []Item
}

// Item is one package's installs over the list's days.
type Item struct {
	Name  string // "user/tap/name" for packages outside Homebrew's own taps
	Rank  int
	Count int
}

type listJSON struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
	Items     []struct {
		Number  int    `json:"number"`
		Formula string `json:"formula"`
		Cask    string `json:"cask"`
		Count   string `json:"count"`
	} `json:"items"`
}

// Parse reads one list. Counts come as text with thousands separators,
// "217,104"; an item whose count isn't a number is skipped.
func Parse(b []byte) (List, error) {
	var in listJSON
	if err := json.Unmarshal(b, &in); err != nil {
		return List{}, fmt.Errorf("parse analytics: %w", err)
	}
	if in.EndDate == "" {
		return List{}, errors.New("parse analytics: not an analytics list")
	}
	l := List{Start: in.StartDate, End: in.EndDate, Items: make([]Item, 0, len(in.Items))}
	for _, it := range in.Items {
		name := cmp.Or(it.Formula, it.Cask)
		n, err := strconv.Atoi(strings.ReplaceAll(it.Count, ",", ""))
		if name == "" || err != nil {
			continue
		}
		l.Items = append(l.Items, Item{Name: name, Rank: it.Number, Count: n})
	}
	return l, nil
}

// Counts maps each package in the list to its installs.
func (l List) Counts() map[string]int {
	m := make(map[string]int, len(l.Items))
	for _, it := range l.Items {
		m[it.Name] = it.Count
	}
	return m
}

// MinInstalls is how many installs in the last 30 days a package needs to
// count as rising, so a jump from 2 to 10 doesn't top the list.
const MinInstalls = 300

// NewBelow is the most installs a month a package can have averaged before
// the last 30 days and still count as new: a handful of installs before a
// release doesn't make it ×5000.
const NewBelow = 10

// Riser is a package whose installs grew.
type Riser struct {
	Name   string
	Count  int     // installs in the last 30 days
	Growth float64 // installs now over a month's installs before; 0 when New
	New    bool    // (almost) no installs before the last 30 days
}

// Rising compares each package's last 30 days with the 60 days before
// them, which the 90-day list covers too: growth is the last 30 days'
// installs over the average 30 days before. Packages that didn't grow are
// left out. The result is in Compare's order.
func Rising(d30, d90 List) []Riser {
	before := d90.Counts()
	var rs []Riser
	for _, it := range d30.Items {
		if it.Count < MinInstalls {
			continue
		}
		r := Riser{Name: it.Name, Count: it.Count}
		if month := float64(before[it.Name]-it.Count) / 2; month >= NewBelow {
			r.Growth = float64(it.Count) / month
		} else {
			r.New = true
		}
		if r.New || r.Growth > 1 {
			rs = append(rs, r)
		}
	}
	slices.SortFunc(rs, Compare)
	return rs
}

// Compare orders risers: new packages first, then by growth, then by
// installs, then by name.
func Compare(a, b Riser) int {
	if a.New != b.New {
		if a.New {
			return -1
		}
		return 1
	}
	return cmp.Or(cmp.Compare(b.Growth, a.Growth), cmp.Compare(b.Count, a.Count), strings.Compare(a.Name, b.Name))
}
