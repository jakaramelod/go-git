package git

import (
	"os"
	"path/filepath"
	"strings"
)

// Config holds repository settings read from .git/config (core.ignorecase).
type Config struct {
	Core struct {
		IgnoreCase bool
	}
}

// Repository is the owning repo for a Worktree.
type Repository struct {
	config Config
}

// Config returns the repository configuration.
func (r *Repository) Config() (Config, error) {
	return r.config, nil
}

// SetIgnoreCase sets core.ignorecase.
func (r *Repository) SetIgnoreCase(v bool) {
	r.config.Core.IgnoreCase = v
}

// IndexEntry is one cached path in the index.
type IndexEntry struct {
	Path    string
	Content string
}

// Worktree is a checkout plus its index.
type Worktree struct {
	r     *Repository
	root  string
	index []IndexEntry
}

// NewWorktree builds a worktree rooted at root.
func NewWorktree(root string, ignoreCase bool) *Worktree {
	repo := &Repository{}
	repo.SetIgnoreCase(ignoreCase)
	return &Worktree{r: repo, root: root}
}

// SetIndex replaces the index snapshot used by Status.
func (w *Worktree) SetIndex(entries []IndexEntry) {
	w.index = append([]IndexEntry(nil), entries...)
}

func (w *Worktree) isIgnoreCase() bool {
	if w.r == nil {
		return false
	}
	cfg, err := w.r.Config()
	if err != nil {
		return false
	}
	return cfg.Core.IgnoreCase
}

func (w *Worktree) pathsMatch(a, b string) bool {
	if w.isIgnoreCase() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (w *Worktree) findIndex(path string) (IndexEntry, bool) {
	for _, e := range w.index {
		if w.pathsMatch(e.Path, path) {
			return e, true
		}
	}
	return IndexEntry{}, false
}

func (w *Worktree) findDisk(files map[string]string, indexPath string) (diskPath, content string, ok bool) {
	if c, hit := files[indexPath]; hit {
		return indexPath, c, true
	}
	if !w.isIgnoreCase() {
		return "", "", false
	}
	for p, c := range files {
		if strings.EqualFold(p, indexPath) {
			return p, c, true
		}
	}
	return "", "", false
}

func listWorktreeFiles(root string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[rel] = string(b)
		return nil
	})
	return out, err
}
