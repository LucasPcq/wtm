package provision

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func (p *provisioning) undo(ctx context.Context, failure *wtm.Error) *wtm.Error {
	err := p.Deps.Git.RemoveWorktree(ctx, p.Worktree.Path)
	if name, created := worktree.CreatedBranch(p.Worktree.Checkout); created && err == nil {
		err = p.Deps.Git.DeleteBranch(ctx, name)
	}
	if err == nil {
		return failure
	}
	return &wtm.Error{
		Kind:     wtm.KindInternal,
		Code:     "unit.undo_failed",
		Message:  "could not undo worktree, left behind: worktree",
		Params:   wtm.Params{"step": "worktree", "left": "worktree"},
		FollowUp: p.leftBehind(),
		Cause:    errors.Join(failure, err),
	}
}

type cleanRequest struct {
	Branches []string `json:"branches"`
	Force    bool     `json:"force"`
}

func (p *provisioning) leftBehind() *wtm.FollowUp {
	branch, path := p.Worktree.Branch, p.Worktree.Path
	request, _ := json.Marshal(cleanRequest{Branches: []string{branch}, Force: true})
	return &wtm.FollowUp{
		Command: "clean",
		Request: request,
		Code:    "provision.left_behind",
		Message: "branch " + branch + " is left half-created at " + path + ": remove it with `wtm clean --force " + branch + "`",
		Params:  wtm.Params{"branch": branch, "path": path},
	}
}
