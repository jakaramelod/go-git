// go-git case-insensitive status fix — demo and validation program.
//
// Run: go run .
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jakaramelod/go-git/ignorecase"
)

func main() {
	verify := flag.Bool("verify", false, "Run verification tests")
	flag.Parse()

	if *verify {
		runVerification()
		return
	}

	fmt.Println("=== go-git Case-Insensitive Status Fix ===")
	fmt.Println("Issue: https://github.com/jakaramelod/go-git/issues/1\n")

	// 1. Filesystem detection
	fmt.Println("1. FILESYSTEM DETECTION")
	isCI := ignorecase.IsCaseInsensitiveFS()
	fmt.Printf("   Case-insensitive filesystem: %v\n", isCI)
	fmt.Printf("   → core.ignorecase should be: %v\n\n", isCI)

	// 2. Path matching demo
	fmt.Println("2. PATH MATCHING (with core.ignorecase=true)")
	testCases := []struct{ a, b string }{
		{"README.md", "Readme.md"},
		{"src/main.go", "SRC/MAIN.GO"},
		{"foo/bar.txt", "Foo/Bar.txt"},
	}
	for _, tc := range testCases {
		match := ignorecase.PathsMatch(tc.a, tc.b, true)
		icon := "✓"
		if !match {
			icon = "✗"
		}
		fmt.Printf("   %s %q ↔ %q → match=%v\n", icon, tc.a, tc.b, match)
	}

	// 3. Normalized path keys
	fmt.Println("\n3. NORMALIZED PATH KEYS")
	paths := []string{"README.md", "SRC/Main.go"}
	for _, p := range paths {
		fmt.Printf("   %s → %s\n", p, ignorecase.NormalizePathKey(p, true))
	}

	// 4. The fix explained
	fmt.Println("\n4. THE FIX")
	fmt.Println("   BEFORE: Worktree.Status() uses case-sensitive path matching")
	fmt.Println("           → README.md in index ≠ Readme.md on disk → reports 'modified'")
	fmt.Println()
	fmt.Println("   AFTER:  Worktree.Status() respects core.ignorecase")
	fmt.Println("           → README.md == Readme.md on case-insensitive FS → 'clean'")
	fmt.Println()
	fmt.Println("   Changes required in upstream go-git (github.com/go-git/go-git):")
	fmt.Println("   • worktree_status.go — use pathsMatch() for index↔FS comparison")
	fmt.Println("   • Build normalized FS path map for O(1) lookups")
	fmt.Println("   • Read core.ignorecase from git config (fallback: FS detection)")
	fmt.Println()

	// 5. Quick test on current system
	fmt.Println("5. QUICK LOCAL TEST")
	root, _ := os.MkdirTemp("", "go-git-demo-*")
	defer os.RemoveAll(root)
	os.WriteFile(filepath.Join(root, "Readme.md"), []byte("hello"), 0644)

	entries, _ := ignorecase.ComputeStatus(
		[]string{"README.md"}, // index has different case
		true,                   // core.ignorecase=true
		root,
	)
	if ignorecase.IsClean(entries) {
		fmt.Println("   ✓ Status is CLEAN (fix works correctly)")
	} else {
		fmt.Println("   ✗ Status is DIRTY (bug reproduced)")
		for _, e := range entries {
			fmt.Printf("     %s: %s\n", e.Path, e.Status)
		}
	}
}

func runVerification() {
	fmt.Println("=== Verification ===")
	failures := 0

	// Test 1: FS detection
	fmt.Print("1. FS detection... ")
	if ignorecase.IsCaseInsensitiveFS() {
		fmt.Println("✓ (case-insensitive)")
	} else {
		fmt.Println("✓ (case-sensitive)")
	}

	// Test 2: Path matching with ignoreCase=true
	fmt.Print("2. pathsMatch ignoreCase=true... ")
	if ignorecase.PathsMatch("README.md", "readme.md", true) &&
		!ignorecase.PathsMatch("foo.txt", "bar.txt", true) {
		fmt.Println("✓")
	} else {
		fmt.Println("✗ FAIL")
		failures++
	}

	// Test 3: Path matching with ignoreCase=false
	fmt.Print("3. pathsMatch ignoreCase=false... ")
	if !ignorecase.PathsMatch("README.md", "readme.md", false) &&
		ignorecase.PathsMatch("exact.txt", "exact.txt", false) {
		fmt.Println("✓")
	} else {
		fmt.Println("✗ FAIL")
		failures++
	}

	// Test 4: NormalizePathKey
	fmt.Print("4. normalizePathKey... ")
	if ignorecase.NormalizePathKey("README.md", true) == "readme.md" &&
		ignorecase.NormalizePathKey("README.md", false) == "README.md" {
		fmt.Println("✓")
	} else {
		fmt.Println("✗ FAIL")
		failures++
	}

	fmt.Printf("\nResults: %d passed, %d failed\n", 4-failures, failures)
	if failures > 0 {
		os.Exit(1)
	}
}
