package brew

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestParseVulns(t *testing.T) {
	scan, err := ParseVulns(fixture(t, "vulns.json"))
	if err != nil {
		t.Fatal(err)
	}
	if scan.Checked != 4 || !slices.Equal(scan.Skipped, []string{"fd"}) {
		t.Errorf("checked %d, skipped %v; want 4 and [fd]", scan.Checked, scan.Skipped)
	}
	var names []string
	for _, f := range scan.Findings {
		names = append(names, f.Formula+" "+f.Version)
	}
	if want := []string{"git 2.50.0", "json-c 0.19", "lua-language-server 3.19.1"}; !slices.Equal(names, want) {
		t.Errorf("findings %v, want %v: pcre2 has only patched ones", names, want)
	}
	want := Vuln{ID: "OSV-2024-512", Severity: "MEDIUM", Summary: "Integer overflow in printbuf",
		Fixed: []string{"0.20", "0f1e2d3c4b5a69788796a5b4c3d2e1f00f1e2d3c"}}
	if jc := scan.Findings[1]; len(jc.Vulns) != 3 || !reflect.DeepEqual(jc.Vulns[1], want) {
		t.Errorf("json-c's findings: %+v", jc.Vulns)
	}
	if git := scan.Findings[0]; len(git.Vulns) != 1 || git.Vulns[0].ID != "CVE-2025-48386" {
		t.Errorf("git keeps only its open finding, not the patched one: %+v", git.Vulns)
	}
}

func TestParseVulnsBadJSON(t *testing.T) {
	if _, err := ParseVulns([]byte("not json")); err == nil {
		t.Error("want an error for output that isn't JSON")
	}
}

func TestClientVulns(t *testing.T) {
	exitErr := exec.Command("false").Run() // a real *exec.ExitError with status 1
	const key = "vulns --json"
	ctx := context.Background()

	// brew vulns exits 1 whenever it finds anything.
	c := NewClient(fakeRunner{out: map[string][]byte{key: fixture(t, "vulns.json")}, err: map[string]error{key: exitErr}})
	if scan, err := c.Vulns(ctx); err != nil || len(scan.Findings) != 3 {
		t.Errorf("exit 1 with JSON: %d findings, %v; want 3 and no error", len(scan.Findings), err)
	}
	c = NewClient(fakeRunner{out: map[string][]byte{key: fixture(t, "vulns.json")}})
	if scan, err := c.Vulns(ctx); err != nil || scan.Checked != 4 {
		t.Errorf("exit 0: %+v, %v", scan, err)
	}

	c = NewClient(fakeRunner{err: map[string]error{key: exitErr}})
	if _, err := c.Vulns(ctx); err == nil || !strings.Contains(err.Error(), "something broke") {
		t.Errorf("exit 1 without JSON should be an error with brew's stderr: %v", err)
	} else if _, ok := errors.AsType[*QueryError](err); !ok {
		t.Errorf("want *QueryError, got %T", err)
	}

	c = NewClient(fakeRunner{err: map[string]error{key: exitErr},
		stderr: map[string]string{key: "Error: Invalid usage: Unknown command: brew vulns"}})
	if _, err := c.Vulns(ctx); !errors.Is(err, ErrNoVulns) {
		t.Errorf("a brew without vulns: %v, want ErrNoVulns", err)
	}

	c = NewClient(fakeRunner{out: map[string][]byte{key: fixture(t, "vulns.json")}, err: map[string]error{key: errors.New("fork failed")}})
	if _, err := c.Vulns(ctx); err == nil {
		t.Error("brew not running at all is an error, whatever it printed")
	}

	c = NewClient(fakeRunner{delay: time.Second})
	c.Timeout = 10 * time.Millisecond
	if _, err := c.Vulns(ctx); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("want a timeout, got %v", err)
	}
}
