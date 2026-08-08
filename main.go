package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	demoReadme = "hello world"
	demoMainGo = "package main"
)

// writeDemoFile creates a file (and its parent directories) under dir.
func writeDemoFile(dir, name, content string) error {
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

// runCase exercises Worktree.Status() against the given index and on-disk
// files and reports whether the outcome matches the expectation.
func runCase(label string, ignoreCase bool, index []IndexEntry, diskFiles map[string]string, expectClean bool) bool {
	dir, err := os.MkdirTemp("", "go-git-demo-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	for name, content := range diskFiles {
		if err := writeDemoFile(dir, name, content); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", name, err)
			os.Exit(1)
		}
	}

	repo := NewRepository(&Config{Core: CoreConfig{IgnoreCase: ignoreCase}}, index)
	wt := NewWorktree(repo, dir)

	status, err := wt.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Status(): %v\n", err)
		os.Exit(1)
	}

	clean := status.IsClean()
	verdict := "PASS"
	if clean != expectClean {
		verdict = "FAIL"
	}
	fmt.Printf("%s: %s (IsClean()=%v, want %v)\n", verdict, label, clean, expectClean)
	if !clean {
		for p, e := range status {
			fmt.Printf("    %s: staging=%v worktree=%v\n", p, e.Staging, e.Worktree)
		}
	}
	return clean == expectClean
}

func main() {
	fmt.Println("go-git — Worktree.Status() case-insensitive fix")
	fmt.Println()

	index := []IndexEntry{
		{Path: "README.md", Content: hashContent([]byte(demoReadme))},
		{Path: "src/main.go", Content: hashContent([]byte(demoMainGo))},
	}

	ok := true

	// Scenario 1: index says README.md / src/main.go but the case-insensitive
	// filesystem stored Readme.md / SRC/Main.go. Contents are identical, so
	// the working tree must be reported clean.
	ok = runCase(
		"case-insensitive FS (core.ignorecase=true) with case-only rename -> clean",
		true,
		index,
		map[string]string{
			"Readme.md":      demoReadme,
			"SRC/Main.go":    demoMainGo,
		},
		true,
	) && ok

	// Scenario 2: same case-only rename on a case-sensitive filesystem
	// (core.ignorecase=false). README.md is deleted and Readme.md is
	// untracked — the tree must NOT be clean.
	ok = runCase(
		"case-sensitive FS (core.ignorecase=false) with case-only rename -> dirty",
		false,
		index,
		map[string]string{
			"Readme.md":   demoReadme,
			"src/Main.go": demoMainGo,
		},
		false,
	) && ok

	// Scenario 3: exact names on a case-sensitive filesystem -> clean.
	ok = runCase(
		"case-sensitive FS with exact names -> clean",
		false,
		index,
		map[string]string{
			"README.md":   demoReadme,
			"src/main.go": demoMainGo,
		},
		true,
	) && ok

	// Scenario 4: real content change must still be detected even when
	// core.ignorecase is enabled.
	ok = runCase(
		"case-insensitive FS with modified content -> dirty",
		true,
		index,
		map[string]string{
			"README.md":   demoReadme,
			"src/main.go": "package main // changed",
		},
		false,
	) && ok

	if !ok {
		os.Exit(1)
	}
	fmt.Println("\nAll scenarios passed.")
}
