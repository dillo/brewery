package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// server serves body for every request with status, recording the last
// request's path and user agent.
func server(t *testing.T, status int, body []byte, delay time.Duration) (*httptest.Server, *[2]string) {
	t.Helper()
	var got [2]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = [2]string{r.URL.Path, r.UserAgent()}
		time.Sleep(delay)
		w.WriteHeader(status)
		w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func TestFetchKeepsACopy(t *testing.T) {
	srv, got := server(t, http.StatusOK, fixture(t, "install-on-request-30d"), 0)
	s := NewStore(t.TempDir(), "1.2.3")
	s.Base = srv.URL + "/api/analytics/"
	l, err := s.Fetch(context.Background(), Tools30)
	if err != nil || len(l.Items) != 7 {
		t.Fatalf("got %d items, %v", len(l.Items), err)
	}
	if *got != [2]string{"/api/analytics/install-on-request/30d.json", "brewery/1.2.3"} {
		t.Errorf("asked for %q", *got)
	}
	cached, at, err := s.Cached(Tools30)
	if err != nil || len(cached.Items) != 7 || time.Since(at) > time.Minute {
		t.Errorf("cached copy: %d items from %v, %v", len(cached.Items), at, err)
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "install-on-request-30d.json")); err != nil {
		t.Error(err)
	}
}

func TestFetchFailureKeepsTheCache(t *testing.T) {
	dir := t.TempDir()
	good, _ := server(t, http.StatusOK, fixture(t, "cask-install-30d"), 0)
	s := NewStore(dir, "dev")
	s.Base = good.URL + "/"
	if _, err := s.Fetch(context.Background(), Apps30); err != nil {
		t.Fatal(err)
	}

	broken, _ := server(t, http.StatusOK, []byte("<html>oops</html>"), 0)
	missing, _ := server(t, http.StatusNotFound, nil, 0)
	slow, _ := server(t, http.StatusOK, fixture(t, "cask-install-30d"), 200*time.Millisecond)
	for name, base := range map[string]string{"broken": broken.URL, "missing": missing.URL, "slow": slow.URL} {
		s := NewStore(dir, "dev")
		s.Base = base + "/"
		s.Client.Timeout = 50 * time.Millisecond
		_, err := s.Fetch(context.Background(), Apps30)
		if err == nil {
			t.Errorf("%s: want an error", name)
		}
		var status *StatusError
		if name == "missing" && (!errors.As(err, &status) || status.Code != http.StatusNotFound || !strings.Contains(err.Error(), "404")) {
			t.Errorf("missing: %v should give the status", err)
		}
		if got := errors.Is(err, ErrBadList); got != (name == "broken") {
			t.Errorf("%s: errors.Is(%v, ErrBadList) = %v", name, err, got)
		}
		if l, _, err := s.Cached(Apps30); err != nil || len(l.Items) != 6 {
			t.Errorf("%s: the cache has %d items, %v", name, len(l.Items), err)
		}
	}
}

func TestCachedWithoutACopy(t *testing.T) {
	if _, _, err := NewStore(t.TempDir(), "dev").Cached(Tools90); err == nil {
		t.Error("want an error when nothing was fetched yet")
	}
}

func TestNewStore(t *testing.T) {
	s := NewStore("/x", "dev")
	if s.Base != BaseURL || s.Client.Timeout != 30*time.Second || s.UserAgent != "brewery/dev" {
		t.Errorf("got %+v", s)
	}
	if len(Lists) != 5 || Lists[0] != Tools30 || Lists[1] != Apps30 {
		t.Errorf("Lists = %v: the 30-day lists come first, for Popular", Lists)
	}
}

func TestTried(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "analytics"), "dev")
	if got := s.Tried(); got != "" {
		t.Errorf("nothing tried yet, got %q", got)
	}
	if err := s.SetTried("2026-10-16"); err != nil {
		t.Fatal(err)
	}
	if got := s.Tried(); got != "2026-10-16" {
		t.Errorf("got %q", got)
	}
}
