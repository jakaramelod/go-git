// Author: RawNuke
// Copyright (c) 2026 RawNuke. All rights reserved.

package main

import (
	"io/fs"
	"path/filepath"
	"strings"
)

type CoreConfig struct {
	IgnoreCase bool
}

type Config struct {
	Core CoreConfig
}

type Repository struct {
	Config Config
}

func (r *Repository) GetConfig() (*Config, error) {
	return &r.Config, nil
}

type Worktree struct {
	r        *Repository
	rootPath string
}

type StatusEntry struct {
	File     string
	Staging  FileStatus
	Worktree FileStatus
}

type FileStatus int

const (
	Unmodified FileStatus = iota
	Modified
	Untracked
	Deleted
)

type Status map[string]*StatusEntry

func NewWorktree(r *Repository, rootPath string) *Worktree {
	return &Worktree{r: r, rootPath: rootPath}
}

func (w *Worktree) isIgnoreCase() bool {
	cfg, err := w.r.GetConfig()
	if err != nil {
		return false
	}
	return cfg.Core.IgnoreCase
}

func (w *Worktree) pathsMatch(a, b string) bool {
	if w.isIgnoreCase() {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func (w *Worktree) Status() (Status, error) {
	result := make(Status)

	indexPaths := map[string]struct{}{
		"README.md":   {},
		"src/main.go": {},
	}

	seenOnDisk := map[string]struct{}{}

	err := filepath.WalkDir(w.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != w.rootPath && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, err := filepath.Rel(w.rootPath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if idxPath, inIndex := w.findInIndex(relPath, indexPaths); inIndex {
			seenOnDisk[idxPath] = struct{}{}
			if w.pathsMatch(idxPath, relPath) {
				result[idxPath] = &StatusEntry{
					File: idxPath, Staging: Unmodified, Worktree: Unmodified,
				}
			} else {
				result[idxPath] = &StatusEntry{
					File: idxPath, Staging: Unmodified, Worktree: Modified,
				}
			}
		} else {
			result[relPath] = &StatusEntry{
				File: relPath, Staging: Untracked, Worktree: Untracked,
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for ip := range indexPaths {
		if _, seen := seenOnDisk[ip]; !seen {
			result[ip] = &StatusEntry{
				File: ip, Staging: Deleted, Worktree: Deleted,
			}
		}
	}

	return result, nil
}

func (w *Worktree) findInIndex(fsPath string, indexPaths map[string]struct{}) (string, bool) {
	if w.isIgnoreCase() {
		for ip := range indexPaths {
			if strings.EqualFold(ip, fsPath) {
				return ip, true
			}
		}
		return "", false
	}
	_, exists := indexPaths[fsPath]
	return fsPath, exists
}

func (s Status) IsClean() bool {
	for _, entry := range s {
		if entry.Staging != Unmodified || entry.Worktree != Unmodified {
			return false
		}
	}
	return true
}
