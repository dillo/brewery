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
}

type query int

const (
	qInstalled query = iota
	qOutdated
	qServices
	qTaps
	qDoctor
)

// resultMsg carries one query's result back to the App.
type resultMsg struct {
	q        query
	packages []brew.Package
	outdated []brew.Outdated
	services []brew.Service
	taps     []brew.Tap
	doctor   []brew.DoctorWarning
	err      error
}

func fetch(src Source, q query) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
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
