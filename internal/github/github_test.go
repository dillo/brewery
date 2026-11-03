package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// fakeGitHub answers like GitHub's /repos endpoint for a few repositories,
// and records the headers of the first request to /repos/o/fine.
func fakeGitHub(t *testing.T, got *http.Header) *Client {
	t.Helper()
	mux := http.NewServeMux()
	answer := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/o/fine" && *got == nil {
				*got = r.Header.Clone()
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}
	answer("/repos/o/fine", `{"full_name": "o/fine", "archived": false}`)
	answer("/repos/o/nameless", `{"full_name": "", "archived": false}`)
	answer("/repos/o/badjson", `{"full_name": `)
	answer("/repos/o/old", `{"full_name": "o/old", "archived": true}`)
	answer("/repositories/42", `{"full_name": "n/new", "archived": false}`)
	mux.HandleFunc("/repos/o/renamed", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/repositories/42", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/repos/o/limited", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	})
	mux.HandleFunc("/repos/o/throttled", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	mux.HandleFunc("/repos/o/forbidden", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "5")
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
		{"nameless", TapCheck{State: Fine}},
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
	if _, err := c.Repo(context.Background(), "o", "throttled"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 429 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Repo(context.Background(), "o", "forbidden"); err == nil || errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with requests left is an error, not the rate limit: %v", err)
	}
	if _, err := c.Repo(context.Background(), "o", "badjson"); err == nil {
		t.Error("a 200 with a bad body should be an error")
	}
	if _, err := c.Repo(context.Background(), "o", "broken"); err == nil || errors.Is(err, ErrRateLimited) {
		t.Errorf("a 500 is an error, not the rate limit: %v", err)
	}
}

func TestRepoTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(300 * time.Millisecond)
	}))
	t.Cleanup(srv.Close)
	c := &Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: 20 * time.Millisecond}}
	_, err := c.Repo(context.Background(), "o", "slow")
	if err == nil || err.Error() != "timed out" {
		t.Errorf("a timeout reads %v, want timed out", err)
	}
}

func TestRepoTransportFailure(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	c := &Client{BaseURL: srv.URL, HTTP: &http.Client{Timeout: time.Second}}
	srv.Close()
	_, err := c.Repo(context.Background(), "o", "r")
	if err == nil || strings.HasPrefix(err.Error(), `Get "`) {
		t.Errorf("the cause alone should show, not the request: %v", err)
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
		{"https://github.com/o/r/", "o", "r", true},
		{"https://github.com/o/r/x", "", "", false},
		{"", "", "", false},
	} {
		owner, repo, ok := RepoOf(tt.remote)
		if owner != tt.owner || repo != tt.repo || ok != tt.ok {
			t.Errorf("%q: %q %q %v; want %q %q %v", tt.remote, owner, repo, ok, tt.owner, tt.repo, tt.ok)
		}
	}
}

// fakeReleases answers like GitHub's releases and tags endpoints, and
// records the query of the first request for o/r's releases.
func fakeReleases(t *testing.T, query *string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	answer := func(path, body string) {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/repos/o/r/releases" && *query == "" {
				*query = r.URL.RawQuery
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		})
	}
	answer("/repos/o/r/releases", `[
	  {"tag_name": "v2", "name": "Two", "body": "notes", "html_url": "https://github.com/o/r/releases/tag/v2",
	   "published_at": "2026-10-01T12:00:00Z", "prerelease": false},
	  {"tag_name": "v3-rc1", "name": "", "body": "", "html_url": "", "published_at": null, "prerelease": true}
	]`)
	answer("/repos/o/r/tags", `[{"name": "v2"}, {"name": "v1"}]`)
	answer("/repos/o/empty/releases", `[]`)
	answer("/repos/o/badjson/releases", `[{`)
	for _, path := range []string{"/repos/o/limited/releases", "/repos/o/limited/tags"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.WriteHeader(http.StatusForbidden)
		})
	}
	mux.HandleFunc("/repos/o/broken/releases", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	// Anything else, /repos/o/gone/releases included, is a 404.
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := NewClient()
	c.BaseURL = srv.URL
	return c
}

func TestReleases(t *testing.T) {
	var query string
	c := fakeReleases(t, &query)
	ctx := context.Background()
	got, err := c.Releases(ctx, "o", "r")
	want := []Release{
		{Tag: "v2", Name: "Two", Body: "notes", URL: "https://github.com/o/r/releases/tag/v2",
			Published: time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)},
		{Tag: "v3-rc1", Prerelease: true},
	}
	if err != nil || len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("releases %+v, %v; want %+v", got, err, want)
	}
	if query != "per_page=100" {
		t.Errorf("query %q, want per_page=100", query)
	}
	if got, err := c.Releases(ctx, "o", "empty"); err != nil || len(got) != 0 {
		t.Errorf("no releases: %+v, %v", got, err)
	}
	if _, err := c.Releases(ctx, "o", "gone"); !errors.Is(err, ErrNotFound) || err.Error() != "the repository is gone" {
		t.Errorf("a 404 is ErrNotFound: %v", err)
	}
	if _, err := c.Releases(ctx, "o", "limited"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Releases(ctx, "o", "broken"); err == nil || errors.Is(err, ErrRateLimited) || errors.Is(err, ErrNotFound) {
		t.Errorf("a 500 is an error of its own: %v", err)
	}
	if _, err := c.Releases(ctx, "o", "badjson"); err == nil {
		t.Error("a 200 with a bad body should be an error")
	}
}

func TestTags(t *testing.T) {
	var query string
	c := fakeReleases(t, &query)
	ctx := context.Background()
	if got, err := c.Tags(ctx, "o", "r"); err != nil || strings.Join(got, " ") != "v2 v1" {
		t.Errorf("tags %v, %v; want v2 v1", got, err)
	}
	if _, err := c.Tags(ctx, "o", "limited"); !errors.Is(err, ErrRateLimited) {
		t.Errorf("a 403 with no requests left is the rate limit: %v", err)
	}
	if _, err := c.Tags(ctx, "o", "gone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a 404 is ErrNotFound: %v", err)
	}
}

func TestSourceRepo(t *testing.T) {
	for _, tt := range []struct {
		addr, owner, repo string
		ok                bool
	}{
		{"https://github.com/cli/cli/archive/refs/tags/v2.104.0.tar.gz", "cli", "cli", true},
		{"https://github.com/cli/cli/releases/download/v2.104.0/gh.zip", "cli", "cli", true},
		{"https://codeload.github.com/o/r/tar.gz/refs/tags/v1", "o", "r", true},
		{"https://github.com/o/r.git", "o", "r", true},
		{"https://github.com/sharkdp/fd", "sharkdp", "fd", true},
		{"https://github.com/json-c/json-c/wiki", "json-c", "json-c", true},
		{"https://github.com/o/r?tab=readme", "o", "r", true},
		{"https://cli.github.com/", "", "", false},
		{"https://gitlab.com/o/r", "", "", false},
		{"http://github.com/o/r", "", "", false},
		{"https://github.com/o", "", "", false},
		{"https://github.com/o/", "", "", false},
		{"", "", "", false},
	} {
		owner, repo, ok := SourceRepo(tt.addr)
		if owner != tt.owner || repo != tt.repo || ok != tt.ok {
			t.Errorf("%q: %q %q %v; want %q %q %v", tt.addr, owner, repo, ok, tt.owner, tt.repo, tt.ok)
		}
	}
}
