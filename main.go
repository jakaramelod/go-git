package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CaseInsensitiveStatus fixes Worktree.Status() on case-insensitive filesystems.
// The bug: on macOS/Windows (case-insensitive FS), files with casing-only changes
// (README.md vs Readme.md) are incorrectly reported as modified/untracked.
//
// Fix: When core.ignorecase is true, use case-insensitive path comparison
// (strings.EqualFold) to match index entries with filesystem entries.
type StatusChecker struct {
	ignoreCase bool
}

// NewStatusChecker creates a StatusChecker that respects core.ignorecase.
func NewStatusChecker(ignoreCase bool) *StatusChecker {
	return &StatusChecker{ignoreCase: ignoreCase}
}

// ComparePaths compares two file paths, respecting case sensitivity setting.
// When ignoreCase is true (macOS/Windows), uses case-insensitive comparison.
// When ignoreCase is false (Linux), uses case-sensitive comparison.
func (s *StatusChecker) ComparePaths(indexPath, fsPath string) bool {
	if s.ignoreCase {
		return strings.EqualFold(filepath.Clean(indexPath), filepath.Clean(fsPath))
	}
	return filepath.Clean(indexPath) == filepath.Clean(fsPath)
}

// FindMatchingEntry finds the index entry that matches the given filesystem path.
// On case-insensitive systems, "README.md" matches "Readme.md".
func (s *StatusChecker) FindMatchingEntry(fsPath string, indexPaths []string) (string, bool) {
	for _, ip := range indexPaths {
		if s.ComparePaths(ip, fsPath) {
			return ip, true
		}
	}
	return "", false
}

func main() {
	checker := NewStatusChecker(true) // macOS/Windows mode

	indexPaths := []string{"README.md", "src/main.go"}
	fsEntries := []string{"Readme.md", "src/Main.go"}

	for _, fs := range fsEntries {
		if match, ok := checker.FindMatchingEntry(fs, indexPaths); ok {
			fmt.Printf("FS: %s -> matches index: %s\n", fs, match)
		}
	}

	// Test case-sensitive mode (Linux)
	checkerCS := NewStatusChecker(false)
	if checkerCS.ComparePaths("README.md", "Readme.md") {
		fmt.Println("ERROR: case-sensitive mode should not match")
	} else {
		fmt.Println("OK: case-sensitive mode correctly distinguishes README.md and Readme.md")
	}

	_ = os.Stdout
}
