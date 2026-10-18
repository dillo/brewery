package analytics

import (
	"os"
	"reflect"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParse(t *testing.T) {
	l, err := Parse(fixture(t, "install-on-request-30d"))
	if err != nil {
		t.Fatal(err)
	}
	if l.Start != "2026-09-15" || l.End != "2026-10-15" || len(l.Items) != 7 {
		t.Fatalf("got %s to %s, %d items", l.Start, l.End, len(l.Items))
	}
	if got, want := l.Items[0], (Item{Name: "gh", Rank: 1, Count: 217104}); got != want {
		t.Errorf("first item %+v, want %+v", got, want)
	}
	if got := l.Items[6]; got.Name != "dagsommer/boks/boks" || got.Count != 1 {
		t.Errorf("last item %+v", got)
	}

	casks, err := Parse(fixture(t, "cask-install-30d"))
	if err != nil || casks.Items[0] != (Item{Name: "codex", Rank: 1, Count: 135078}) {
		t.Errorf("casks: %+v, %v", casks.Items, err)
	}
}

func TestParseRejects(t *testing.T) {
	for _, b := range []string{`not json`, `{"error":"rate limited"}`, `<html>`} {
		if _, err := Parse([]byte(b)); err == nil {
			t.Errorf("%q: want an error", b)
		}
	}
	// A broken item is skipped, not fatal.
	l, err := Parse([]byte(`{"start_date":"a","end_date":"b","items":[{"number":1,"formula":"x","count":"n/a"},{"number":2,"formula":"y","count":"1,000"}]}`))
	if err != nil || !reflect.DeepEqual(l.Items, []Item{{Name: "y", Rank: 2, Count: 1000}}) {
		t.Errorf("got %+v, %v", l.Items, err)
	}
}

func list(items ...Item) List { return List{Start: "s", End: "e", Items: items} }

func TestRising(t *testing.T) {
	d30 := list(
		Item{"steady", 1, 3000},  // 3000 a month before too: ×1, so not rising
		Item{"doubled", 2, 2000}, // 1000 a month before: ×2
		Item{"fresh", 3, 900},    // 9 a month before: too few to count
		Item{"newer", 4, 1500},   // nothing before either, but more installs
		Item{"tied-b", 5, 600},   // ×2, like doubled, with fewer installs
		Item{"falling", 8, 1000}, // 2000 a month before: ×0.5
		Item{"tied-a", 6, 600},   // same again: then by name
		Item{"tiny", 7, 299},     // under the floor
	)
	d90 := list(
		Item{"steady", 1, 9000}, Item{"doubled", 2, 4000}, Item{"fresh", 3, 918},
		Item{"tied-b", 4, 1200}, Item{"tied-a", 5, 1200}, Item{"tiny", 6, 300}, Item{"falling", 7, 5000},
	)
	got := Rising(d30, d90)
	want := []Riser{
		{Name: "newer", Count: 1500, New: true},
		{Name: "fresh", Count: 900, New: true},
		{Name: "doubled", Count: 2000, Growth: 2},
		{Name: "tied-a", Count: 600, Growth: 2},
		{Name: "tied-b", Count: 600, Growth: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestRisingFromFixtures(t *testing.T) {
	d30, _ := Parse(fixture(t, "install-on-request-30d"))
	d90, _ := Parse(fixture(t, "install-on-request-90d"))
	rs := Rising(d30, d90)
	if rs[0].Name != "python@3.15" || !rs[0].New {
		t.Errorf("first %+v, want python@3.15, new", rs[0])
	}
	if rs[1].Name != "wget" || rs[1].Growth < 4 || rs[1].Growth > 4.01 {
		t.Errorf("second %+v, want wget at ×4.0", rs[1])
	}
	if len(rs) != 3 || rs[2].Name != "git" {
		t.Errorf("got %+v, want python@3.15, wget and git: the rest didn't grow", rs)
	}
}

func TestCounts(t *testing.T) {
	l := list(Item{"a", 1, 10}, Item{"b", 2, 5})
	if got := l.Counts(); !reflect.DeepEqual(got, map[string]int{"a": 10, "b": 5}) {
		t.Errorf("got %v", got)
	}
}

func TestCompareNewFirst(t *testing.T) {
	fresh := Riser{Name: "b", Count: 10, New: true}
	grown := Riser{Name: "a", Count: 9000, Growth: 50}
	if got := Compare(grown, fresh); got != 1 {
		t.Errorf("Compare(grown, new) = %d, want 1: a new package comes first, however much the other grew", got)
	}
	if got := Compare(fresh, grown); got != -1 {
		t.Errorf("Compare(new, grown) = %d, want -1", got)
	}
}
