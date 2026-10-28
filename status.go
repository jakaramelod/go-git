package git

// FileStatus is one path in a Status map.
type FileStatus struct {
	Worktree StatusCode
}

// StatusCode is staging/worktree change kind.
type StatusCode byte

const (
	Unmodified StatusCode = ' '
	Untracked  StatusCode = '?'
	Modified   StatusCode = 'M'
	Deleted    StatusCode = 'D'
)

// Status is the result of Worktree.Status.
type Status map[string]FileStatus

// IsClean reports whether there are no untracked, modified, or deleted paths.
func (s Status) IsClean() bool {
	for _, st := range s {
		if st.Worktree != Unmodified {
			return false
		}
	}
	return true
}

// Status compares the index to the worktree. When core.ignorecase is set,
// path matching is case-insensitive (APFS/NTFS). When it is unset, paths
// that differ only by case stay distinct (Linux ext4).
func (w *Worktree) Status() (Status, error) {
	files, err := listWorktreeFiles(w.root)
	if err != nil {
		return nil, err
	}

	st := Status{}
	matchedDisk := map[string]bool{}

	for _, e := range w.index {
		diskPath, content, ok := w.findDisk(files, e.Path)
		if !ok {
			st[e.Path] = FileStatus{Worktree: Deleted}
			continue
		}
		matchedDisk[diskPath] = true
		if content != e.Content {
			st[e.Path] = FileStatus{Worktree: Modified}
			continue
		}
		st[e.Path] = FileStatus{Worktree: Unmodified}
	}

	for p := range files {
		if matchedDisk[p] {
			continue
		}
		if _, ok := w.findIndex(p); ok {
			continue
		}
		st[p] = FileStatus{Worktree: Untracked}
	}
	return st, nil
}
