// Package publish turns a change a flow just made into the event reporting it.
// The identity is read after the change, so the event carries the state a
// consumer converges to; a read that fails publishes nothing, and the next
// snapshot settles it.
package publish

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/hooks"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

func Created(ctx context.Context, project flow.Context, branch string) {
	emit(ctx, emitParams{Context: project, Branch: branch, Event: domain.Event{Type: domain.EventWorktreeCreated}})
}

type UpdatedParams struct {
	Context flow.Context
	Branch  string
	Changed []domain.IdentityField
}

func Updated(ctx context.Context, params UpdatedParams) {
	emit(ctx, emitParams{Context: params.Context, Branch: params.Branch, Event: domain.Event{Type: domain.EventWorktreeUpdated, Changed: params.Changed}})
}

type RelocatedParams struct {
	Context  flow.Context
	Branch   string
	FromPath string
}

func Relocated(ctx context.Context, params RelocatedParams) {
	emit(ctx, emitParams{Context: params.Context, Branch: params.Branch, Event: domain.Event{Type: domain.EventWorktreeRelocated, FromPath: params.FromPath}})
}

type ReparentedParams struct {
	Context    flow.Context
	Branch     string
	FromParent string
}

func Reparented(ctx context.Context, params ReparentedParams) {
	emit(ctx, emitParams{Context: params.Context, Branch: params.Branch, Event: domain.Event{Type: domain.EventWorktreeReparented, FromParent: params.FromParent}})
}

// ReparentedAll reports each move that was written, including the ones a
// failing batch got through before it stopped: they are not rolled back.
func ReparentedAll(ctx context.Context, project flow.Context, results []domain.ReparentResult) {
	for _, result := range results {
		Reparented(ctx, ReparentedParams{Context: project, Branch: result.Branch, FromParent: result.OldParent})
	}
}

// Capture reads the identity a removal is about to erase: removed carries the
// last state a consumer saw, and once git forgot the worktree there is nothing
// left to read.
func Capture(ctx context.Context, project flow.Context, branch string) (domain.WorktreeIdentity, bool) {
	if !project.Listening() {
		return domain.WorktreeIdentity{}, false
	}
	identity, err := worktree.Identity(ctx, ref(project, branch))
	return identity, err == nil
}

func Removed(ctx context.Context, project flow.Context, last domain.WorktreeIdentity) {
	project.Publish(ctx, domain.Event{Type: domain.EventWorktreeRemoved, Worktree: &last})
}

type ProvisionedParams struct {
	Context flow.Context
	Branch  string
	// Err is the on_create phase's; nil is a worktree ready to use.
	Err error
}

func Provisioned(ctx context.Context, params ProvisionedParams) {
	emit(ctx, emitParams{Context: params.Context, Branch: params.Branch, Event: outcome(domain.EventWorktreeProvisioned, params.Err)})
}

type DeprovisionedParams struct {
	Context flow.Context
	Branch  string
	// Err is the on_clean phase's; nil lets the removal go on.
	Err error
}

func Deprovisioned(ctx context.Context, params DeprovisionedParams) {
	emit(ctx, emitParams{Context: params.Context, Branch: params.Branch, Event: outcome(domain.EventWorktreeDeprovisioned, params.Err)})
}

func outcome(typ domain.EventType, err error) domain.Event {
	ok := err == nil
	event := domain.Event{Type: typ, OK: &ok}
	var failure hooks.Failure
	if errors.As(err, &failure) {
		event.Hook = failure.Cmd
		event.ExitCode = failure.ExitCode
	}
	return event
}

type emitParams struct {
	Context flow.Context
	Branch  string
	Event   domain.Event
}

func emit(ctx context.Context, params emitParams) {
	if !params.Context.Listening() {
		return
	}
	identity, err := worktree.Identity(ctx, ref(params.Context, params.Branch))
	if err != nil {
		return
	}
	event := params.Event
	event.Worktree = &identity
	params.Context.Publish(ctx, event)
}

func ref(ctx flow.Context, branch string) worktree.WorktreeRef {
	return worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: branch}
}
