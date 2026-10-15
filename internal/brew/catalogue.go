package brew

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// Catalogue is every formula and cask brew can install, with their
// descriptions, from brew's local package data.
func (c *Client) Catalogue(ctx context.Context) ([]Entry, error) {
	out, err := c.query(ctx, "search", "--desc", "")
	if err != nil {
		return nil, err
	}
	return ParseCatalogue(string(out)), nil
}

// ParseCatalogue reads `brew search --desc ""`: "name: description" lines
// under "==> Formulae" and "==> Casks".
func ParseCatalogue(out string) []Entry {
	var es []Entry
	kind, in := KindFormula, false
	for _, line := range strings.Split(out, "\n") {
		switch line = strings.TrimSpace(line); line {
		case "==> Formulae":
			kind, in = KindFormula, true
			continue
		case "==> Casks":
			kind, in = KindCask, true
			continue
		}
		name, desc, ok := strings.Cut(line, ":")
		if !in || !ok || name == "" || strings.Contains(name, " ") {
			continue
		}
		desc = strings.TrimSpace(desc)
		if kind == KindCask {
			desc = dropDisplayNames(desc)
		}
		es = append(es, Entry{Name: name, Desc: desc, Kind: kind})
	}
	return es
}

// dropDisplayNames drops the display names in brackets that start a cask's
// description, which can nest: "(ApE (A Plasmid Editor)) Software for…".
// A cask with no description keeps its display names instead.
func dropDisplayNames(desc string) string {
	if !strings.HasPrefix(desc, "(") {
		return desc
	}
	depth := 0
	for i, r := range desc {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth > 0 {
				continue
			}
			if rest := strings.TrimSpace(desc[i+1:]); rest != "" && rest != "[no description]" {
				return rest
			}
			return desc[1:i]
		}
	}
	return desc
}

// NewSinceUpdate lists what brew's last update added. brew keeps the name
// lists from before an update beside the current ones; without them (brew
// hasn't updated since it started keeping them) there is nothing new.
func (c *Client) NewSinceUpdate(ctx context.Context) ([]Entry, error) {
	out, err := c.query(ctx, "--cache")
	if err != nil {
		return nil, err
	}
	api := filepath.Join(strings.TrimSpace(string(out)), "api")
	var es []Entry
	for _, l := range []struct {
		file string
		kind Kind
	}{{"formula_names", KindFormula}, {"cask_names", KindCask}} {
		before, err := os.ReadFile(filepath.Join(api, l.file+".before.txt"))
		if err != nil {
			continue
		}
		now, err := os.ReadFile(filepath.Join(api, l.file+".txt"))
		if err != nil {
			continue
		}
		for _, name := range added(string(before), string(now)) {
			es = append(es, Entry{Name: name, Kind: l.kind})
		}
	}
	return es, nil
}

// added lists the names in now that aren't in before, one name a line.
func added(before, now string) []string {
	had := map[string]bool{}
	for _, n := range strings.Fields(before) {
		had[n] = true
	}
	var out []string
	for _, n := range strings.Fields(now) {
		if !had[n] {
			out = append(out, n)
		}
	}
	return out
}
