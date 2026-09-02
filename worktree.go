package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// StatusCode represents the status of a file in the worktree or index.
type StatusCode rune

const (
	Unmodified StatusCode = ' '
	Modified   StatusCode = 'M'
	Added      StatusCode = 'A'
	Deleted    StatusCode = 'D'
	Untracked  StatusCode = '?'
)

// FileStatus represents the status of an individual file.
type FileStatus struct {
	Worktree StatusCode
	Staging  StatusCode
	Extra    string
}

// Status represents the full status of all files in the worktree.
type Status map[string]*FileStatus

// IsClean returns true if there are no modified, untracked, added, or deleted files.
func (s Status) IsClean() bool {
	for _, fs := range s {
		if fs.Worktree != Unmodified || fs.Staging != Unmodified {
			return false
		}
	}
	return true
}

// CoreConfig represents the core configuration section of Git.
type CoreConfig struct {
	IgnoreCase bool
}

// Config represents the Git repository configuration.
type Config struct {
	Core CoreConfig
}

// IndexEntry represents a cached file entry in the Git index.
type IndexEntry struct {
	Path string
	Hash string
	Mode os.FileMode
}

// Repository represents a git repository wrapper.
type Repository struct {
	cfg Config
}

func (r *Repository) Config() (*Config, error) {
	return &r.cfg, nil
}

// Worktree represents a working tree of a git repository.
type Worktree struct {
	r          *Repository
	Root       string
	Index      map[string]*IndexEntry
	ignoreCase *bool // cached ignoreCase setting
}

// NewWorktree creates a new Worktree instance with the given repository and root directory.
func NewWorktree(r *Repository, root string) *Worktree {
	return &Worktree{
		r:     r,
		Root:  root,
		Index: make(map[string]*IndexEntry),
	}
}

// isIgnoreCase returns whether case-insensitive path comparisons should be used.
// It respects the core.ignorecase configuration.
func (w *Worktree) isIgnoreCase() bool {
	if w.ignoreCase != nil {
		return *w.ignoreCase
	}

	val := false
	if w.r != nil {
		cfg, err := w.r.Config()
		if err == nil {
			val = cfg.Core.IgnoreCase
		}
	} else if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		val = true
	}

	w.ignoreCase = &val
	return val
}

// pathsMatch compares two path strings respecting core.ignorecase.
func (w *Worktree) pathsMatch(a, b string) bool {
	if w.isIgnoreCase() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// sha256String computes SHA-256 for a string content.
func sha256String(content string) string {
	h := sha256.New()
	h.Write([]byte(content))
	return hex.EncodeToString(h.Sum(nil))
}

// hashFile computes the SHA-256 hash of a file on disk.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Status calculates the worktree status by comparing index entries with filesystem files.
// It handles case-folding on case-insensitive filesystems when core.ignorecase is enabled.
func (w *Worktree) Status() (Status, error) {
	status := make(Status)
	ignoreCase := w.isIgnoreCase()

	// 1. Scan the physical filesystem under w.Root
	fsFiles := make(map[string]string) // normalized/folded path -> actual disk path
	err := filepath.Walk(w.Root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			// Skip .git directory
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(w.Root, p)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		key := relPath
		if ignoreCase {
			key = strings.ToLower(relPath)
		}
		fsFiles[key] = relPath
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan worktree: %w", err)
	}

	// 2. Build index lookup tables
	indexKeyMap := make(map[string]*IndexEntry)
	for indexPath, entry := range w.Index {
		key := indexPath
		if ignoreCase {
			key = strings.ToLower(indexPath)
		}
		indexKeyMap[key] = entry
	}

	// 3. Compare Index entries against Filesystem
	for key, entry := range indexKeyMap {
		actualFsPath, existsOnDisk := fsFiles[key]
		if !existsOnDisk {
			// File is in index but not on disk -> Deleted
			status[entry.Path] = &FileStatus{
				Worktree: Deleted,
				Staging:  Unmodified,
			}
			continue
		}

		// File exists on disk. Compare content hashes.
		diskFullPath := filepath.Join(w.Root, filepath.FromSlash(actualFsPath))
		diskHash, err := hashFile(diskFullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to hash %s: %w", diskFullPath, err)
		}

		if diskHash != entry.Hash {
			// Content changed -> Modified
			status[entry.Path] = &FileStatus{
				Worktree: Modified,
				Staging:  Unmodified,
			}
		}
		// If content matches and core.ignorecase is true, even if actualFsPath
		// has different casing than entry.Path, it is considered UNMODIFIED / CLEAN.
	}

	// 4. Check for Untracked files (on disk but not in index)
	for key, actualFsPath := range fsFiles {
		if _, inIndex := indexKeyMap[key]; !inIndex {
			status[actualFsPath] = &FileStatus{
				Worktree: Untracked,
				Staging:  Untracked,
			}
		}
	}

	return status, nil
}
