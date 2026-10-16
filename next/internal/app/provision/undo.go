package provision

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func (p *provisioning) undo(ctx context.Context, failure *wtm.Error) *wtm.Error {
	if err := p.Deps.Git.RemoveWorktree(ctx, p.Worktree.Path); err != nil {
		return p.leftBehind(leftBehind{Failure: failure, Err: err, Worktree: true})
	}
	name, created := worktree.CreatedBranch(p.Worktree.Checkout)
	if !created {
		return failure
	}
	if err := p.Deps.Git.DeleteBranch(ctx, name); err != nil {
		return p.leftBehind(leftBehind{Failure: failure, Err: err})
	}
	return failure
}

type leftBehind struct {
	Failure  *wtm.Error
	Err      error
	Worktree bool
}

type cleanRequest struct {
	Branches []string `json:"branches"`
	Force    bool     `json:"force"`
}

func (p *provisioning) leftBehind(left leftBehind) *wtm.Error {
	branch, path := p.Worktree.Branch, p.Worktree.Path
	request, _ := json.Marshal(cleanRequest{Branches: []string{branch}, Force: true})
	clean := "remove it with `wtm clean --force " + branch + "`"
	message, kept := "could not delete the branch, left behind: branch", "branch"
	followUp := &wtm.FollowUp{Message: "branch " + branch + " is left behind: " + clean, Params: wtm.Params{"branch": branch}}
	if left.Worktree {
		message, kept = "could not remove the worktree, left behind: worktree", "worktree"
		followUp = &wtm.FollowUp{Message: "branch " + branch + " is left half-created at " + path + ": " + clean, Params: wtm.Params{"branch": branch, "path": path}}
	}
	followUp.Command, followUp.Request, followUp.Code = "clean", request, "provision.left_behind"
	return &wtm.Error{
		Kind:     wtm.KindInternal,
		Code:     "unit.undo_failed",
		Message:  message,
		Params:   wtm.Params{"left": kept},
		FollowUp: followUp,
		Cause:    errors.Join(left.Failure, left.Err),
	}
}
