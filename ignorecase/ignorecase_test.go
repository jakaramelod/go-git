package ignorecase

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCaseInsensitiveFSDetection verifies the filesystem detection logic.
func TestCaseInsensitiveFSDetection(t *testing.T) {
	result := IsCaseInsensitiveFS()
	t.Logf("Case-insensitive filesystem: %v", result)
	// On Windows and macOS, this should be true
	// On Linux, depends on the filesystem type
}

// TestPathsMatch verifies case-insensitive path comparison.
func TestPathsMatch(t *testing.T) {
	tests := []struct {
		a, b       string
		ignoreCase bool
		want       bool
	}{
		{"README.md", "Readme.md", true, true},
		{"README.md", "Readme.md", false, false},
		{"src/Main.go", "SRC/MAIN.GO", true, true},
		{"src/Main.go", "SRC/MAIN.GO", false, false},
		{"exact.txt", "exact.txt", true, true},
		{"exact.txt", "exact.txt", false, true},
		{"foo.txt", "bar.txt", true, false},
		{"foo.txt", "bar.txt", false, false},
	}

	for _, tt := range tests {
		got := PathsMatch(tt.a, tt.b, tt.ignoreCase)
		if got != tt.want {
			t.Errorf("PathsMatch(%q, %q, %v) = %v, want %v",
				tt.a, tt.b, tt.ignoreCase, got, tt.want)
		}
	}
}

// TestNormalizePathKey verifies path key normalization.
func TestNormalizePathKey(t *testing.T) {
	tests := []struct {
		path       string
		ignoreCase bool
		want       string
	}{
		{"README.md", true, "readme.md"},
		{"README.md", false, "README.md"},
		{"SRC/Main.go", true, "src/main.go"},
		{"SRC/Main.go", false, "SRC/Main.go"},
	}

	for _, tt := range tests {
		got := NormalizePathKey(tt.path, tt.ignoreCase)
		if got != tt.want {
			t.Errorf("NormalizePathKey(%q, %v) = %q, want %q",
				tt.path, tt.ignoreCase, got, tt.want)
		}
	}
}

// TestStatusCleanAfterCaseOnlyChange simulates the exact scenario from the issue:
// After a case-only rename on a case-insensitive filesystem, status must be clean.
func TestStatusCleanAfterCaseOnlyChange(t *testing.T) {
	if !IsCaseInsensitiveFS() {
		t.Skip("Test requires case-insensitive filesystem (Windows/macOS)")
	}

	// Create temp worktree
	root, err := os.MkdirTemp("", "go-git-status-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Create a file on disk with one case: "Readme.md"
	if err := os.WriteFile(filepath.Join(root, "Readme.md"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// Index has same file but different case: "README.md"
	indexPaths := []string{"README.md"}

	// Compute status with ignoreCase=true
	entries, err := ComputeStatus(indexPaths, true, root)
	if err != nil {
		t.Fatal(err)
	}

	if !IsClean(entries) {
		t.Errorf("Expected clean status, got dirty:")
		for _, e := range entries {
			t.Logf("  %s: %s", e.Path, e.Status)
		}
	} else {
		t.Log("✓ Status is clean (case-insensitive matching works)")
	}
}

// TestStatusCleanMatchesIndexCorrectly verifies normal status computation.
func TestStatusCleanMatchesIndexCorrectly(t *testing.T) {
	root, err := os.MkdirTemp("", "go-git-status-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Create files on disk
	files := []string{"README.md", "src/main.go", "src/utils/helper.go"}
	for _, f := range files {
		fullPath := filepath.Join(root, f)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		if err := os.WriteFile(fullPath, []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Index has exact same paths
	indexPaths := []string{"README.md", "src/main.go", "src/utils/helper.go"}

	entries, err := ComputeStatus(indexPaths, false, root)
	if err != nil {
		t.Fatal(err)
	}

	if !IsClean(entries) {
		t.Errorf("Expected clean status for exact match:")
		for _, e := range entries {
			t.Logf("  %s: %s", e.Path, e.Status)
		}
	} else {
		t.Logf("✓ Clean status: %d files match", len(files))
	}
}

// TestDeletedFileDetection verifies that truly deleted files are detected.
func TestDeletedFileDetection(t *testing.T) {
	root, err := os.MkdirTemp("", "go-git-status-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Index references a file that doesn't exist on disk
	indexPaths := []string{"deleted-file.txt"}

	entries, err := ComputeStatus(indexPaths, false, root)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range entries {
		if e.Path == "deleted-file.txt" && e.Status == "deleted" {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'deleted-file.txt' to be marked as 'deleted'")
	} else {
		t.Log("✓ Deleted file correctly detected")
	}
}

// TestUntrackedFileDetection verifies untracked file detection.
func TestUntrackedFileDetection(t *testing.T) {
	root, err := os.MkdirTemp("", "go-git-status-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Create a file on disk not in index
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	// Index is empty
	indexPaths := []string{}

	entries, err := ComputeStatus(indexPaths, false, root)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, e := range entries {
		if e.Path == "untracked.txt" && e.Status == "untracked" {
			found = true
		}
	}
	if !found {
		t.Error("Expected 'untracked.txt' to be marked as 'untracked'")
	} else {
		t.Log("✓ Untracked file correctly detected")
	}
}

// TestMixedCaseIndexOnCaseInsensitiveFS is the core regression test.
// It verifies that when the index has paths with different casing than
// the filesystem, status is clean (on case-insensitive systems).
func TestMixedCaseIndexOnCaseInsensitiveFS(t *testing.T) {
	if !IsCaseInsensitiveFS() {
		t.Skip("Test requires case-insensitive filesystem")
	}

	root, err := os.MkdirTemp("", "go-git-case-regression-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Files on disk use lowercase
	fsFiles := []string{
		"readme.md",
		"src/main.go",
		"docs/install.md",
	}
	for _, f := range fsFiles {
		fullPath := filepath.Join(root, f)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(f), 0644)
	}

	// Index has mixed-case versions of the same files
	indexPaths := []string{
		"README.md",
		"SRC/Main.Go",
		"DOCS/INSTALL.MD",
	}

	entries, err := ComputeStatus(indexPaths, true, root)
	if err != nil {
		t.Fatal(err)
	}

	// All entries should be "unmodified" on case-insensitive FS
	for _, e := range entries {
		if e.Status != "unmodified" {
			t.Errorf("Index path %q (FS path %q) has status %q, expected 'unmodified'",
				e.IndexPath, e.FSPath, e.Status)
		}
	}

	if IsClean(entries) {
		t.Log("✓ All mixed-case index paths match correctly on case-insensitive FS")
	}
}

// TestCaseSensitiveFSHandling verifies case-sensitive behavior is preserved.
func TestCaseSensitiveFSHandling(t *testing.T) {
	if IsCaseInsensitiveFS() {
		t.Skip("Test requires case-sensitive filesystem (Linux ext4/xfs)")
	}

	// On case-sensitive FS, paths differing by case ARE different files
	result := PathsMatch("README.md", "readme.md", false)
	if result {
		t.Error("On case-sensitive FS, 'README.md' != 'readme.md'")
	} else {
		t.Log("✓ Case-sensitive comparison preserved: paths differ by case → not match")
	}
}

// TestBuildFSPathMap verifies the filesystem path map construction.
func TestBuildFSPathMap(t *testing.T) {
	root, err := os.MkdirTemp("", "go-git-fsmap-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	// Create test files
	fsFiles := []string{"README.md", "src/main.go", "src/utils/helper.go"}
	for _, f := range fsFiles {
		fullPath := filepath.Join(root, f)
		os.MkdirAll(filepath.Dir(fullPath), 0755)
		os.WriteFile(fullPath, []byte(f), 0644)
	}

	// Build map with ignoreCase=true
	pathMap, err := BuildFSPathMap(root, true)
	if err != nil {
		t.Fatal(err)
	}

	// Verify all files are accessible via lowercased keys
	for _, f := range fsFiles {
		key := strings.ToLower(f)
		if _, ok := pathMap[key]; !ok {
			t.Errorf("Missing key %q in path map", key)
		}
	}

	t.Logf("✓ Built FS path map with %d entries", len(pathMap))
}
