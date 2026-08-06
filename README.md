# go-git Case-Insensitive Status Fix

Fix for [Issue #1](https://github.com/jakaramelod/go-git/issues/1): `Worktree.Status()` incorrectly reports dirty status on case-insensitive filesystems after case-only filename changes.

## Problem

On case-insensitive filesystems (Windows NTFS, macOS APFS), `go-git`'s `Worktree.Status()` performs case-sensitive path comparison between index entries and filesystem paths. This causes false "modified" reports when checking out a branch with filename casing changes (e.g., `README.md` → `Readme.md`), even though the file contents are identical.

## Solution

When `core.ignorecase` is `true`:
1. Build a **normalized filesystem path map** with lowercased keys for O(1) lookups
2. Use **case-insensitive comparison** (`strings.EqualFold`) for index→FS path matching
3. Use `filepath.ToSlash()` to normalize path separators (Windows `\` → `/`) to match git index format
4. Preserve case-sensitive behavior when `core.ignorecase` is `false`

## Package

- **`ignorecase/`** — Core fix logic: `BuildFSPathMap()`, `ComputeStatus()`, `PathsMatch()`, `NormalizePathKey()`
- **`cmd/go-git-fix/`** — Demo and verification program

## Tests

```bash
go test ./ignorecase/ -v
# All tests pass on Windows (case-insensitive FS)
# TestCaseSensitiveFSHandling is skipped on Windows (requires Linux)
```

## Upstream Integration

The fix should be integrated into `github.com/go-git/go-git` in:
- `worktree_status.go` — Use `pathsMatch()` for index↔FS comparison
- `worktree.go` — Build normalized FS path map, read `core.ignorecase`
- `config/config.go` — Ensure `core.ignorecase` is accessible
