package main

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FileStatus represents the state of a file relative to the index.
type FileStatus int

const (
	// Unmodified means the file matches its index entry.
	Unmodified FileStatus = iota
	// Modified means the file content differs from its index entry.
	Modified
	// Untracked means the file exists on disk but has no index entry.
	Untracked
	// Deleted means the index entry has no matching file on disk.
	Deleted
)

// StatusEntry describes the state of a single file.
type StatusEntry struct {
	Staging  FileStatus
	Worktree FileStatus
}

// Status maps file paths (as recorded in the index) to their state.
type Status map[string]*StatusEntry

// IndexEntry records a tracked file path and the hash of the content the
// index expects for it. An empty Content means no content was recorded and
// the file is treated as unmodified when present on disk.
type IndexEntry struct {
	Path    string
	Content string
}

// CoreConfig mirrors git's core.* configuration section.
type CoreConfig struct {
	// IgnoreCase corresponds to core.ignorecase. Git automatically sets it
	// to true on case-insensitive filesystems (APFS, NTFS) and false on
	// case-sensitive filesystems (ext4).
	IgnoreCase bool
}

// Config is the repository configuration.
type Config struct {
	Core CoreConfig
}

// Repository is a minimal repository: it carries git configuration and the
// index of tracked files.
type Repository struct {
	config *Config
	index  []IndexEntry
}

// NewRepository returns a repository with the given configuration and index.
func NewRepository(cfg *Config, index []IndexEntry) *Repository {
	return &Repository{config: cfg, index: index}
}

// Config returns the repository configuration.
func (r *Repository) Config() (*Config, error) {
	if r.config == nil {
		return &Config{}, nil
	}
	return r.config, nil
}

// Worktree represents a working tree bound to a repository and a root
// directory on disk.
type Worktree struct {
	r        *Repository
	rootPath string
}

// NewWorktree returns a Worktree rooted at rootPath.
func NewWorktree(r *Repository, rootPath string) *Worktree {
	return &Worktree{r: r, rootPath: rootPath}
}

// isIgnoreCase reports whether the repository is configured for a
// case-insensitive filesystem (core.ignorecase = true).
func (w *Worktree) isIgnoreCase() bool {
	cfg, err := w.r.Config()
	if err != nil {
		return false
	}
	return cfg.Core.IgnoreCase
}

// pathsMatch reports whether two paths refer to the same file, honouring
// the core.ignorecase configuration: on case-insensitive filesystems the
// comparison is case-folded, on case-sensitive filesystems it is exact.
func (w *Worktree) pathsMatch(a, b string) bool {
	if w.isIgnoreCase() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// Status compares the working tree against the repository index and returns
// the status of every file.
//
// On case-insensitive filesystems (core.ignorecase = true), an index path
// such as "README.md" is matched against the on-disk name "Readme.md": the
// case-only difference is ignored, so the file is not falsely reported as
// modified, untracked, or deleted. On case-sensitive filesystems, names
// that differ only in case remain distinct, matching git's behaviour.
func (w *Worktree) Status() (Status, error) {
	result := make(Status)
	if w.r == nil {
		return result, nil
	}

	ignoreCase := w.isIgnoreCase()

	// Index lookup maps. On case-insensitive filesystems we additionally
	// index the lowercased paths so a case-folding lookup is O(1).
	exact := make(map[string]IndexEntry, len(w.r.index))
	folded := make(map[string]IndexEntry, len(w.r.index))
	for _, e := range w.r.index {
		exact[e.Path] = e
		folded[strings.ToLower(e.Path)] = e
	}

	// Index paths that were found on disk.
	seen := make(map[string]bool, len(w.r.index))

	err := filepath.WalkDir(w.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != w.rootPath && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(w.rootPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)

		var (
			entry IndexEntry
			ok    bool
		)
		if ignoreCase {
			entry, ok = folded[strings.ToLower(rel)]
		} else {
			entry, ok = exact[rel]
		}
		if !ok {
			// Not tracked: the on-disk file is untracked.
			result[rel] = &StatusEntry{Staging: Untracked, Worktree: Untracked}
			return nil
		}

		seen[entry.Path] = true

		if entry.Content != "" && contentHash(path) != entry.Content {
			// Content differs from what the index records.
			result[entry.Path] = &StatusEntry{Staging: Unmodified, Worktree: Modified}
			return nil
		}

		result[entry.Path] = &StatusEntry{Staging: Unmodified, Worktree: Unmodified}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Index entries with no matching file on disk are deleted.
	for _, e := range w.r.index {
		if !seen[e.Path] {
			result[e.Path] = &StatusEntry{Staging: Deleted, Worktree: Deleted}
		}
	}

	return result, nil
}

// hashContent returns the SHA-256 hex digest of b.
func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// contentHash returns the SHA-256 hex digest of the file at path, or "" if
// the file cannot be read.
func contentHash(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return hashContent(b)
}

// IsClean reports whether the working tree matches the index exactly.
func (s Status) IsClean() bool {
	for _, e := range s {
		if e.Staging != Unmodified || e.Worktree != Unmodified {
			return false
		}
	}
	return true
}
