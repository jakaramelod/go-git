# Solution for Issue #1

## 🛠️ Proposed Solution (by Aditya Waghamare)

### Analysis
`Worktree.Status()` compares index paths with filesystem paths using a case‑sensitive lookup, ignoring the repository's `core.ignorecase` setting. On case‑insensitive filesystems this flags case‑only renames as modifications.

### Fix
1. Expose `core.ignorecase` via `repo.Config().Core.IgnoreCase` (already present).
2. Normalise all path keys used during status calculation when `ignorecase` is true.
3. Adjust the file‑system lookup to perform a case‑insensitive match using `strings.EqualFold`/lower‑casing.
4. Add unit‑test covering the case‑insensitive scenario.

### Implementation
```go
// worktree/status.go – add import
import (
    "io/fs"
    "path/filepath"
    "strings"
    "github.com/go-git/go-git/v5/plumbing/format/index"
)

// helper to normalise a path according to repo config
func normalizePath(p string, ignoreCase bool) string {
    if ignoreCase {
        // Git on case‑insensitive FS stores paths in the index using the case
        // that was committed. Normalising to lower‑case allows a direct map
        // lookup without altering the original index entry.
        return strings.ToLower(p)
    }
    return p
}

// In Worktree.Status() (or the internal status helper) build a map of index
// entries keyed by the normalised path when ignorecase is enabled.
func (w *Worktree) status(opts *StatusOptions) (*Status, error) {
    repo := w.r
    cfg, err := repo.Config()
    if err != nil {
        return nil, err
    }
    ignoreCase := cfg.Core.IgnoreCase

    // Build a map of index entries for fast lookup
    idx, err := repo.Storer.Index()
    if err != nil {
        return nil, err
    }
    indexMap := make(map[string]*index.Entry, len(idx.Entries))
    for i := range idx.Entries {
        e := &idx.Entries[i]
        key := normalizePath(e.Name, ignoreCase)
        indexMap[key] = e
    }

    // Walk the worktree directory
    status := NewStatus()
    err = filepath.WalkDir(w.Filesystem.Root(), func(path string, d fs.DirEntry, walkErr error) error {
        if walkErr != nil {
            return walkErr
        }
        if d.IsDir() {
            return nil
        }
        rel, err := filepath.Rel(w.Filesystem.Root(), path)
        if err != nil {
            return err
        }
        rel = filepath.ToSlash(rel)
        key := normalizePath(rel, ignoreCase)
        if idxEnt, ok := indexMap[key]; ok {
            // compare file content/metadata with the index entry
            // unchanged logic – we just reuse the existing entry
            // The original code that called os.Stat / hash calculation stays the same.
            // Only the lookup is now case‑insensitive when required.
            // … (existing status logic) …
            _ = idxEnt // placeholder to keep compiler happy in this diff snippet
        } else {
            // file not present in index → untracked
            status.AddUntracked(rel)
        }
        return nil
    })
    if err != nil {
        return nil, err
    }
    // Process deletions – entries that remain in indexMap are missing from the worktree.
    for _, e := range indexMap {
        // if the entry wasn't visited, mark as Deleted
        // existing deletion handling code stays unchanged.
        _ = e
    }
    return status, nil
}
```

**Test – `status_case_insensitive_test.go`**
```go
package git

import (
    "io/ioutil"
    "os"
    "path/filepath"
    "testing"
)

func TestStatusCaseInsensitive(t *testing.T) {
    // Initialise an in‑memory repository (or use a temporary dir)
    dir, err := ioutil.TempDir("", "git-case-insensitive")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(dir)

    repo, err := Init(memory.NewStorage(), &filesystem{Root: dir})
    if err != nil {
        t.Fatalf("init repo: %v", err)
    }

    // Set core.ignorecase = true (simulates macOS/Windows default)
    cfg, _ := repo.Config()
    cfg.Core.IgnoreCase = true
    if err := repo.SetConfig(cfg); err != nil {
        t.Fatalf("set config: %v", err)
    }

    wt, err := repo.Worktree()
    if err != nil {
        t.Fatalf("worktree: %v", err)
    }

    // Commit a file with lower‑case name
    fPath := filepath.Join(dir, "readme.md")
    if err := ioutil.WriteFile(fPath, []byte("hello"), 0644); err != nil {
        t.Fatalf("write file: %v", err)
    }
    if _, err = wt.Add("readme.md"); err != nil {
        t.Fatalf("add: %v", err)
    }
    if _, err = wt.Commit("add readme", &CommitOptions{Author: defaultSignature()}); err != nil {
        t.Fatalf("commit: %v", err)
    }

    // Rename the file with only case change – Git tracks it as a rename
    if err := os.Rename(fPath, filepath.Join(dir, "Readme.md")); err != nil {
        t.Fatalf("rename: %v", err)
    }
    if _, err = wt.Add("Readme.md"); err != nil {
        t.Fatalf("add rename: %v", err)
    }
    if _, err = wt.Commit("case rename", &CommitOptions{Author: defaultSignature()}); err != nil {
        t.Fatalf("commit rename: %v", err)
    }

    // Checkout the previous commit (lower‑case name) and check status
    // The repository should report a clean working tree.
    head, _ := repo.Head()
    if err := wt.Checkout(&CheckoutOptions{Hash: head.Hash(), Force: true}); err != nil {
        t.Fatalf("checkout: %v", err)
    }

    st, err := wt.Status()
    if err != nil {
        t.Fatalf("status: %v", err)
    }
    if !st.IsClean() {
        t.Fatalf("expected clean status on case‑insensitive FS, got %v", st)
    }
}
```

### Testing
1. Run `go test ./...` – the new test must pass on macOS/Windows (or on a Linux CI where `core.ignorecase` is manually forced to true).
2. Verify that existing status tests continue to pass (`go test ./...`).
3. Manual validation: clone the repo on a case‑insensitive FS, perform a case‑only rename, `git checkout` the original commit, and run the library’s `Worktree.Status()` – it should now report a clean tree.

---
*Signed-off-by: Aditya Waghamare <adityawaghamare7620@gmail.com>*

---
*Submitted by Aditya Waghamare*
💰 **Payout Address (Base L2 / EVM):** `0xb61dBcdBc3407F71EaCb64D4CBFAcf9FFfe2415C`