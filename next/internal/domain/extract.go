package domain

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	CodeExtractConflict  = "extract.conflict"
	CodeExtractHalfMoved = "extract.half_moved"
)

type ExtractConflictParams struct {
	Conflicts []worktree.Conflict
	Target    string
}

func ExtractConflict(params ExtractConflictParams) *Error {
	byKind := map[worktree.ConflictKind][]string{}
	var all []string
	for _, conflict := range params.Conflicts {
		byKind[conflict.Kind] = append(byKind[conflict.Kind], conflict.Path)
		all = append(all, conflict.Path)
	}
	refusal := &Error{Kind: KindConflict, Code: CodeExtractConflict, Params: Params{"target": params.Target, "files": strings.Join(all, ", ")}}
	if binary := byKind[worktree.ConflictBinary]; len(binary) > 0 {
		refusal.Params["files"] = strings.Join(binary, ", ")
		refusal.Message = fmt.Sprintf("cannot apply the selected changes: %s %s in %q and %s binary — conflict markers cannot be written, remove %s there or extract to another worktree",
			strings.Join(binary, ", "), plural(len(binary), "already exists", "already exist"), params.Target, plural(len(binary), "is", "are"), plural(len(binary), "it", "them"))
		return refusal
	}
	var clauses []string
	if modified := byKind[worktree.ConflictModified]; len(modified) > 0 {
		clauses = append(clauses, strings.Join(modified, ", ")+" "+plural(len(modified), "was also modified", "were also modified"))
	}
	if exists := byKind[worktree.ConflictExists]; len(exists) > 0 {
		clauses = append(clauses, strings.Join(exists, ", ")+" "+plural(len(exists), "already exists", "already exist"))
	}
	refusal.Message = fmt.Sprintf("cannot apply the selected changes: %s in %q — resolve %s there, extract to another worktree, or re-run with --on-conflict resolve",
		strings.Join(clauses, " and "), params.Target, plural(len(all), "it", "them"))
	return refusal
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}
