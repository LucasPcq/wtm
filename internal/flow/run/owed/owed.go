// Package owed settles the namespaces a clean could not give back because the
// shared service holding them was down: the moment to pay is whenever that
// service is up again, so every run that finds it up settles what it can.
package owed

import (
	"context"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	Context   flow.Context
	Presenter flow.Presenter
}

type Result struct {
	Settled []domain.NamespaceRef
	// Owed counts what is still owed, by shared job: its service is down.
	Owed map[string]int
}

// Settle drops every owed namespace whose service is up, and says so one line
// each. A debt whose worktree exists again is withdrawn instead: the namespace
// now belongs to the new worktree, and dropping it would destroy its data.
func Settle(ctx context.Context, params Params) Result {
	result := Result{Owed: map[string]int{}}
	owed := runjobs.LoadPendingRemovals(params.Context.StateDir)
	if len(owed) == 0 {
		return result
	}
	cfg, err := runconfig.Load(params.Context.StateDir)
	if err != nil {
		return result
	}

	live := liveBranches(ctx, params.Context.ProjectDir)
	up := rules.SharedJobsUp(rules.SharedJobsUpParams{Jobs: runjobs.Load(ctx), Config: cfg})
	var done []domain.NamespaceRef
	for _, ref := range owed {
		name := rules.NamespaceName(rules.NamespaceNameParams{Config: cfg, Ref: ref})
		if branches := live[ref.Worktree]; len(branches) > 0 {
			done = append(done, ref)
			params.Presenter.Status(flow.Notice{Kind: flow.NoticeNote, Text: fmt.Sprintf(domain.OwedRecreatedFmt, name, branches[0])})
			continue
		}
		if !up[ref.Job] {
			result.Owed[ref.Job]++
			continue
		}
		var removed runjobs.RemoveNamespacesResult
		_ = params.Presenter.Stage(ctx, flow.StageParams{
			Message: fmt.Sprintf(domain.OwedDroppingFmt, name, ref.Job),
			Work: func(ctx context.Context) error {
				removed = runjobs.RemoveWorktreeNamespaces(ctx, runjobs.RemoveNamespacesParams{
					Config:  rules.JobsNamed(cfg, ref.Job),
					Env:     rules.NamespaceEnv(rules.NamespaceEnvParams{Worktree: ref.Worktree, Ordinal: ref.Ordinal}),
					WorkDir: params.Context.ProjectDir,
					Up:      up,
				})
				return nil
			},
		})
		for _, failed := range removed.Failed {
			params.Presenter.Status(flow.Notice{
				Kind: flow.NoticeWarning,
				Text: fmt.Sprintf(domain.CleanDropFailedFmt, name, ref.Job, failed.Err),
			})
		}
		if len(removed.Released) == 0 {
			result.Owed[ref.Job]++
			continue
		}
		done = append(done, ref)
		result.Settled = append(result.Settled, ref)
		params.Presenter.Status(flow.Notice{
			Kind: flow.NoticeSuccess,
			Text: fmt.Sprintf(domain.PruneSettledNamespaceFmt, name, ref.Job, ref.Worktree),
		})
	}
	_ = runjobs.SettleRemovals(runjobs.SettleRemovalsParams{StateDir: params.Context.StateDir, Refs: done})
	return result
}

// liveBranches are the worktrees that exist, by the slug a namespace names
// them by. More than one under a slug is a collision that predates the refusal
// at creation.
func liveBranches(ctx context.Context, projectDir string) map[string][]string {
	live := map[string][]string{}
	all, err := worktree.ListAll(ctx, worktree.ListAllParams{ProjectDir: projectDir})
	if err != nil {
		return live
	}
	for _, wt := range all {
		if wt.Branch == "" {
			continue
		}
		slug := rules.WorktreeSlug(wt.Branch)
		live[slug] = append(live[slug], wt.Branch)
	}
	return live
}
