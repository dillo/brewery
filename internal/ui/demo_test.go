//go:build demo

package ui

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/dillo/brewery/internal/brew"
)

// The demo video: `make demo` records this with VHS (docs/demo.tape). It is
// brewery on the screenshots' made-up machine, driven by a script of keys
// and captions instead of a person, so every take is the same. Run it by
// hand with `go test -tags demo -run TestDemo ./internal/ui` in a terminal
// of about 140×40, and the captions show in a band under the app. When
// DEMO_CAPTIONS names a file, the band is left out and each caption's start
// is written there instead, for docs/captions.swift to draw larger than a
// terminal can.

// captionH is the number of lines the caption band takes under the app.
const captionH = 3

// captionMsg sets the caption under the app; "" clears it.
type captionMsg string

// captioned is the app with a caption band below it, unless band is off.
type captioned struct {
	app     *App
	band    bool
	caption string
	w       int
}

func (c *captioned) Init() tea.Cmd { return c.app.Init() }

func (c *captioned) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case captionMsg:
		c.caption = string(m)
		return c, nil
	case tea.WindowSizeMsg:
		c.w = m.Width
		if c.band {
			m.Height -= captionH
		}
		msg = m
	}
	_, cmd := c.app.Update(msg)
	return c, cmd
}

func (c *captioned) View() tea.View {
	v := c.app.View()
	if !c.band {
		return v
	}
	st := c.app.st
	band := lipgloss.NewStyle().Width(c.w).Height(captionH).
		AlignHorizontal(lipgloss.Center).AlignVertical(lipgloss.Center).
		Background(st.P.Mantle).Foreground(st.P.Text).Bold(true)
	text := ""
	if c.caption != "" {
		text = lipgloss.NewStyle().Foreground(st.P.Mauve).Background(st.P.Mantle).Render("▍ ") +
			ansi.Truncate(c.caption, c.w-4, "…")
	}
	v.Content += "\n" + band.Render(text)
	return v
}

// liveSource is the demo machine, which gh stops being outdated on once
// the demo has upgraded it.
type liveSource struct {
	demoSource
	upgraded *atomic.Bool
}

func (l liveSource) Outdated(ctx context.Context) ([]brew.Outdated, error) {
	if !l.upgraded.Load() {
		return l.demoSource.Outdated(ctx)
	}
	var out []brew.Outdated
	for _, o := range l.snap.Outdated {
		if o.Name != "gh" {
			out = append(out, o)
		}
	}
	return out, nil
}

// installPipx is brew's output while it installs pipx.
const installPipx = "==> Fetching downloads for: pipx\r\n" +
	"==> Downloading https://ghcr.io/v2/homebrew/core/pipx/manifests/1.18.1\r\n" +
	"######################################################################### 100.0%\r\n" +
	"==> Fetching pipx\r\n" +
	"==> Downloading https://ghcr.io/v2/homebrew/core/pipx/blobs/sha256:9a0d3e…\r\n" +
	"######################################################################### 100.0%\r\n" +
	"==> Pouring pipx--1.18.1.arm64_tahoe.bottle.tar.gz\r\n" +
	"🍺  /opt/homebrew/Cellar/pipx/1.18.1: 1,032 files, 6.1MB\r\n"

// scriptedStart starts a pretend brew that prints its output a line at a
// time, as a real download would, then exits.
func scriptedStart(upgraded *atomic.Bool) Starter {
	return func(args, _ []string, _, _ int) (Proc, error) {
		out := ""
		switch args[0] {
		case "upgrade":
			out = upgradeGh + "🍺  /opt/homebrew/Cellar/gh/2.102.0: 222 files, 37.8MB\r\n"
		case "install":
			out = installPipx
		}
		p := &fakeProc{out: make(chan []byte, 64)}
		go func() {
			for _, line := range strings.SplitAfter(out, "\r\n") {
				time.Sleep(450 * time.Millisecond)
				p.out <- []byte(line)
			}
			if args[0] == "upgrade" {
				upgraded.Store(true)
			}
			p.exit()
		}()
		return p, nil
	}
}

// step is one beat of the demo: a caption (kept if ""), then keys typed a
// beat apart, then a pause to look.
type step struct {
	caption string
	keys    []string
	hold    time.Duration
}

// typed is s as one key per character.
func typed(s string) []string { return strings.Split(s, "") }

// jump goes to the sidebar entry that starts with prefix, from any pane,
// as a person would: back to the sidebar, then / and a few letters.
func jump(prefix string) []string {
	return slices.Concat([]string{"h", "h", "h", "/"}, typed(prefix), []string{"enter", "l"})
}

// demoScript is the demo, about a minute and a half of it.
var demoScript = []step{
	{caption: "brewery · a terminal UI for Homebrew", hold: 3 * time.Second},
	{caption: "Home: what's outdated, vulnerable or worth a look, at a glance", keys: []string{"l", "l", "l"}, hold: 2 * time.Second},

	{caption: "Everything installed, with its dependency tree and what uses it",
		keys: slices.Concat([]string{"h", "h", "h", "h", "j", "j", "l"}, typed("jjjjjjjjj"), []string{"l"}), hold: 3 * time.Second},
	{caption: "/ filters any list as you type", keys: slices.Concat([]string{"h"}, typed("/python")), hold: 2 * time.Second},
	{keys: []string{"esc", "esc"}, hold: time.Second},

	{caption: "Jump anywhere from the sidebar: / and a few letters", keys: slices.Concat([]string{"h", "h", "/"}, typed("outd")), hold: 1500 * time.Millisecond},
	{caption: "Outdated: what has a newer version", keys: []string{"enter", "l"}, hold: 2 * time.Second},
	{caption: "u upgrades · brew runs in a drawer, its output live", keys: slices.Concat(typed("/gh"), []string{"enter", "u"}), hold: 6 * time.Second},
	{caption: "When the job ends, brewery refreshes: gh is up to date", keys: []string{"o"}, hold: 3 * time.Second},

	{caption: "Skunky: known vulnerabilities, from brew vulns", keys: jump("skun"), hold: 3 * time.Second},

	{caption: "Discover: popular, rising and new, from Homebrew's analytics", keys: jump("popular t"), hold: 3 * time.Second},
	{caption: "Find something, then a installs it", keys: slices.Concat(typed("/pipx"), []string{"enter"}), hold: 2 * time.Second},
	{keys: []string{"a"}, hold: 2500 * time.Millisecond},
	{caption: "Installs ask first, and say what comes along", keys: []string{"y"}, hold: 5 * time.Second},

	{caption: "Services: start, stop and read their logs", keys: slices.Concat([]string{"o"}, jump("serv"), []string{"l"}), hold: 3500 * time.Millisecond},
	{caption: "Maintenance: doctor, cleanup and tap health in one place", keys: jump("main"), hold: 3 * time.Second},
	{caption: "Vim-style keys throughout · ? shows them all", keys: []string{"?"}, hold: 3500 * time.Millisecond},
	{caption: "brewery · github.com/dillo/brewery", keys: []string{"esc", "1"}, hold: 4 * time.Second},
}

func TestDemo(t *testing.T) {
	var upgraded atomic.Bool
	a := demoApp(t, &fakeStarter{})
	a.src = liveSource{demoSource: a.src.(demoSource), upgraded: &upgraded}
	a.start = scriptedStart(&upgraded)
	a.tickEvery = 100 * time.Millisecond // the job spinner turns
	began := time.Now()
	a.now = func() time.Time { return demoNow.Add(time.Since(began)) } // jobs' clocks run

	// Each caption's start, in seconds, then a tab and the caption; the
	// last line is when the demo ends, with no caption.
	logCaption := func(string) {}
	path := os.Getenv("DEMO_CAPTIONS")
	if path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		logCaption = func(c string) { fmt.Fprintf(f, "%.2f\t%s\n", time.Since(began).Seconds(), c) }
	}

	p := tea.NewProgram(&captioned{app: a, band: path == ""})
	go func() {
		time.Sleep(1500 * time.Millisecond) // the first frame
		for _, s := range demoScript {
			if s.caption != "" {
				p.Send(captionMsg(s.caption))
				logCaption(s.caption)
			}
			for _, k := range s.keys {
				time.Sleep(140 * time.Millisecond)
				p.Send(press(k))
			}
			time.Sleep(s.hold)
		}
		logCaption("")
		p.Send(captionMsg(""))
		p.Send(press("q"))
	}()
	if _, err := p.Run(); err != nil {
		t.Fatal(err)
	}
}
