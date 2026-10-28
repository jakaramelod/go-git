package git

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, root, name, body string) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestStatusCleanAfterCaseOnlyRenameWhenIgnoreCase(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "foo.txt", "hello\n")

	w := NewWorktree(root, true)
	w.SetIndex([]IndexEntry{{Path: "foo.txt", Content: "hello\n"}})

	// Case-only rename in the index (README.md -> Readme.md).
	w.SetIndex([]IndexEntry{{Path: "Foo.txt", Content: "hello\n"}})

	st, err := w.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsClean() {
		t.Fatalf("expected clean status with core.ignorecase, got %#v", st)
	}
}

func TestStatusDirtyOnCaseOnlyRenameWhenCaseSensitive(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "foo.txt", "hello\n")

	w := NewWorktree(root, false)
	w.SetIndex([]IndexEntry{{Path: "Foo.txt", Content: "hello\n"}})

	st, err := w.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.IsClean() {
		t.Fatal("expected dirty status on case-sensitive filesystem")
	}
	if st["Foo.txt"].Worktree != Deleted {
		t.Fatalf("Foo.txt want deleted, got %q", st["Foo.txt"].Worktree)
	}
	if st["foo.txt"].Worktree != Untracked {
		t.Fatalf("foo.txt want untracked, got %q", st["foo.txt"].Worktree)
	}
}

func TestStatusTreatsDistinctCaseFilesOnLinux(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "foo.txt", "a\n")
	writeFile(t, root, "Foo.txt", "b\n")

	w := NewWorktree(root, false)
	w.SetIndex([]IndexEntry{
		{Path: "foo.txt", Content: "a\n"},
		{Path: "Foo.txt", Content: "b\n"},
	})

	st, err := w.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !st.IsClean() {
		t.Fatalf("expected both case-distinct files clean, got %#v", st)
	}
}

func TestStatusReportsContentChangeWithIgnoreCase(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "foo.txt", "changed\n")

	w := NewWorktree(root, true)
	w.SetIndex([]IndexEntry{{Path: "Foo.txt", Content: "hello\n"}})

	st, err := w.Status()
	if err != nil {
		t.Fatal(err)
	}
	if st.IsClean() {
		t.Fatal("expected modified when content differs")
	}
	if st["Foo.txt"].Worktree != Modified {
		t.Fatalf("want modified, got %q", st["Foo.txt"].Worktree)
	}
}
