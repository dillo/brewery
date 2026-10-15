package ui

import (
	"strings"
	"testing"
	"time"
)

func TestOutdatedSaysHowOldBrewsDataIs(t *testing.T) {
	a := newTestApp(t, newFakeSource(t))
	drain(t, a, a.Init())
	keys(t, a, "3")
	if out := stripANSI(a.render()); !strings.Contains(out, "Outdated · 2 · brew's data from 6h ago") {
		t.Errorf("the title should say how old the data is:\n%s", out)
	}
	a.snap.Outdated = nil
	a.sync()
	if out := stripANSI(a.render()); !strings.Contains(out, "brew's data from 6h ago") || !strings.Contains(out, "up to date") {
		t.Errorf("\"up to date\" says as of when:\n%s", out)
	}
	a.snap.DataAt = time.Time{}
	if out := stripANSI(a.render()); strings.Contains(out, "brew's data") {
		t.Errorf("no time known, nothing said:\n%s", out)
	}
}
