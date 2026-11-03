// Package github asks GitHub's REST API about repositories: how a tap's
// repository stands, and a package's releases and tags.
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

// ErrNotFound means GitHub has no such repository.
var ErrNotFound = errors.New("the repository is gone")

// Release is one of a repository's releases on GitHub.
type Release struct {
	Tag        string    `json:"tag_name"`
	Name       string    `json:"name"`
	Body       string    `json:"body"` // the notes, in GitHub's markdown
	URL        string    `json:"html_url"`
	Published  time.Time `json:"published_at"` // zero when GitHub gives none
	Prerelease bool      `json:"prerelease"`
}

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

// get asks GitHub for path, for the caller to read and close. A timeout or
// a failed connection comes back as its cause, without the request.
func (c *Client) get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "brewery")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		var ue *url.Error
		switch {
		case !errors.As(err, &ue):
			return nil, err
		case ue.Timeout():
			return nil, errors.New("timed out")
		}
		return nil, ue.Err // the cause, without the request it wraps
	}
	return resp, nil
}

// repoPath is the API's path for owner/repo.
func repoPath(owner, repo string) string {
	return "/repos/" + url.PathEscape(owner) + "/" + url.PathEscape(repo)
}

// Repo asks how owner/repo stands. A renamed or transferred repository
// redirects, which the HTTP client follows, so its name differs in the
// answer. The TapCheck's Tap is left for the caller to fill in.
func (c *Client) Repo(ctx context.Context, owner, repo string) (TapCheck, error) {
	resp, err := c.get(ctx, repoPath(owner, repo))
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
	case body.FullName == "":
		return TapCheck{State: Fine}, nil // no name to compare
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

// list reads the first page, of up to 100, of one of owner/repo's lists
// (releases or tags) into out.
func (c *Client) list(ctx context.Context, owner, repo, what string, out any) error {
	resp, err := c.get(ctx, repoPath(owner, repo)+"/"+what+"?per_page=100")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return ErrNotFound
	case rateLimited(resp):
		return ErrRateLimited
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("GitHub answered %s", resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("read GitHub's answer: %w", err)
	}
	return nil
}

// Releases asks for owner/repo's newest releases, newest first as GitHub
// lists them. Drafts aren't shown without a token.
func (c *Client) Releases(ctx context.Context, owner, repo string) ([]Release, error) {
	var rels []Release
	if err := c.list(ctx, owner, repo, "releases", &rels); err != nil {
		return nil, err
	}
	return rels, nil
}

// Tags asks for the names of owner/repo's newest tags.
func (c *Client) Tags(ctx context.Context, owner, repo string) ([]string, error) {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := c.list(ctx, owner, repo, "tags", &tags); err != nil {
		return nil, err
	}
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names, nil
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

// SourceRepo reads the owner and repository from an address on GitHub:
// https://github.com/<owner>/<repo> or https://codeload.github.com/<owner>/<repo>,
// followed by anything (an archive, a release download, a wiki). A
// trailing .git is dropped.
func SourceRepo(addr string) (owner, repo string, ok bool) {
	rest, found := strings.CutPrefix(addr, "https://github.com/")
	if !found {
		rest, found = strings.CutPrefix(addr, "https://codeload.github.com/")
	}
	if !found {
		return "", "", false
	}
	if i := strings.IndexAny(rest, "?#"); i >= 0 {
		rest = rest[:i]
	}
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 {
		return "", "", false
	}
	owner, repo = parts[0], strings.TrimSuffix(parts[1], ".git")
	if owner == "" || repo == "" {
		return "", "", false
	}
	return owner, repo, true
}
