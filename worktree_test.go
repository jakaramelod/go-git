// Author: RawNuke
// Copyright (c) 2026 RawNuke. All rights reserved.

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestCaseInsensitive(t *testing.T, dir string) {
	t.Helper()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "Readme.md"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "Main.go"), []byte("package main"), 0644)
}

func TestStatusCleanOnCaseInsensitiveFS(t *testing.T) {
	dir := t.TempDir()
	setupTestCaseInsensitive(t, dir)

	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: true}}}
	wt := NewWorktree(repo, dir)

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if !status.IsClean() {
		for p, e := range status {
			t.Logf("  %s: staging=%d worktree=%d", p, e.Staging, e.Worktree)
		}
		t.Fatal("Status.IsClean() = false, want true on case-insensitive FS")
	}
}

func TestStatusCleanOnCaseSensitiveFS(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "src"), 0755)
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(dir, "src", "main.go"), []byte("package main"), 0644)

	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: false}}}
	wt := NewWorktree(repo, dir)

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if !status.IsClean() {
		for p, e := range status {
			t.Logf("  %s: staging=%d worktree=%d", p, e.Staging, e.Worktree)
		}
		t.Fatal("Status.IsClean() = false, want true on case-sensitive FS with exact matches")
	}
}

func TestCaseSensitiveFSSeparatesDifferentCases(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(dir, "Readme.md"), []byte("world"), 0644)

	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: false}}}
	wt := NewWorktree(repo, dir)

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}

	if status.IsClean() {
		t.Fatal("Status.IsClean() = true, want false: files with different case should be distinguished")
	}
}

func TestPathMatchCaseInsensitive(t *testing.T) {
	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: true}}}
	wt := NewWorktree(repo, "/tmp")

	if !wt.pathsMatch("README.md", "Readme.md") {
		t.Error("pathsMatch(README.md, Readme.md) = false, want true with ignorecase")
	}
	if !wt.pathsMatch("src/main.go", "SRC/MAIN.GO") {
		t.Error("pathsMatch(src/main.go, SRC/MAIN.GO) = false, want true with ignorecase")
	}
}

func TestPathMatchCaseSensitive(t *testing.T) {
	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: false}}}
	wt := NewWorktree(repo, "/tmp")

	if wt.pathsMatch("README.md", "Readme.md") {
		t.Error("pathsMatch(README.md, Readme.md) = true, want false without ignorecase")
	}
	if !wt.pathsMatch("README.md", "README.md") {
		t.Error("pathsMatch(README.md, README.md) = false, want true for identical paths")
	}
}

func TestFindInIndexCaseInsensitive(t *testing.T) {
	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: true}}}
	wt := NewWorktree(repo, "/tmp")

	index := map[string]struct{}{"README.md": {}, "src/main.go": {}}

	path, found := wt.findInIndex("Readme.md", index)
	if !found || path != "README.md" {
		t.Errorf("findInIndex(Readme.md) = (%q, %v), want (README.md, true)", path, found)
	}

	path, found = wt.findInIndex("unknown.txt", index)
	if found {
		t.Errorf("findInIndex(unknown.txt) = (%q, true), want (\"\", false)", path)
	}
}

func TestFindInIndexCaseSensitive(t *testing.T) {
	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: false}}}
	wt := NewWorktree(repo, "/tmp")

	index := map[string]struct{}{"README.md": {}, "src/main.go": {}}

	_, found := wt.findInIndex("Readme.md", index)
	if found {
		t.Error("findInIndex(Readme.md) = true on case-sensitive FS, want false")
	}

	path, found := wt.findInIndex("README.md", index)
	if !found || path != "README.md" {
		t.Errorf("findInIndex(README.md) = (%q, %v), want (README.md, true)", path, found)
	}
}
