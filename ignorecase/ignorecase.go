// Package ignorecase provides the fix for go-git Worktree.Status() to handle
// case-insensitive filesystems correctly.
//
// Problem: On case-insensitive filesystems (Windows NTFS, macOS APFS),
// go-git's Worktree.Status() performs case-sensitive path comparison between
// index entries and filesystem paths, causing false "modified" reports when
// a branch has filename casing changes (e.g., README.md → Readme.md).
//
// Solution: When core.ignorecase is true, use case-insensitive path matching.
// This involves:
//  1. Building a normalized (lowercased) filesystem path map
//  2. Using case-insensitive comparison for index→FS path matching
//  3. Preserving case-sensitive behavior on case-sensitive filesystems
//
// See: https://github.com/jakaramelod/go-git/issues/1
package ignorecase

import (
	"os"
	"path/filepath"
	"strings"
)

// IsCaseInsensitiveFS detects whether the underlying filesystem is
// case-insensitive by creating two files that differ only in case.
func IsCaseInsensitiveFS() bool {
	tmpDir, err := os.MkdirTemp("", "go-git-case-test-*")
	if err != nil {
		return false // safe default
	}
	defer os.RemoveAll(tmpDir)

	lower := filepath.Join(tmpDir, "a")
	upper := filepath.Join(tmpDir, "A")

	if err := os.WriteFile(lower, []byte("lower"), 0644); err != nil {
		return false
	}
	if err := os.WriteFile(upper, []byte("UPPER"), 0644); err != nil {
		return false
	}

	data, err := os.ReadFile(lower)
	if err != nil {
		return false
	}

	// On case-insensitive FS, writing "A" overwrites "a"
	return string(data) == "UPPER"
}

// PathsMatch compares two filesystem paths, handling case sensitivity.
// When ignoreCase is true, uses case-insensitive comparison.
func PathsMatch(a, b string, ignoreCase bool) bool {
	if ignoreCase {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// NormalizePathKey returns a normalized path for use as a map key.
// When ignoreCase is true, lowercases the path.
func NormalizePathKey(path string, ignoreCase bool) string {
	if ignoreCase {
		return strings.ToLower(path)
	}
	return path
}

// BuildFSPathMap walks a directory and returns a map from normalized path
// to actual filesystem path. On case-insensitive filesystems, map keys
// are lowercased to enable case-insensitive lookups.
//
// Paths are normalized to use forward slashes (matching git index format).
// Directories named ".git" are skipped.
func BuildFSPathMap(root string, ignoreCase bool) (map[string]string, error) {
	pathMap := make(map[string]string)
	err := filepath.Walk(root, func(fsPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		relPath, err := filepath.Rel(root, fsPath)
		if err != nil {
			return err
		}
		// Normalize to forward slashes (git index format)
		relPath = filepath.ToSlash(relPath)
		key := NormalizePathKey(relPath, ignoreCase)
		pathMap[key] = relPath
		return nil
	})
	return pathMap, err
}

// StatusEntry represents a single file's status.
type StatusEntry struct {
	Path     string
	Status   string // "unmodified", "modified", "deleted", "untracked"
	IndexPath string
	FSPath    string
}

// ComputeStatus compares index entries against filesystem paths,
// respecting case sensitivity. This is the core fix for the bug.
//
// Parameters:
//   - indexPaths: list of paths from the git index
//   - ignoreCase: core.ignorecase config value
//   - rootDir: worktree root directory
//
// Returns a list of status entries.
func ComputeStatus(indexPaths []string, ignoreCase bool, rootDir string) ([]StatusEntry, error) {
	fsPathMap, err := BuildFSPathMap(rootDir, ignoreCase)
	if err != nil {
		return nil, err
	}

	matchedFSNorms := make(map[string]bool)
	var entries []StatusEntry

	// Phase 1: Compare each index entry against the filesystem
	for _, indexPath := range indexPaths {
		// Normalize to forward slashes (git always uses /)
		indexPath = filepath.ToSlash(indexPath)
		normKey := NormalizePathKey(indexPath, ignoreCase)
		fsPath, existsInFS := fsPathMap[normKey]
		entry := StatusEntry{Path: indexPath, IndexPath: indexPath}

		if existsInFS {
			matchedFSNorms[normKey] = true

			// KEY FIX: On case-insensitive FS, if paths differ only by case,
			// they refer to the SAME file — treat as unmodified
			if ignoreCase && strings.EqualFold(indexPath, fsPath) {
				entry.Status = "unmodified"
				entry.FSPath = fsPath
			} else if indexPath == fsPath {
				entry.Status = "unmodified"
				entry.FSPath = fsPath
			} else {
				// Paths differ in more than case → potentially modified
				entry.Status = "modified"
				entry.FSPath = fsPath
			}
		} else {
			// Index entry not found on filesystem
			entry.Status = "deleted"
		}
		entries = append(entries, entry)
	}

	// Phase 2: Detect untracked files (on FS but not in index)
	for normKey, fsPath := range fsPathMap {
		if !matchedFSNorms[normKey] {
			entries = append(entries, StatusEntry{
				Path:   fsPath,
				Status: "untracked",
				FSPath: fsPath,
			})
		}
	}

	return entries, nil
}

// IsClean returns true if all entries are unmodified.
func IsClean(entries []StatusEntry) bool {
	for _, e := range entries {
		if e.Status != "unmodified" {
			return false
		}
	}
	return true
}
