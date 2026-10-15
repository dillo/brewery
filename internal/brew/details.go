package brew

import (
	"context"
	"fmt"
	"strings"
)

// Details is what brew says about one package, installed or not: its info,
// and every formula it needs to run.
type Details struct {
	Package
	Needs []string // its dependencies at any depth, by name
}

// Details asks brew about one package, running its info and its
// dependencies at once.
func (c *Client) Details(ctx context.Context, name string, kind Kind) (Details, error) {
	flag := "--formula"
	if kind == KindCask {
		flag = "--cask"
	}
	type result struct {
		out []byte
		err error
	}
	deps := make(chan result, 1)
	go func() {
		out, err := c.query(ctx, "deps", flag, name)
		deps <- result{out, err}
	}()
	info, err := c.query(ctx, "info", "--json=v2", flag, name)
	d := <-deps
	if err != nil {
		return Details{}, err
	}
	if d.err != nil {
		return Details{}, d.err
	}
	pkgs, err := ParseInfo(info)
	if err != nil {
		return Details{}, err
	}
	if len(pkgs) == 0 {
		return Details{}, fmt.Errorf("brew has no %s named %s", kind, name)
	}
	return Details{Package: pkgs[0], Needs: strings.Fields(string(d.out))}, nil
}
