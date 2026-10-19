package ui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/dillo/brewery/internal/jobs"
)

// testMenu is a menu whose items record which one ran.
func testMenu(ran *string) *menuBox {
	item := func(key, label string) menuItem {
		return menuItem{key, label, func() tea.Cmd { *ran = key; return nil }}
	}
	return &menuBox{title: "fd 10.5.0", items: []menuItem{
		item("u", "Upgrade to 10.6.0"), item("x", "Uninstall…"), item("gx", "Open the homepage"),
	}}
}

func TestMenuKeys(t *testing.T) {
	var ran string
	pick := func(m *menuBox, ks ...string) (done bool) {
		for _, k := range ks {
			run, d := m.key(k)
			if run != nil {
				run()
			}
			done = d
		}
		return done
	}

	m := testMenu(&ran)
	if done := pick(m, "k"); done || m.cursor != 2 {
		t.Errorf("k from the top wraps to the last: cursor %d, done %v", m.cursor, done)
	}
	if done := pick(m, "j", "down"); done || m.cursor != 1 {
		t.Errorf("j and down move down, wrapping: cursor %d", m.cursor)
	}
	if done := pick(m, "up"); done || m.cursor != 0 {
		t.Errorf("up moves up: cursor %d", m.cursor)
	}
	if done := pick(m, "j", "enter"); !done || ran != "x" {
		t.Errorf("enter runs the item under the cursor: ran %q, done %v", ran, done)
	}

	for _, tt := range []struct {
		keys []string
		ran  string
		done bool
	}{
		{[]string{"u"}, "u", true},
		{[]string{"x"}, "x", true},
		{[]string{"g", "x"}, "gx", true},
		{[]string{"g", "u"}, "", false}, // g then anything but x does nothing
		{[]string{"g", "esc"}, "", true},
		{[]string{"g", "q"}, "", true},
		{[]string{"esc"}, "", true},
		{[]string{"q"}, "", true},
		{[]string{"z"}, "", false},
	} {
		ran = ""
		if done := pick(testMenu(&ran), tt.keys...); done != tt.done || ran != tt.ran {
			t.Errorf("%v: ran %q, done %v; want %q, %v", tt.keys, ran, done, tt.ran, tt.done)
		}
	}
}

func TestMenuView(t *testing.T) {
	var ran string
	m := testMenu(&ran)
	m.cursor = 1
	out := stripANSI(m.view(testStyles, 140))
	for _, want := range []string{"fd 10.5.0", "  Upgrade to 10.6.0", "› Uninstall…", "Open the homepage", "gx", menuFooter} {
		if !strings.Contains(out, want) {
			t.Errorf("menu lacks %q:\n%s", want, out)
		}
	}
	lines := strings.Split(out, "\n")
	for _, l := range lines[1 : len(lines)-1] {
		if w := len([]rune(l)); w != len([]rune(lines[0])) {
			t.Errorf("ragged box, line %q is %d wide, want %d:\n%s", l, w, len([]rune(lines[0])), out)
		}
	}
	for _, l := range lines {
		if strings.Contains(l, "Uninstall…") && !strings.HasSuffix(strings.TrimRight(l, " "), "x │") {
			t.Errorf("the key sits at the right of its item: %q", l)
		}
	}
	if narrow := stripANSI(m.view(testStyles, 24)); strings.Contains(narrow, "Upgrade to 10.6.0") {
		t.Errorf("a narrow menu truncates its labels:\n%s", narrow)
	}
}

// menuLabels opens the menu with enter and lists its title and labels;
// it closes the menu again.
func menuLabels(t *testing.T, a *App) (title string, labels []string) {
	t.Helper()
	keys(t, a, "enter")
	if a.menu == nil {
		t.Fatal("enter didn't open the menu")
	}
	for _, it := range a.menu.items {
		labels = append(labels, it.label)
	}
	title = a.menu.title
	keys(t, a, "esc")
	return title, labels
}

func TestMenuItemsOnInstalled(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	for _, tt := range []struct {
		name, title string
		want        []string
	}{
		{"lua-language-server", "lua-language-server 3.19.1", []string{"Upgrade to 3.19.2", "Fetch 3.19.2", "Reinstall…", "Unlink…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"fd", "fd 10.5.0", []string{"Reinstall…", "Unlink…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"git", "git 2.51.0", []string{"Reinstall…", "Unlink…", "Unpin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"json-c", "json-c 0.19", []string{"Reinstall…", "Link…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
		{"docker-desktop", "docker-desktop 4.39.0,184744", []string{"Upgrade to 4.95.0,242831", "Fetch 4.95.0,242831", "Reinstall…", "Pin", "Uninstall…", "Copy the name", "Open the homepage"}},
	} {
		selectInstalled(t, a, tt.name)
		if title, got := menuLabels(t, a); title != tt.title || !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: %q %q, want %q %q", tt.name, title, got, tt.title, tt.want)
		}
	}
	selectInstalled(t, a, "fd")
	keys(t, a, "space")
	selectInstalled(t, a, "git")
	keys(t, a, "space")
	if title, got := menuLabels(t, a); title != "2 marked" || !reflect.DeepEqual(got, []string{"Upgrade", "Fetch", "Reinstall…", "Uninstall…"}) {
		t.Errorf("marked: %q %q", title, got)
	}
}

func TestMenuItemsOnOtherTabs(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	selectOutdated(t, a, "lua-language-server")
	if title, got := menuLabels(t, a); title != "lua-language-server 3.19.1" ||
		!reflect.DeepEqual(got, []string{"Upgrade to 3.19.2", "Fetch 3.19.2", "Upgrade all…", "Copy the name", "Open the homepage"}) {
		t.Errorf("Outdated: %q %q", title, got)
	}
	keys(t, a, "space")
	if title, got := menuLabels(t, a); title != "1 marked" || !reflect.DeepEqual(got, []string{"Upgrade", "Fetch"}) {
		t.Errorf("Outdated, marked: %q %q", title, got)
	}

	show(t, a, "Services")
	keys(t, a, "g", "g") // postgresql@16, in error
	if title, got := menuLabels(t, a); title != "postgresql@16" || !reflect.DeepEqual(got, []string{"Start", "Follow the log", "Copy the name"}) {
		t.Errorf("Services, stopped: %q %q", title, got)
	}
	keys(t, a, "j") // redis, started
	if _, got := menuLabels(t, a); !reflect.DeepEqual(got, []string{"Stop…", "Restart", "Follow the log", "Copy the name"}) {
		t.Errorf("Services, started: %q", got)
	}

	show(t, a, "Taps")
	keys(t, a, "g", "g") // anomalyco/tap, on GitHub
	if title, got := menuLabels(t, a); title != "anomalyco/tap" || !reflect.DeepEqual(got, []string{"Untap…", "Copy the name", "Open the homepage"}) {
		t.Errorf("Taps: %q %q", title, got)
	}

	a.UseHistory(jobs.History{Dir: t.TempDir()}, recentJobs(a.now()))
	show(t, a, "History")
	keys(t, a, "g", "g")
	if title, got := menuLabels(t, a); title != "brew upgrade fd" || !reflect.DeepEqual(got, []string{"Run again…", "Copy the command"}) {
		t.Errorf("History: %q %q", title, got)
	}

	a.UseCatalog(newFakeCatalog(t))
	show(t, a, "Search all")
	if _, got := menuLabels(t, a); len(got) < 2 || got[0] != "Install…" || got[1] != "Copy the name" {
		t.Errorf("Discover: %q", got)
	}
}

func TestEnterOpensTheMenuNotTheDetails(t *testing.T) {
	fs := &fakeStarter{}
	a := newJobApp(t, fs)
	selectInstalled(t, a, "fd")
	keys(t, a, "enter")
	if a.menu == nil || a.focus != colList {
		t.Fatalf("enter should open the menu and leave the focus: menu %v, focus %v", a.menu, a.focus)
	}
	if out := stripANSI(a.render()); !strings.Contains(out, "› Reinstall…") || !strings.Contains(stripANSI(a.statusBar()), menuFooter) {
		t.Errorf("the menu should show, with its hint:\n%s", out)
	}
	keys(t, a, "x") // any key goes to the menu while it's open: x picks Uninstall…
	if a.menu != nil || a.confirm == nil || a.confirm.title != "Uninstall fd 10.5.0?" {
		t.Fatalf("x in the menu should close it and ask: menu %v, confirm %+v", a.menu, a.confirm)
	}
	keys(t, a, "n")

	keys(t, a, "l", "enter") // from the details too
	if a.menu == nil || a.focus != colDetails {
		t.Errorf("enter in the details should open the menu: menu %v, focus %v", a.menu, a.focus)
	}
	keys(t, a, "esc", "esc")
	keys(t, a, "tab")
	if a.focus != colDetails {
		t.Errorf("tab still opens the details: focus %v", a.focus)
	}

	show(t, a, "Maintenance")
	keys(t, a, "enter")
	if a.menu != nil {
		t.Errorf("Maintenance's enter runs its row, with no menu: %+v", a.menu)
	}
}

func TestMenuNeedsARow(t *testing.T) {
	a := newJobApp(t, &fakeStarter{})
	a.snap.Outdated = nil
	a.sync()
	show(t, a, "Outdated")
	keys(t, a, "enter")
	if a.menu != nil {
		t.Errorf("an empty list has no menu: %+v", a.menu)
	}
}
