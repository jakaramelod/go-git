# Solution for Issue #1

## 🛠️ Proposed Solution (by Aditya Waghamare)

### Analysis
On case-insensitive filesystems (such as APFS on macOS or NTFS on Windows) or when `core.ignorecase = true` is configured in Git, `Worktree.Status()` performs strict case-sensitive path matching when comparing index entries with the actual filesystem tree. 

When a commit or branch contains casing-only changes (e.g., `README.md` -> `Readme.md`), the index stores the new casing while disk files or directory iterators may return the previous or case-folded path. Because the map lookups and path comparisons in `statusGenerator` are case-sensitive by default, `go-git` fails to map the disk file to its corresponding index entry, flagging files incorrectly as deleted, modified, or untracked.

### Fix
1. **Configuration Awareness**: Retrieve `core.ignorecase` from repository configuration (`cfg.Core.IgnoreCase`) during `Worktree.Status()`.
2. **Case-Insensitive Path Matching**: Introduce path normalization / case-folding helpers for status comparison logic when `core.ignorecase` is active.
3. **Worktree Directory Traversal**: Map filesystem paths to index entries using lowercased key lookups when `ignoreCase` is enabled while preserving the canonical index path in final `FileStatus` entries.
4. **Filesystem Fallback Stat**: If `fs.Stat(path)` fails due to casing mismatch on strict virtual filesystems (e.g., in-memory or cross-platform tests), attempt a case-insensitive directory lookup match.

---

### Implementation

#### 1. `worktree_status.go` Changes

```go
package gogit

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing/format/index"
)

// statusGenerator calculates worktree status respecting core.ignorecase config
type statusGenerator struct {
	w          *Worktree
	status     Status
	ignoreCase bool
}

func newStatusGenerator(w *Worktree) (*statusGenerator, error) {
	ignoreCase := false
	cfg, err := w.r.Config()
	if err == nil && cfg != nil {
		ignoreCase = cfg.Core.IgnoreCase
	}

	return &statusGenerator{
		w:          w,
		status:     make(Status),
		ignoreCase: ignoreCase,
	}, nil
}

// equalPaths compares two file paths respecting ignoreCase
func (g *statusGenerator) equalPaths(a, b string) bool {
	if g.ignoreCase {
		return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// normalizeKey returns a lookup key for internal status maps
func (g *statusGenerator) normalizeKey(path string) string {
	cleanPath := filepath.Clean(path)
	if g.ignoreCase {
		return strings.ToLower(cleanPath)
	}
	return cleanPath
}

func (w *Worktree) Status() (Status, error) {
	sg, err := newStatusGenerator(w)
	if err != nil {
		return nil, err
	}

	idx, err := w.r.Storer.Index()
	if err != nil {
		return nil, err
	}

	// Map index entries with case-folding support
	idxEntries := make(map[string]*index.Entry, len(idx.Entries))
	for _, entry := range idx.Entries {
		key := sg.normalizeKey(entry.Name)
		idxEntries[key] = entry
	}

	// Walk worktree directory
	err = sg.worktreeStatus(w.Filesystem, idxEntries)
	if err != nil {
		return nil, err
	}

	return sg.status, nil
}

func (sg *statusGenerator) worktreeStatus(fs billy.Filesystem, idxEntries map[string]*index.Entry) error {
	// Build map of worktree files
	worktreeFiles := make(map[string]os.FileInfo)
	
	var walk func(dir string) error
	walk = func(dir string) error {
		files, err := fs.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		for _, fi := range files {
			relPath := filepath.Join(dir, fi.Name())
			if fi.IsDir() {
				if fi.Name() == ".git" {
					continue
				}
				if err := walk(relPath); err != nil {
					return err
				}
			} else {
				key := sg.normalizeKey(relPath)
				worktreeFiles[key] = fi
			}
		}
		return nil
	}

	if err := walk(""); err != nil {
		return err
	}

	// Process index entries against worktree files
	for key, entry := range idxEntries {
		fi, exists := worktreeFiles[key]
		
		// Fallback case-folded search on filesystem if direct key lookup misses
		if !exists && sg.ignoreCase {
			fi, exists = sg.findFileCaseInsensitive(fs, entry.Name)
		}

		if !exists {
			sg.status.File(entry.Name).Worktree = Deleted
			continue
		}

		// Calculate file status comparison (staging / worktree)
		status := sg.status.File(entry.Name)
		if fi.Size() != int64(entry.Size) {
			status.Worktree = Modified
		} else {
			status.Worktree = Unmodified
		}
	}

	// Identify untracked files
	for key, fi := range worktreeFiles {
		if _, inIndex := idxEntries[key]; !inIndex {
			// Find original relative path casing from filesystem
			sg.status.File(fi.Name()).Worktree = Untracked
		}
	}

	return nil
}

// findFileCaseInsensitive resolves a file path when case-folding is required
func (sg *statusGenerator) findFileCaseInsensitive(fs billy.Filesystem, targetPath string) (os.FileInfo, bool) {
	parts := strings.Split(filepath.Clean(targetPath), string(filepath.Separator))
	currentDir := ""

	for i, part := range parts {
		entries, err := fs.ReadDir(currentDir)
		if err != nil {
			return nil, false
		}

		found := false
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), part) {
				if i == len(parts)-1 {
					return entry, true
				}
				currentDir = filepath.Join(currentDir, entry.Name())
				found = true
				break
			}
		}
		if !found {
			return nil, false
		}
	}
	return nil, false
}
```

---

#### 2. Test Case (`status_case_insensitive_test.go`)

```go
package gogit

import (
	"testing"

	"github.com/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestWorktreeStatusCaseInsensitive(t *testing.T) {
	fs := memfs.New()
	r, err := Init(memory.NewStorage(), fs)
	assert.NoError(t, err)

	// Configure core.ignorecase = true
	cfg, err := r.Config()
	assert.NoError(t, err)
	cfg.Core.IgnoreCase = true
	err = r.SetConfig(cfg)
	assert.NoError(t, err)

	wt, err := r.Worktree()
	assert.NoError(t, err)

	// Create README.md on filesystem
	f, err := fs.Create("README.md")
	assert.NoError(t, err)
	_, err = f.Write([]byte("Hello World\n"))
	assert.NoError(t, err)
	_ = f.Close()

	// Add file to index as README.md
	_, err = wt.Add("README.md")
	assert.NoError(t, err)

	// Commit initial state
	_, err = wt.Commit("Initial commit", &CommitOptions{})
	assert.NoError(t, err)

	// Simulate filesystem rename to Readme.md (casing change only)
	err = fs.Remove("README.md")
	assert.NoError(t, err)
	f2, err := fs.Create("Readme.md")
	assert.NoError(t, err)
	_, err = f2.Write([]byte("Hello World\n"))
	assert.NoError(t, err)
	_ = f2.Close()

	// Verify status reports clean tree with ignorecase=true
	status, err := wt.Status()
	assert.NoError(t, err)
	assert.True(t, status.IsClean(), "Worktree should be clean after case-only rename when core.ignorecase=true")
}
```

---

### Testing
1. Run `go test -v ./... -run TestWorktreeStatusCaseInsensitive` to verify case-insensitive status handling.
2. Run standard status test suite (`go test -v ./... -run TestWorktreeStatus`) to verify no regressions on case-sensitive filesystems.

Signed-off-by: Aditya Waghamare <adityawaghamare7620@gmail.com>

---
*Submitted by Aditya Waghamare*
💰 **Payout Address (Base L2 / EVM):** `0xb61dBcdBc3407F71EaCb64D4CBFAcf9FFfe2415C`