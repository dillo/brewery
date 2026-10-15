//go:build live

package analytics

import (
	"context"
	"testing"
)

// Downloads one real list from formulae.brew.sh, to catch format drift.
// Run with: go test -tags live ./internal/analytics/
func TestLiveFetch(t *testing.T) {
	s := NewStore(t.TempDir(), "live-test")
	l, err := s.Fetch(context.Background(), Apps30)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Items) < 1000 || l.Items[0].Rank != 1 || l.Items[0].Count < 1000 || l.End == "" {
		t.Errorf("got %d items from %s to %s, first %+v", len(l.Items), l.Start, l.End, l.Items[0])
	}
	if _, _, err := s.Cached(Apps30); err != nil {
		t.Error(err)
	}
}
