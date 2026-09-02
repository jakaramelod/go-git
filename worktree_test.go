package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorktreeStatus_CaseInsensitiveClean(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "go-git-test-case-insens-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := "Hello Open Source World!"
	// Physical file on disk with casing "Readme.md"
	filePath := filepath.Join(tempDir, "Readme.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := &Repository{
		cfg: Config{
			Core: CoreConfig{
				IgnoreCase: true, // core.ignorecase = true
			},
		},
	}

	wt := NewWorktree(repo, tempDir)
	// Index contains the path with casing "README.md"
	wt.Index["README.md"] = &IndexEntry{
		Path: "README.md",
		Hash: sha256String(content),
		Mode: 0644,
	}

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Worktree.Status() returned error: %v", err)
	}

	if !status.IsClean() {
		t.Errorf("Expected status to be Clean when core.ignorecase=true, but got: %+v", status)
		for k, v := range status {
			t.Errorf("  File: %s -> Worktree:%c, Staging:%c", k, v.Worktree, v.Staging)
		}
	}
}

func TestWorktreeStatus_CaseSensitiveDirty(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "go-git-test-case-sens-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	content := "Case Sensitive Test"
	filePath := filepath.Join(tempDir, "Readme.md")
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := &Repository{
		cfg: Config{
			Core: CoreConfig{
				IgnoreCase: false, // core.ignorecase = false
			},
		},
	}

	wt := NewWorktree(repo, tempDir)
	// Index has "README.md", disk has "Readme.md"
	wt.Index["README.md"] = &IndexEntry{
		Path: "README.md",
		Hash: sha256String(content),
		Mode: 0644,
	}

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Worktree.Status() returned error: %v", err)
	}

	// On case-sensitive mode, "README.md" is missing/deleted, "Readme.md" is untracked
	if status.IsClean() {
		t.Errorf("Expected status to be Dirty when core.ignorecase=false with casing mismatch, but got Clean")
	}
}

func TestWorktreeStatus_ContentModified(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "go-git-test-modified-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	origContent := "Original Content"
	newContent := "Modified Content"

	filePath := filepath.Join(tempDir, "Readme.md")
	if err := os.WriteFile(filePath, []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := &Repository{
		cfg: Config{
			Core: CoreConfig{
				IgnoreCase: true,
			},
		},
	}

	wt := NewWorktree(repo, tempDir)
	wt.Index["README.md"] = &IndexEntry{
		Path: "README.md",
		Hash: sha256String(origContent),
		Mode: 0644,
	}

	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Worktree.Status() returned error: %v", err)
	}

	if status.IsClean() {
		t.Errorf("Expected status to be Modified, but got Clean")
	}

	fs, exists := status["README.md"]
	if !exists || fs.Worktree != Modified {
		t.Errorf("Expected README.md to have status Modified, got: %+v", fs)
	}
}

func TestWorktreeStatus_UntrackedFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "go-git-test-untracked-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	filePath := filepath.Join(tempDir, "new_feature.go")
	if err := os.WriteFile(filePath, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	repo := &Repository{
		cfg: Config{
			Core: CoreConfig{
				IgnoreCase: true,
			},
		},
	}

	wt := NewWorktree(repo, tempDir)
	status, err := wt.Status()
	if err != nil {
		t.Fatalf("Worktree.Status() returned error: %v", err)
	}

	if status.IsClean() {
		t.Errorf("Expected status to have untracked file, got Clean")
	}
	if fs, ok := status["new_feature.go"]; !ok || fs.Worktree != Untracked {
		t.Errorf("Expected new_feature.go to be Untracked, got %+v", fs)
	}
}
