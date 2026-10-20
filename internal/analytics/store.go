package analytics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The lists Discover reads, named by their paths on formulae.brew.sh:
// tools are formulae people asked for (not ones pulled in as dependencies),
// apps are casks.
const (
	Tools30  = "install-on-request/30d"
	Tools90  = "install-on-request/90d"
	Tools365 = "install-on-request/365d"
	Apps30   = "cask-install/30d"
	Apps90   = "cask-install/90d"
)

// Lists is every list, in the order they're fetched: Popular's first.
var Lists = []string{Tools30, Apps30, Tools90, Apps90, Tools365}

// BaseURL is where Homebrew publishes its analytics.
const BaseURL = "https://formulae.brew.sh/api/analytics/"

// maxSize bounds a download; the largest list is about 4MB.
const maxSize = 64 << 20

// StatusError is a list formulae.brew.sh answered with a status other
// than 200 OK.
type StatusError struct {
	List string
	Code int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("%s: %d %s", e.List, e.Code, http.StatusText(e.Code))
}

// ErrBadList is a list that came back but isn't one: an error page, or
// JSON in another shape.
var ErrBadList = errors.New("not an analytics list")

// Store downloads analytics lists and keeps the last copy of each in Dir.
type Store struct {
	Dir       string
	Client    *http.Client
	UserAgent string
	Base      string // BaseURL; tests point it at a local server
}

// NewStore keeps copies in dir and names brewery's version in each request.
// A download gives up after 30 seconds.
func NewStore(dir, version string) Store {
	return Store{Dir: dir, Client: &http.Client{Timeout: 30 * time.Second}, UserAgent: "brewery/" + version, Base: BaseURL}
}

// path is where the copy of list is kept.
func (s Store) path(list string) string {
	return filepath.Join(s.Dir, strings.ReplaceAll(list, "/", "-")+".json")
}

// Cached is the last copy of list, and when it was fetched.
func (s Store) Cached(list string) (List, time.Time, error) {
	path := s.path(list)
	b, err := os.ReadFile(path) // #nosec G304 -- our own cache dir; path() strips "/" from list
	if err != nil {
		return List{}, time.Time{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return List{}, time.Time{}, err
	}
	l, err := Parse(b)
	return l, info.ModTime(), err
}

// Fetch downloads list and keeps a copy. A failed download leaves the
// copy as it was.
func (s Store) Fetch(ctx context.Context, list string) (List, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.Base+list+".json", nil)
	if err != nil {
		return List{}, err
	}
	req.Header.Set("User-Agent", s.UserAgent)
	resp, err := s.Client.Do(req)
	if err != nil {
		return List{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return List{}, &StatusError{List: list, Code: resp.StatusCode}
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxSize))
	if err != nil {
		return List{}, err
	}
	l, err := Parse(b)
	if err != nil {
		return List{}, fmt.Errorf("%s: %w: %w", list, ErrBadList, err)
	}
	_ = s.save(list, b) // without a copy, the next start only shows the lists once they're fetched again
	return l, nil
}

// triedFile holds the day a fetch last started.
const triedFile = "tried"

// Tried is the day a fetch last started, as 2006-01-02, or "" if none
// is known.
func (s Store) Tried() string {
	b, err := os.ReadFile(filepath.Join(s.Dir, triedFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// SetTried records day (2006-01-02) as the day a fetch last started, so a
// fetch that failed isn't tried again by every start that day.
func (s Store) SetTried(day string) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.Dir, triedFile), []byte(day+"\n"), 0o600)
}

// save writes the copy of list atomically, so a crash midway never leaves
// half a file.
func (s Store) save(list string, b []byte) error {
	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(s.Dir, ".fetch-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // fails harmlessly once renamed
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), s.path(list))
}
