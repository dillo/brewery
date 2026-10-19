package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// fakeGitHub answers like GitHub's /repos endpoint for a few repositories,
// and records the last request's headers.
func fakeGitHub(t *testing.T, got *http.Header) *Client {
	t.Helper()
	mux := http.NewServeMux()
	answer := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			*got = r.Header.Clone()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}
	answer("/repos/o/fine", `{"full_name": "o/fine", "archived": false}`)
	answer("/repos/o/old", `{"full_name": "o/old", "archived": true}`)
	answer("/repositories/42", `{"full_name": "n/new", "archived": false}`)
	mux.HandleFunc("/repos/o/renamed", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/repositories/42", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/repos/o/limited", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/repos/o/broken", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	// Anything else, /repos/o/gone included, is a 404.
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.BaseURL = srv.URL
	return c
}

func TestRepo(t *testing.T) {
	var hdr http.Header
	c := fakeGitHub(t, &hdr)
	for _, tt := range []struct {
		repo string
		want TapCheck
	}{
		{"fine", TapCheck{State: Fine}},
		{"old", TapCheck{State: Archived}},
		{"renamed", TapCheck{State: Moved, MovedTo: "n/new"}},
		{"gone", TapCheck{State: Gone}},
	} {
		got, err := c.Repo(context.Background(), "o", tt.repo)
		if err != nil || got != tt.want {
			t.Errorf("%s: %+v, %v; want %+v", tt.repo, got, err, tt.want)
		}
	}
	if hdr.Get("Accept") != "application/vnd.github+json" || hdr.Get("User-Agent") != "brewery" {
		t.Errorf("headers %v", hdr)
	}
	if _, err := c.Repo(context.Background(), "o", "limited"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Repo(context.Background(), "o", "broken"); err == nil || errors.Is(err, ErrRateLimited) {
		t.Errorf("a 500 is an error, not the rate limit: %v", err)
	}
}

func TestRepoOf(t *testing.T) {
	for _, tt := range []struct {
		remote, owner, repo string
		ok                  bool
	}{
		{"https://github.com/anomalyco/homebrew-tap.git", "anomalyco", "homebrew-tap", true},
		{"https://github.com/anomalyco/homebrew-tap", "anomalyco", "homebrew-tap", true},
		{"http://github.com/anomalyco/homebrew-tap", "", "", false},
		{"https://gitlab.com/someone/homebrew-tap.git", "", "", false},
		{"https://github.com/anomalyco", "", "", false},
		{"", "", "", false},
	} {
		owner, repo, ok := RepoOf(tt.remote)
		if owner != tt.owner || repo != tt.repo || ok != tt.ok {
			t.Errorf("%q: %q %q %v; want %q %q %v", tt.remote, owner, repo, ok, tt.owner, tt.repo, tt.ok)
		}
	}
}
