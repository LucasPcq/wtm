// Package owed settles the namespaces a clean could not give back because the
// shared service holding them was down: the moment to pay is whenever that
// service is up again, so every run that finds it up settles what it can.
package owed

import (
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
func Settle(params Params) Result {
	result := Result{Owed: map[string]int{}}
	owed := runjobs.LoadPendingRemovals(params.Context.StateDir)
	if len(owed) == 0 {
		return result
	}
	cfg, err := runconfig.Load(params.Context.StateDir)
	if err != nil {
		return result
	}

	live := liveWorktrees(params.Context.ProjectDir)
	up := rules.SharedJobsUp(rules.SharedJobsUpParams{Jobs: runjobs.Load(), Config: cfg})
	var done []domain.NamespaceRef
	for _, ref := range owed {
		name := rules.NamespaceName(rules.NamespaceNameParams{Config: cfg, Ref: ref})
		if live[ref.Worktree] {
			done = append(done, ref)
			params.Presenter.Status(flow.Notice{Kind: flow.NoticeNote, Text: fmt.Sprintf(domain.OwedRecreatedFmt, name, ref.Worktree)})
			continue
		}
		if !up[ref.Job] {
			result.Owed[ref.Job]++
			continue
		}
		var removed runjobs.RemoveNamespacesResult
		_ = params.Presenter.Stage(flow.StageParams{
			Message: fmt.Sprintf(domain.OwedDroppingFmt, name, ref.Job),
			Work: func() error {
				removed = runjobs.RemoveWorktreeNamespaces(runjobs.RemoveNamespacesParams{
					Config:  rules.JobsNamed(cfg, ref.Job),
					Env:     rules.NamespaceEnv(rules.NamespaceEnvParams{Worktree: ref.Worktree, Ordinal: ref.Ordinal}),
					WorkDir: params.Context.ProjectDir,
					Up:      up,
				})
				return nil
			},
		})
		for _, err := range removed.Errs {
			params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
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

// liveWorktrees are the worktrees that exist, by the slug a debt names them by.
func liveWorktrees(projectDir string) map[string]bool {
	live := map[string]bool{}
	all, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: projectDir})
	if err != nil {
		return live
	}
	for _, wt := range all {
		if wt.Branch != "" {
			live[rules.WorktreeSlug(wt.Branch)] = true
		}
	}
	return live
}
