package worktree

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/go-git/go-git/v5/storage"
)

// isIgnoreCaseEnabled checks whether core.ignorecase is set in the git configuration.
func isIgnoreCaseEnabled(s storage.Storer) bool {
	cfg, err := s.Config()
	if err != nil {
		return false
	}
	return cfg.Core.IgnoreCase
}

// matchPathCaseSensitiveOrFold compares two paths respecting core.ignorecase.
func matchPathCaseSensitiveOrFold(path1, path2 string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(path1, path2)
	}
	return path1 == path2
}