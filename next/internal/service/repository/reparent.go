package repository

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ReparentParams are the worktree of Branch, at Path ("" when no worktree holds it), to record as created from To, the record written through Meta and the change reported to Report.
type ReparentParams struct {
	Meta   MetaWriter
	Branch string
	Path   string
	To     string
	Report domain.Report
}

// Reparent records To as the parent of params' branch, the branch sync rebases it onto, and reports the worktree updated; a record that cannot be written is reported nowhere, its error returned.
func Reparent(ctx context.Context, params ReparentParams) error {
	var record worktree.Meta
	_, err := params.Meta.Update(ctx, worktree.MetaUpdate{Branch: params.Branch, Change: func(meta *worktree.Meta) error {
		meta.SourceBranch = params.To
		record = *meta
		return nil
	}})
	if err != nil {
		return err
	}

	params.Report(domain.WorktreeUpdated{Worktree: worktree.IdentityOf(worktree.IdentityParams{Branch: params.Branch, Path: params.Path, Record: record}), Changed: []worktree.IdentityField{worktree.IdentityParent}})
	return nil
}
