package ui

// action is something the user asked brew to do from a pane.
type action struct {
	title   string   // shown in the drawer and status bar, e.g. "upgrade fd"
	args    []string // brew's arguments
	refresh []query  // queries to re-run when it finishes
	full    bool     // re-run every query, doctor included (after brew update)
}
