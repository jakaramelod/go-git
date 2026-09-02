package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	fmt.Println("=== go-git Case-Insensitive Status Fix Validator ===")

	tempDir, err := os.MkdirTemp("", "go-git-demo-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tempDir)

	content := "Package main\n"
	_ = os.WriteFile(filepath.Join(tempDir, "Readme.md"), []byte(content), 0644)

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
		Hash: sha256String(content),
		Mode: 0644,
	}

	status, err := wt.Status()
	if err != nil {
		fmt.Printf("Error calculating status: %v\n", err)
		return
	}

	fmt.Printf("Disk file: Readme.md\n")
	fmt.Printf("Index file: README.md\n")
	fmt.Printf("Core.IgnoreCase: %v\n", wt.isIgnoreCase())
	if status.IsClean() {
		fmt.Println("SUCCESS: Case-insensitive path comparison resolved clean state perfectly!")
	}
}
