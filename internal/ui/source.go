// Package ui is brewery's Bubble Tea front end. It never runs brew itself:
// it asks a Source for data and renders what comes back.
package ui

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"brewery/internal/brew"
	"brewery/internal/state"
)

// Source provides brew data. *brew.Client satisfies it; tests use a fake.
type Source interface {
	Installed(ctx context.Context) ([]brew.Package, error)
	Outdated(ctx context.Context) ([]brew.Outdated, error)
	Services(ctx context.Context) ([]brew.Service, error)
	Taps(ctx context.Context) ([]brew.Tap, error)
	Doctor(ctx context.Context) ([]brew.DoctorWarning, error)
	Cleanup(ctx context.Context) (brew.Cleanup, error)
	Sizes(ctx context.Context, pkgs []brew.Package) (map[string]int64, error)
}

var _ Source = (*brew.Client)(nil)

type query int

const (
	qInstalled query = iota
	qOutdated
	qServices
	qTaps
	qDoctor
	qCleanup
	qSizes
)

// queryNames name each query in the status bar.
var queryNames = [...]string{"installed", "outdated", "services", "taps", "doctor", "cleanup", "sizes"}

// resultMsg carries one query's result back to the App.
type resultMsg struct {
	q        query
	packages []brew.Package
	outdated []brew.Outdated
	services []brew.Service
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	cleanup  brew.Cleanup
	sizes    map[string]int64
	err      error
}

// jsonQueries are the queries a full refresh runs in parallel; doctor,
// cleanup's dry-runs and the disk sizes follow once they have all returned.
var jsonQueries = []query{qInstalled, qOutdated, qServices, qTaps}

// fetch runs one query. ctx is the App's root context, so quitting stops
// any brew process still running. Sizes are measured for pkgs.
func fetch(ctx context.Context, src Source, q query, pkgs []brew.Package) tea.Cmd {
	return func() tea.Msg {
		m := resultMsg{q: q}
		switch q {
		case qInstalled:
			m.packages, m.err = src.Installed(ctx)
		case qOutdated:
			m.outdated, m.err = src.Outdated(ctx)
		case qServices:
			m.services, m.err = src.Services(ctx)
		case qTaps:
			m.taps, m.err = src.Taps(ctx)
		case qDoctor:
			m.doctor, m.err = src.Doctor(ctx)
		case qCleanup:
			m.cleanup, m.err = src.Cleanup(ctx)
		case qSizes:
			m.sizes, m.err = src.Sizes(ctx, pkgs)
		}
		return m
	}
}

// savedMsg reports the result of writing the snapshot cache.
type savedMsg struct{ err error }

func save(path string, snap state.Snapshot) tea.Cmd {
	return func() tea.Msg {
		return savedMsg{err: state.Save(path, snap)}
	}
}
