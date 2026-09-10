package git

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/utils/ioutil"
)

// ignoreCaseEnabled checks if core.ignorecase is set to true in the repository configuration.
func (w *Worktree) ignoreCaseEnabled() bool {
	cfg, err := w.r.Config()
	if err != nil {
		return false
	}
	return cfg.Core.IgnoreCase
}

// matchPath compares two paths respecting core.ignorecase.
func (w *Worktree) matchPath(path1, path2 string) bool {
	if w.ignoreCaseEnabled() {
		return strings.EqualFold(path1, path2)
	}
	return path1 == path2
}

// resolveFilesystemPath handles case-insensitive file lookup in the worktree if core.ignorecase is active.
func (w *Worktree) resolveFilesystemPath(relPath string) (string, error) {
	fullPath := filepath.Join(w.Filesystem.Root(), relPath)
	if _, err := os.Stat(fullPath); err == nil {
		return relPath, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if !w.ignoreCaseEnabled() {
		return relPath, os.ErrNotExist
	}

	// Case-insensitive fallback lookup in parent directories
	dir, file := filepath.Split(relPath)
	entries, err := os.ReadDir(filepath.Join(w.Filesystem.Root(), dir))
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), file) {
			return filepath.Join(dir, entry.Name()), nil
		}
	}

	return relPath, os.ErrNotExist
}