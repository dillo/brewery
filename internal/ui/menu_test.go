package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
