package worktree

import (
	"slices"
	"strings"
)

// ChangeStatus is what an uncommitted change does to its file.
type ChangeStatus string

// The statuses of an uncommitted change.
const (
	ChangeModified  ChangeStatus = "modified"
	ChangeUntracked ChangeStatus = "untracked"
	ChangeDeleted   ChangeStatus = "deleted"
	ChangeRenamed   ChangeStatus = "renamed"
)

// Change is one uncommitted change of a worktree, as git status lists it; OrigPath is the path a renamed file had.
type Change struct {
	Path     string       `json:"path"`
	Status   ChangeStatus `json:"status"`
	OrigPath string       `json:"orig_path,omitempty"`
}

// Tag returns the status as the three-letter tag a list of changes shows: new, del, ren or mod.
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

// Label returns the change's path, preceded by its original path for a rename.
func (c Change) Label() string {
	if c.OrigPath == "" {
		return c.Path
	}

	return c.OrigPath + " → " + c.Path
}

// SelectChanges returns the changes of available that paths name, each once, in the order of paths; a path names the change at that path, else every change under it as a directory. When a path names none, it returns that path as the second result.
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

// matching returns the change at path if there is one, else the changes under path as a directory.
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

// ConflictKind is why a change cannot land in its target as it is.
type ConflictKind string

// The kinds of conflict.
const (
	ConflictModified ConflictKind = "modified" // the target changed the same file differently
	ConflictExists   ConflictKind = "exists"   // the target already holds a different file at a new file's path
	ConflictBinary   ConflictKind = "binary"   // the file differs and is binary, so it cannot be merged
)

// Conflict is one file a move of changes would conflict on.
type Conflict struct {
	Path string       `json:"path"`
	Kind ConflictKind `json:"kind"`
}

// ConflictRefusal is the error of a move refused because of its conflicts: a binary one, or any when the move does not resolve them.
type ConflictRefusal struct{ Conflicts []Conflict }

// Error names the paths that conflict.
func (r *ConflictRefusal) Error() string {
	paths := make([]string, 0, len(r.Conflicts))
	for _, conflict := range r.Conflicts {
		paths = append(paths, conflict.Path)
	}

	return "the selected changes conflict with the target: " + strings.Join(paths, ", ")
}

// HalfMoved is the error of a move that wrote the changes into the target but could not remove them from the source.
type HalfMoved struct{ Cause error }

// Error says the changes are on both sides, followed by the cause.
func (h *HalfMoved) Error() string {
	return "the changes are in the target but the source still holds them: " + h.Cause.Error()
}

// Unwrap returns the cause.
func (h *HalfMoved) Unwrap() error {
	return h.Cause
}

// ConflictCheck asks which of Changes, taken from the worktree at Source, would conflict in the worktree at Target, or, when Target is empty, in the tree of TargetRef.
type ConflictCheck struct {
	Source    string
	Target    string
	TargetRef string
	Changes   []Change
}

// Move asks to move Changes from the worktree at Source into the worktree at Target.
type Move struct {
	Source      string
	Target      string
	Changes     []Change
	Keep        bool   // leave the changes in the source too
	Resolve     bool   // merge conflicting text files with conflict markers rather than refuse
	TargetLabel string // the target's name in conflict markers
	SourceLabel string // the source's name in conflict markers
}

// Refuses reports whether a move meeting conflicts is refused: a binary conflict cannot take conflict markers, so it is refused even when the move resolves them, and any conflict is refused when it does not.
func Refuses(conflicts []Conflict, resolve bool) bool {
	binary := slices.ContainsFunc(conflicts, func(c Conflict) bool { return c.Kind == ConflictBinary })
	return binary || len(conflicts) > 0 && !resolve
}

// Refusal is the refusal of the move meeting conflicts, as Refuses decides it; nil when the move can go on.
func (m Move) Refusal(conflicts []Conflict) *ConflictRefusal {
	if !Refuses(conflicts, m.Resolve) {
		return nil
	}

	return &ConflictRefusal{Conflicts: conflicts}
}

// Moved is what a move did: the paths it merged with conflict markers, and whether the source kept its changes.
type Moved struct {
	Merged []string
	Kept   bool
}
