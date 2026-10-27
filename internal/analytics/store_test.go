package analytics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
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
		_, _ = w.Write(body)
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

// unwritable is a path that can't be made into a directory: its parent
// is a regular file.
func unwritable(t *testing.T) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(file, "analytics")
}

func TestFetchWithoutAPlaceToKeepACopy(t *testing.T) {
	srv, _ := server(t, http.StatusOK, fixture(t, "install-on-request-30d"), 0)
	dirs := map[string]string{"under a file": unwritable(t)}
	if os.Geteuid() == 0 {
		t.Log("skipping the read-only case: root can write to a read-only directory")
	} else {
		readOnly := t.TempDir()
		if err := os.Chmod(readOnly, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(readOnly, 0o700) })
		dirs["read-only"] = readOnly
	}
	for name, dir := range dirs {
		s := NewStore(dir, "dev")
		s.Base = srv.URL + "/"
		// The list still comes back: only the copy for next time is lost.
		if l, err := s.Fetch(context.Background(), Tools30); err != nil || len(l.Items) != 7 {
			t.Errorf("%s: got %d items, %v", name, len(l.Items), err)
		}
		if _, _, err := s.Cached(Tools30); err == nil {
			t.Errorf("%s: no copy could be kept, yet Cached found one", name)
		}
	}
}

func TestFetchCutShort(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		_, _ = w.Write([]byte(`{"items": [`)) // and the connection closes
	}))
	t.Cleanup(srv.Close)
	s := NewStore(t.TempDir(), "dev")
	s.Base = srv.URL + "/"
	if _, err := s.Fetch(context.Background(), Tools30); err == nil || errors.Is(err, ErrBadList) {
		t.Errorf("got %v, want the download's error", err)
	}
	if _, _, err := s.Cached(Tools30); err == nil {
		t.Error("half a download must not be kept")
	}
}

func TestFetchFromABadBase(t *testing.T) {
	s := NewStore(t.TempDir(), "dev")
	s.Base = "http://\x7f/"
	var urlErr *url.Error
	if _, err := s.Fetch(context.Background(), Tools30); !errors.As(err, &urlErr) {
		t.Errorf("got %v, want a *url.Error", err)
	}
}

func TestSetTriedFails(t *testing.T) {
	s := NewStore(unwritable(t), "dev")
	if err := s.SetTried("2026-10-16"); err == nil {
		t.Error("want an error when the dir can't be made")
	}
	if got := s.Tried(); got != "" {
		t.Errorf("nothing was recorded, got %q", got)
	}
}

func TestSetTriedReplacesASymlink(t *testing.T) {
	s := NewStore(t.TempDir(), "dev")
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(s.Dir, triedFile)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTried("2026-10-16"); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(victim); string(b) != "original" {
		t.Errorf("SetTried wrote through the symlink: victim holds %q", b)
	}
	if got := s.Tried(); got != "2026-10-16" {
		t.Errorf("got %q", got)
	}
}

func TestStoreWithoutADirKeepsNothing(t *testing.T) {
	srv, _ := server(t, http.StatusOK, fixture(t, "install-on-request-30d"), 0)
	t.Chdir(t.TempDir()) // a relative path would land here
	s := NewStore("", "dev")
	s.Base = srv.URL + "/"
	if _, err := s.Fetch(context.Background(), Tools30); err != nil {
		t.Fatal(err)
	}
	if err := s.SetTried("2026-10-16"); err != nil {
		t.Fatal(err)
	}
	if left, _ := os.ReadDir("."); len(left) != 0 {
		t.Errorf("a store without a dir wrote %v", left)
	}
	if _, _, err := s.Cached(Tools30); err == nil {
		t.Error("a store without a dir has no copies")
	}
}

func TestFilesArePrivate(t *testing.T) {
	s := NewStore(filepath.Join(t.TempDir(), "analytics"), "dev")
	if err := s.SetTried("2026-10-16"); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(s.Dir); err != nil || fi.Mode().Perm() != 0o700 {
		t.Errorf("dir mode = %v, %v; want 0700", fi.Mode().Perm(), err)
	}
	if fi, err := os.Stat(filepath.Join(s.Dir, triedFile)); err != nil || fi.Mode().Perm() != 0o600 {
		t.Errorf("file mode = %v, %v; want 0600", fi.Mode().Perm(), err)
	}
}
