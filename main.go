// Author: RawNuke
// Copyright (c) 2026 RawNuke. All rights reserved.

package main

import (
	"fmt"
	"os"
)

func main() {
	dir, err := os.MkdirTemp("", "go-git-demo-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	os.MkdirAll(dir+"/src", 0755)
	os.WriteFile(dir+"/Readme.md", []byte("hello world"), 0644)
	os.WriteFile(dir+"/src/Main.go", []byte("package main"), 0644)

	repo := &Repository{Config: Config{Core: CoreConfig{IgnoreCase: true}}}
	wt := NewWorktree(repo, dir)

	status, err := wt.Status()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Status() error: %v\n", err)
		os.Exit(1)
	}

	if status.IsClean() {
		fmt.Println("PASS: Status.IsClean() = true on case-insensitive filesystem")
	} else {
		fmt.Println("FAIL: Status.IsClean() = false")
		for p, e := range status {
			fmt.Printf("  %s: staging=%d worktree=%d\n", p, e.Staging, e.Worktree)
		}
		os.Exit(1)
	}
}
