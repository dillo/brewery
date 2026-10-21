package brew

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoVulns means this brew has no vulns command: it's older than the
// scanner.
var ErrNoVulns = errors.New("brew has no vulns command")

// VulnScan is what `brew vulns --json` found in the installed formulae.
type VulnScan struct {
	Findings []VulnFinding `json:"findings"` // formulae with open findings
	Checked  int           `json:"checked"`  // how many formulae were looked up
	Skipped  []string      `json:"skipped"`  // ones brew couldn't scan
}

// VulnFinding is one formula's open findings, at the version scanned.
type VulnFinding struct {
	Formula string `json:"formula"`
	Version string `json:"version"` // as brew vulns gives it: without brew's revision
	Vulns   []Vuln `json:"vulns"`
}

// Vuln is one known vulnerability, as OSV.dev records it.
type Vuln struct {
	ID       string   `json:"id"`
	Severity string   `json:"severity"` // CRITICAL, HIGH, MEDIUM, LOW or UNKNOWN, as brew gives it
	Summary  string   `json:"summary"`
	Fixed    []string `json:"fixed"` // versions or commit hashes that fix it
}

type vulnsJSON struct {
	Findings []struct {
		Formula         string     `json:"formula"`
		Version         string     `json:"version"`
		Vulnerabilities []vulnJSON `json:"vulnerabilities"`
	} `json:"findings"`
	Checked int      `json:"checked"`
	Skipped []string `json:"skipped_formulae"`
}

type vulnJSON struct {
	ID            string   `json:"id"`
	Severity      string   `json:"severity"`
	Summary       string   `json:"summary"`
	FixedVersions []string `json:"fixed_versions"`
}

// ParseVulns parses `brew vulns --json`. Findings that one of brew's
// patches already fixes (its "patched" list) aren't kept, nor are formulae
// left with no open findings.
func ParseVulns(b []byte) (VulnScan, error) {
	var in vulnsJSON
	if err := json.Unmarshal(jsonPayload(b), &in); err != nil {
		return VulnScan{}, fmt.Errorf("parse brew vulns: %w", err)
	}
	scan := VulnScan{Checked: in.Checked, Skipped: in.Skipped}
	for _, f := range in.Findings {
		if len(f.Vulnerabilities) == 0 {
			continue
		}
		vf := VulnFinding{Formula: f.Formula, Version: f.Version}
		for _, v := range f.Vulnerabilities {
			vf.Vulns = append(vf.Vulns, Vuln{ID: v.ID, Severity: v.Severity, Summary: v.Summary, Fixed: v.FixedVersions})
		}
		scan.Findings = append(scan.Findings, vf)
	}
	return scan, nil
}

// Vulns runs `brew vulns --json`, which looks the installed formulae up on
// OSV.dev. It exits 1 whenever it finds anything, so a non-zero exit is a
// result as long as its output parses. A brew without the command gives
// ErrNoVulns.
func (c *Client) Vulns(ctx context.Context) (VulnScan, error) {
	args := []string{"vulns", "--json"}
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	out, errb, err := c.r.Run(ctx, args...)
	if err != nil && ctx.Err() != nil {
		return VulnScan{}, &QueryError{Args: args, Stderr: string(errb), Err: fmt.Errorf("timed out after %s", c.Timeout)}
	}
	if strings.Contains(string(errb), "Unknown command") {
		return VulnScan{}, ErrNoVulns
	}
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return VulnScan{}, &QueryError{Args: args, Stderr: string(errb), Err: err}
	}
	scan, perr := ParseVulns(out)
	if perr != nil {
		if err == nil {
			err = perr
		}
		return VulnScan{}, &QueryError{Args: args, Stderr: string(errb), Err: err}
	}
	return scan, nil
}
