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

func TestStatusCleanWithCaseOnlyRenameOnCaseInsensitiveFS(t *testing.T) {
	// The index has "README.md" and "src/main.go".
	// On a case-insensitive filesystem the files were checked out or created
	// as "Readme.md" and "SRC/Main.go".
	repo, dir := newTestRepo(t, true,
		entry("README.md", "hello"),
		entry("src/main.go", "package main"),
	)
	write(t, dir, "Readme.md", "hello")
	write(t, dir, "SRC/Main.go", "package main")

	status := statusOf(t, repo, dir)
	if !status.IsClean() {
		t.Errorf("Status.IsClean() = false, want true (casing differences should match under core.ignorecase)")
	}
	if len(status) != 2 {
		t.Fatalf("len(status) = %d, want 2", len(status))
	}
	for _, path := range []string{"README.md", "src/main.go"} {
		got := status[path]
		if got == nil {
			t.Errorf("missing status for %s", path)
			continue
		}
		if got.Worktree != Unmodified || got.Staging != Unmodified {
			t.Errorf("status[%s] = %v/%v, want Unmodified/Unmodified", path, got.Staging, got.Worktree)
		}
	}
}

func TestStatusDirtyWithCaseOnlyRenameOnCaseSensitiveFS(t *testing.T) {
	// Without ignorecase, "README.md" is considered missing and "Readme.md"
	// is treated as an untracked file.
	repo, dir := newTestRepo(t, false, entry("README.md", "hello"))
	write(t, dir, "Readme.md", "hello")

	status := statusOf(t, repo, dir)
	if status.IsClean() {
		t.Error("Status.IsClean() = true, want false (case-sensitive FS must not fold names)")
	}
	if got := status["README.md"]; got == nil || got.Worktree != Deleted {
		t.Errorf("README.md status = %v, want Deleted", got)
	}
	if got := status["Readme.md"]; got == nil || got.Worktree != Untracked {
		t.Errorf("Readme.md status = %v, want Untracked", got)
	}
}

func TestStatusCleanExactMatches(t *testing.T) {
	repo, dir := newTestRepo(t, false, entry("README.md", "hello"))
	write(t, dir, "README.md", "hello")

	status := statusOf(t, repo, dir)
	if !status.IsClean() {
		t.Errorf("Status.IsClean() = false, want true")
	}
}

func TestStatusDetectsModified(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "expected"))
	write(t, dir, "Foo.txt", "changed")

	status := statusOf(t, repo, dir)
	if status.IsClean() {
		t.Error("Status.IsClean() = true, want false")
	}
	if got := status["foo.txt"]; got == nil || got.Worktree != Modified {
		t.Errorf("foo.txt status = %v, want Modified", got)
	}
}

func TestStatusDetectsUntracked(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "expected"))
	write(t, dir, "foo.txt", "expected")
	write(t, dir, "new.txt", "untracked")

	status := statusOf(t, repo, dir)
	if status.IsClean() {
		t.Error("Status.IsClean() = true, want false")
	}
	if got := status["new.txt"]; got == nil || got.Worktree != Untracked {
		t.Errorf("new.txt status = %v, want Untracked", got)
	}
}

func TestStatusDetectsDeleted(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "expected"))

	status := statusOf(t, repo, dir)
	if status.IsClean() {
		t.Error("Status.IsClean() = true, want false")
	}
	if got := status["foo.txt"]; got == nil || got.Worktree != Deleted {
		t.Errorf("foo.txt status = %v, want Deleted", got)
	}
}

func TestStatusSkipsGitDir(t *testing.T) {
	repo, dir := newTestRepo(t, true, entry("foo.txt", "hello"))
	write(t, dir, "foo.txt", "hello")
	write(t, dir, ".git/HEAD", "ref: refs/heads/main")
	write(t, dir, ".git/objects/aa/bb", "data")

	status := statusOf(t, repo, dir)
	if !status.IsClean() {
		t.Errorf("Status.IsClean() = false, want true (.git files must be ignored)")
	}
	if _, ok := status[".git/HEAD"]; ok {
		t.Error(".git/HEAD leaked into status")
	}
}

func TestStatusEmptyIndex(t *testing.T) {
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
