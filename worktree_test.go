package main

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestRepo builds a repository with the given ignorecase setting and
// index, backed by a fresh temporary directory.
func newTestRepo(t *testing.T, ignoreCase bool, index ...IndexEntry) (*Repository, string) {
	t.Helper()
	dir := t.TempDir()
	repo := NewRepository(&Config{Core: CoreConfig{IgnoreCase: ignoreCase}}, index)
	return repo, dir
}

// write creates a file (and its parent directories) under dir.
func write(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// entry returns an index entry for path whose expected content is hashed.
func entry(path, content string) IndexEntry {
	return IndexEntry{Path: path, Content: hashContent([]byte(content))}
}

// statusOf returns the Worktree.Status() result for a repo+dir combo,
// failing the test on error.
func statusOf(t *testing.T, repo *Repository, dir string) Status {
	t.Helper()
	status, err := NewWorktree(repo, dir).Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	return status
}

// dump logs every status entry, useful when a test fails.
func dump(t *testing.T, s Status) {
	t.Helper()
	for p, e := range s {
		t.Logf("%s: staging=%v worktree=%v", p, e.Staging, e.Worktree)
	}
}

func TestStatusCleanCaseInsensitiveCaseOnlyRename(t *testing.T) {
	// Index records README.md and src/main.go, but the case-insensitive
	// filesystem stored Readme.md and SRC/Main.go (different file and
	// directory casing). Contents are identical, so the tree is clean.
	repo, dir := newTestRepo(t, true,
		entry("README.md", "hello world"),
		entry("src/main.go", "package main"),
	)
	write(t, dir, "Readme.md", "hello world")
	write(t, dir, "SRC/Main.go", "package main")

	status := statusOf(t, repo, dir)
	if !status.IsClean() {
		dump(t, status)
		t.Fatal("IsClean() = false, want true on case-insensitive FS with case-only rename")
	}
}

func TestStatusCleanCaseSensitiveExactMatch(t *testing.T) {
	repo, dir := newTestRepo(t, false, entry("README.md", "hello"))
	write(t, dir, "README.md", "hello")

	status := statusOf(t, repo, dir)
	if !status.IsClean() {
		dump(t, status)
		t.Fatal("IsClean() = false, want true on case-sensitive FS with exact names")
	}
}

func TestStatusCaseSensitiveDistinguishesCaseOnly(t *testing.T) {
	// On a case-sensitive filesystem, README.md in the index is NOT the
	// same file as Readme.md on disk: README.md is deleted and Readme.md
	// is untracked.
	repo, dir := newTestRepo(t, false, entry("README.md", "hello"))
	write(t, dir, "Readme.md", "hello")

	status := statusOf(t, repo, dir)
	if status.IsClean() {
		t.Fatal("IsClean() = true, want false: case-sensitive FS must treat case-only names as distinct")
	}
	if got := status["README.md"]; got == nil || got.Worktree != Deleted {
		t.Errorf("README.md status = %v, want Deleted", got)
	}
	if got := status["Readme.md"]; got == nil || got.Worktree != Untracked {
		t.Errorf("Readme.md status = %v, want Untracked", got)
	}
}

func TestStatusDetectsModified(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "original"))
	write(t, dir, "foo.txt", "changed")

	status := statusOf(t, repo, dir)
	if status.IsClean() {
		t.Fatal("IsClean() = true, want false")
	}
	if got := status["foo.txt"]; got == nil || got.Worktree != Modified {
		t.Errorf("foo.txt status = %v, want Modified", got)
	}
}

func TestStatusDetectsUntracked(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "tracked"))
	write(t, dir, "foo.txt", "tracked")
	write(t, dir, "new.txt", "untracked")

	status := statusOf(t, repo, dir)
	if got := status["new.txt"]; got == nil || got.Worktree != Untracked {
		t.Errorf("new.txt status = %v, want Untracked", got)
	}
}

func TestStatusDetectsDeleted(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "tracked"))

	status := statusOf(t, repo, dir)
	if got := status["foo.txt"]; got == nil || got.Worktree != Deleted {
		t.Errorf("foo.txt status = %v, want Deleted", got)
	}
}

func TestStatusSkipsDotGitDirectory(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "x"))
	write(t, dir, "foo.txt", "x")
	write(t, dir, filepath.Join(".git", "config"), "dummy")

	status := statusOf(t, repo, dir)
	if !status.IsClean() {
		dump(t, status)
		t.Fatal("IsClean() = false, want true: .git contents must be skipped")
	}
}

func TestStatusCleanIgnoresEmptyIndex(t *testing.T) {
	repo, dir := newTestRepo(t, true)
	write(t, dir, "file.txt", "anything")

	status := statusOf(t, repo, dir)
	if got := status["file.txt"]; got == nil || got.Worktree != Untracked {
		t.Errorf("file.txt status = %v, want Untracked (empty index, all files untracked)", got)
	}
}

func TestPathsMatchHonoursIgnoreCase(t *testing.T) {
	ci := NewWorktree(NewRepository(&Config{Core: CoreConfig{IgnoreCase: true}}, nil), "/tmp")
	if !ci.pathsMatch("README.md", "Readme.md") {
		t.Error("pathsMatch(README.md, Readme.md) = false, want true with ignorecase")
	}
	if !ci.pathsMatch("src/main.go", "SRC/MAIN.GO") {
		t.Error("pathsMatch(src/main.go, SRC/MAIN.GO) = false, want true with ignorecase")
	}

	cs := NewWorktree(NewRepository(&Config{Core: CoreConfig{IgnoreCase: false}}, nil), "/tmp")
	if cs.pathsMatch("README.md", "Readme.md") {
		t.Error("pathsMatch(README.md, Readme.md) = true, want false without ignorecase")
	}
	if !cs.pathsMatch("README.md", "README.md") {
		t.Error("pathsMatch(README.md, README.md) = false, want true")
	}
}
