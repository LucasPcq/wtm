// Package ordinal allocates a worktree's number from the flow that needs it, so
// the change is published like every other change to its identity.
package ordinal

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func Ensure(ctx flow.Context, branch string) error {
	claim, err := worktree.EnsureOrdinal(ref(ctx, branch))
	if err != nil {
		return err
	}
	if claim.Allocated {
		publish.Updated(publish.UpdatedParams{Context: ctx, Branch: branch, Changed: []domain.IdentityField{domain.IdentityOrdinal}})
	}
	return nil
}

type RetryParams struct {
	Context flow.Context
	Branch  string
	Do      func() error
}

// Retry allocates exactly when the service would once have done it lazily: the
// service answers ErrOrdinalUnallocated only when it needed the number, so the
// flow never has to know which reads do.
func Retry(params RetryParams) error {
	err := params.Do()
	if !errors.Is(err, domain.ErrOrdinalUnallocated) {
		return err
	}
	if err := Ensure(params.Context, params.Branch); err != nil {
		return err
	}
	return params.Do()
}

// BeforeHooks is best effort, as the hooks' environment always was: a number
// that cannot be allocated leaves the hooks their own environment.
func BeforeHooks(ctx flow.Context, branch string) {
	if !worktree.HookEnvPending(ref(ctx, branch)) {
		return
	}
	_ = Ensure(ctx, branch)
}

func ref(ctx flow.Context, branch string) worktree.WorktreeRef {
	return worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: branch}
}
