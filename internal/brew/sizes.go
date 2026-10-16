package brew

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
)

// PrefixOf is the Homebrew prefix for the brew executable at bin, such as
// /opt/homebrew for /opt/homebrew/bin/brew.
func PrefixOf(bin string) string { return filepath.Dir(filepath.Dir(bin)) }

// Sizes measures the disk space each package uses, keyed by Package.Key: a
// formula's kegs in the Cellar, or a cask's files (its app in /Applications,
// fonts…) and its Caskroom folder. It walks the files itself rather than
// asking brew, which has no such command. If it runs out of time, it
// returns the packages measured so far with the error.
func (c *Client) Sizes(ctx context.Context, pkgs []Package) (map[string]int64, error) {
	ctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	sizes := make(map[string]int64, len(pkgs))
	for _, p := range pkgs {
		paths := []string{filepath.Join(c.Prefix, "Cellar", p.Name)}
		if p.Kind == KindCask {
			paths = append([]string{filepath.Join(c.Prefix, "Caskroom", p.Name)}, p.Targets...)
		}
		var n int64
		for _, path := range paths {
			size, err := dirSize(ctx, path)
			if err != nil {
				return sizes, err // p is half measured: leave it out
			}
			n += size
		}
		sizes[p.Key()] = n
		if c.measured != nil {
			c.measured(p)
		}
	}
	return sizes, nil
}

// dirSize adds up the sizes of the files under path (or of path, if it is a
// file). Symlinks count as themselves, not what they point to. A missing
// path is 0; unreadable entries inside are skipped. It stops when ctx is
// done.
func dirSize(ctx context.Context, path string) (int64, error) {
	var n int64
	err := filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return nil // a folder we can't read: count what we can
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if info, err := d.Info(); err == nil && !d.IsDir() {
			n += info.Size()
		}
		return nil
	})
	return n, err
}
