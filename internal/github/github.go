// Package github asks GitHub's REST API how a tap's repository stands:
// still there as it was, archived, moved or gone.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// State is how a tap's repository stands on GitHub.
type State string

// The states a check can find. Unknown means it couldn't check; the
// TapCheck's Reason says why.
const (
	Fine     State = "fine"
	Archived State = "archived"
	Moved    State = "moved"
	Gone     State = "gone"
	Unknown  State = "unknown"
)

// TapCheck is what checking one tap's repository found.
type TapCheck struct {
	Tap     string `json:"tap"`
	State   State  `json:"state"`
	MovedTo string `json:"moved_to,omitempty"` // the repository's owner/name now, when moved
	Reason  string `json:"reason,omitempty"`   // why it couldn't be checked, when unknown
}

// ErrRateLimited means GitHub won't answer more requests for now.
var ErrRateLimited = errors.New("rate limit reached")

// Client asks GitHub's REST API about repositories, without a token.
type Client struct {
	BaseURL string // https://api.github.com, or a test server
	HTTP    *http.Client
}

// NewClient is a Client for GitHub itself, waiting at most 10 seconds for
// each answer.
func NewClient() *Client {
	return &Client{BaseURL: "https://api.github.com", HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Repo asks how owner/repo stands. A renamed or transferred repository
// redirects, which the HTTP client follows, so its name differs in the
// answer. The TapCheck's Tap is left for the caller to fill in.
func (c *Client) Repo(ctx context.Context, owner, repo string) (TapCheck, error) {
	u := c.BaseURL + "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return TapCheck{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "brewery")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return TapCheck{}, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return TapCheck{State: Gone}, nil
	case rateLimited(resp):
		return TapCheck{}, ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return TapCheck{}, fmt.Errorf("GitHub answered %s", resp.Status)
	}
	var body struct {
		FullName string `json:"full_name"`
		Archived bool   `json:"archived"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return TapCheck{}, fmt.Errorf("read GitHub's answer: %w", err)
	}
	switch {
	case body.Archived:
		return TapCheck{State: Archived}, nil
	case !strings.EqualFold(body.FullName, owner+"/"+repo):
		return TapCheck{State: Moved, MovedTo: body.FullName}, nil
	}
	return TapCheck{State: Fine}, nil
}

// rateLimited reports whether resp refuses for want of requests left.
func rateLimited(resp *http.Response) bool {
	refused := resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests
	return refused && resp.Header.Get("X-RateLimit-Remaining") == "0"
}

// RepoOf reads the owner and repository from a tap's remote when it's on
// GitHub: https://github.com/<owner>/<repo>, with or without .git.
func RepoOf(remote string) (owner, repo string, ok bool) {
	rest, found := strings.CutPrefix(remote, "https://github.com/")
	if !found {
		return "", "", false
	}
	rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
	owner, repo, ok = strings.Cut(rest, "/")
	if !ok || owner == "" || repo == "" || strings.Contains(repo, "/") {
		return "", "", false
	}
	return owner, repo, true
}
