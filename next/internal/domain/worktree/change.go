package worktree

import "strings"

type ChangeStatus string

const (
	ChangeModified  ChangeStatus = "modified"
	ChangeUntracked ChangeStatus = "untracked"
	ChangeDeleted   ChangeStatus = "deleted"
	ChangeRenamed   ChangeStatus = "renamed"
)

type Change struct {
	Path     string       `json:"path"`
	Status   ChangeStatus `json:"status"`
	OrigPath string       `json:"orig_path,omitempty"`
}

func (s ChangeStatus) Tag() string {
	switch s {
	case ChangeUntracked:
		return "new"
	case ChangeDeleted:
		return "del"
	case ChangeRenamed:
		return "ren"
	case ChangeModified:
	}
	return "mod"
}

func (c Change) Label() string {
	if c.OrigPath == "" {
		return c.Path
	}
	return c.OrigPath + " → " + c.Path
}

func SelectChanges(available []Change, paths []string) ([]Change, string) {
	seen := map[string]bool{}
	var selected []Change
	for _, path := range paths {
		matches := matching(available, path)
		if len(matches) == 0 {
			return nil, path
		}
		for _, change := range matches {
			if !seen[change.Path] {
				seen[change.Path] = true
				selected = append(selected, change)
			}
		}
	}
	return selected, ""
}

func matching(available []Change, path string) []Change {
	for _, change := range available {
		if change.Path == path {
			return []Change{change}
		}
	}
	prefix := strings.TrimSuffix(path, "/") + "/"
	var under []Change
	for _, change := range available {
		if strings.HasPrefix(change.Path, prefix) {
			under = append(under, change)
		}
	}
	return under
}

type ConflictKind string

const (
	ConflictModified ConflictKind = "modified"
	ConflictExists   ConflictKind = "exists"
	ConflictBinary   ConflictKind = "binary"
)

type Conflict struct {
	Path string       `json:"path"`
	Kind ConflictKind `json:"kind"`
}

type ConflictRefusal struct{ Conflicts []Conflict }

func (r *ConflictRefusal) Error() string {
	paths := make([]string, 0, len(r.Conflicts))
	for _, conflict := range r.Conflicts {
		paths = append(paths, conflict.Path)
	}
	return "the selected changes conflict with the target: " + strings.Join(paths, ", ")
}

type HalfMoved struct{ Cause error }

func (h *HalfMoved) Error() string {
	return "the changes are in the target but the source still holds them: " + h.Cause.Error()
}

func (h *HalfMoved) Unwrap() error {
	return h.Cause
}

type ConflictCheck struct {
	Source    string
	Target    string
	TargetRef string
	Changes   []Change
}

type Move struct {
	Source      string
	Target      string
	Changes     []Change
	Keep        bool
	Resolve     bool
	TargetLabel string
	SourceLabel string
}

type Moved struct {
	Merged []string
	Kept   bool
}
