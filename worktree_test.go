package git

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// isIgnoreCaseEnabled checks if core.ignorecase is set to true in the repository config.
func (w *Worktree) isIgnoreCaseEnabled() bool {
	cfg, err := w.r.Config()
	if err != nil {
		return false
	}
	return cfg.Core.IgnoreCase
}

// resolveWorktreePath checks filesystem case-insensitively if ignorecase is enabled.
func (w *Worktree) resolveWorktreePath(relPath string, ignoreCase bool) (string, bool) {
	fullPath := filepath.Join(w.Filesystem.Root(), relPath)
	if _, err := os.Stat(fullPath); err == nil {
		return relPath, true
	}

	if !ignoreCase {
		return relPath, false
	}

	// Case-insensitive fallback traversal
	dir, file := filepath.Split(relPath)
	dirPath := filepath.Join(w.Filesystem.Root(), dir)
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return relPath, false
	}

	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), file) {
			return filepath.Join(dir, entry.Name()), true
		}
	}

	return relPath, false
}