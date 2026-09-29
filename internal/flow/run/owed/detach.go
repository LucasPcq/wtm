package owed

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	DataStart = "start"
	DataDefer = "defer"
)

// Snapshot is what the worktrees about to be removed hold in the shared
// services, and which of those services are up to take it back.
type Snapshot struct {
	Config   domain.RunConfig
	Holdings []domain.NamespaceHolding
	Up       map[string]bool
}

func (s Snapshot) Held() []domain.HeldNamespace {
	return rules.HeldNamespaces(rules.HeldNamespacesParams{Holdings: s.Holdings, Up: s.Up})
}

type ReadParams struct {
	Context  flow.Context
	Branches []string
}

// Read must run before the removal: the holdings are read from each worktree's
// own state, and its remove commands run in its directory.
func Read(params ReadParams) Snapshot {
	cfg, err := runconfig.Load(params.Context.StateDir)
	if err != nil || len(rules.Removable(cfg).Jobs) == 0 {
		return Snapshot{}
	}
	var holdings []domain.NamespaceHolding
	for _, branch := range params.Branches {
		holding, found := holdingOf(params.Context, cfg, branch)
		if found {
			holdings = append(holdings, holding)
		}
	}
	if len(holdings) == 0 {
		return Snapshot{Config: cfg}
	}
	up := rules.SharedJobsUp(rules.SharedJobsUpParams{Jobs: runjobs.Load(), Config: cfg})
	return Snapshot{Config: cfg, Holdings: holdings, Up: up}
}

// holdingOf keeps only what the worktree actually carved out: one created and
// thrown away without ever starting the stack owes nothing, and running its
// detach would be a DROP DATABASE on a database that never existed.
func holdingOf(ctx flow.Context, cfg domain.RunConfig, branch string) (domain.NamespaceHolding, bool) {
	held := worktree.NamespacesOf(worktree.ParentBranchParams{StateDir: ctx.StateDir, Branch: branch})
	if len(held) == 0 {
		return domain.NamespaceHolding{}, false
	}
	jobs := rules.Removable(rules.JobsHeld(cfg, held))
	if len(jobs.Jobs) == 0 {
		return domain.NamespaceHolding{}, false
	}
	wt, err := worktree.FindByBranch(worktree.FindByBranchParams{ProjectDir: ctx.ProjectDir, Branch: branch})
	if err != nil {
		return domain.NamespaceHolding{}, false
	}
	env, err := worktree.JobEnv(worktree.JobEnvParams{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Dir: wt.Path})
	if err != nil {
		return domain.NamespaceHolding{}, false
	}
	return domain.NamespaceHolding{Branch: branch, WorkDir: wt.Path, Env: env, Config: jobs}, true
}

// DataPreset answers the data step from --drop-data, so it is not asked and the
// recap still reads the answer.
func DataPreset(dropData bool) string {
	if dropData {
		return DataStart
	}
	return ""
}

type DataStepParams struct {
	Key      string
	KeepData bool
	Snapshot func(flow.Answers) Snapshot
}

// DataStep asks, before the recap, what to do with data held by a service that
// is down. It is only ever asked then: a service that is up takes its data back
// without a question, and --keep-data has already answered.
func DataStep(params DataStepParams) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   params.Key,
		Label: domain.DataStepLabel,
		Skip: func(answers flow.Answers) (bool, string) {
			if params.KeepData {
				return true, ""
			}
			return len(rules.DownServices(params.Snapshot(answers).Held())) == 0, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			held := params.Snapshot(answers).Held()
			return flow.StepContent{
				Title:       domain.DataStepTitle,
				Description: dataDescription(held),
				Options: []flow.Option{
					{Label: fmt.Sprintf(domain.DataStartOptionFmt, strings.Join(rules.DownServices(held), ", ")), Value: DataStart},
					{Separator: true},
					{Label: domain.DataDeferOption, Value: DataDefer},
				},
			}, nil
		},
		// Starting a service nobody asked for is not a safe default.
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: DataDefer}, nil
		},
		Summarize: func(answer flow.Answer) string {
			if answer.Value == DataStart {
				return domain.DataStartSummary
			}
			return domain.DataDeferSummary
		},
	}
}

func dataDescription(held []domain.HeldNamespace) string {
	lines := []string{domain.DataStepIntro}
	for _, namespace := range held {
		if !namespace.Up {
			lines = append(lines, fmt.Sprintf(domain.DataStepLineFmt, namespace.Name, namespace.Job))
		}
	}
	return strings.Join(append(lines, "", domain.DataStepOutro), "\n")
}

type DetachParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	Snapshot  Snapshot
	StartDown bool
}

// Detach gives back what the snapshot's worktrees hold, before their claims are
// released: a claim released may be the last, and a namespace cannot be given
// back to a service that is down. What is left is queued for the service's next
// start, and every line says which way it went.
func Detach(params DetachParams) {
	if len(params.Snapshot.Holdings) == 0 {
		return
	}
	up := params.Snapshot.Up
	var errs []error
	if params.StartDown {
		release, started, startErrs := bringUpDown(params)
		defer release()
		up = started
		errs = startErrs
	}

	var result runjobs.RemoveNamespacesResult
	for _, holding := range params.Snapshot.Holdings {
		removed := drop(dropParams{Presenter: params.Presenter, Holding: holding, Up: up})
		result.Released = append(result.Released, removed.Released...)
		result.Deferred = append(result.Deferred, removed.Deferred...)
		result.Errs = append(result.Errs, removed.Errs...)
	}
	result.Errs = append(errs, result.Errs...)
	report(reportParams{Presenter: params.Presenter, Config: params.Snapshot.Config, Result: result})

	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: params.Context.StateDir, Refs: result.Deferred}); err != nil {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
	}
}

// bringUpDown starts every service the snapshot needs that is down, and hands
// back how to let them all go once the data is dropped.
func bringUpDown(params DetachParams) (release func(), up map[string]bool, errs []error) {
	up = make(map[string]bool, len(params.Snapshot.Up))
	for job, isUp := range params.Snapshot.Up {
		up[job] = isUp
	}
	var releases []func()
	for _, job := range rules.DownServices(params.Snapshot.Held()) {
		var jobRelease func()
		err := params.Presenter.Stage(flow.StageParams{
			Message: fmt.Sprintf(domain.OwedBringUpStageFmt, job),
			Work: func() error {
				var bringErr error
				jobRelease, bringErr = BringUp(BringUpParams{Context: params.Context, Config: params.Snapshot.Config, Job: job})
				return bringErr
			},
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		up[job] = true
		releases = append(releases, jobRelease)
	}
	return func() {
		for _, jobRelease := range releases {
			jobRelease()
		}
	}, up, errs
}

type dropParams struct {
	Presenter flow.Presenter
	Holding   domain.NamespaceHolding
	Up        map[string]bool
}

// drop runs the detach commands under a stage: a DROP DATABASE takes seconds,
// and the line reporting it would otherwise follow a silent pause.
func drop(params dropParams) runjobs.RemoveNamespacesResult {
	names := make([]string, 0, len(params.Holding.Config.Jobs))
	for _, job := range params.Holding.Config.Jobs {
		names = append(names, job.Name)
	}
	var result runjobs.RemoveNamespacesResult
	_ = params.Presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.DataDroppingFmt, params.Holding.Branch, strings.Join(names, ", ")),
		Work: func() error {
			result = runjobs.RemoveWorktreeNamespaces(runjobs.RemoveNamespacesParams{
				Config:  params.Holding.Config,
				Env:     params.Holding.Env,
				WorkDir: params.Holding.WorkDir,
				Up:      params.Up,
			})
			return nil
		},
	})
	return result
}

type reportParams struct {
	Presenter flow.Presenter
	Config    domain.RunConfig
	Result    runjobs.RemoveNamespacesResult
}

func report(params reportParams) {
	for _, ref := range params.Result.Released {
		params.Presenter.Status(flow.Notice{
			Kind: flow.NoticeSuccess,
			Text: fmt.Sprintf(domain.CleanRemovedNamespaceFmt, rules.NamespaceName(rules.NamespaceNameParams{Config: params.Config, Ref: ref}), ref.Job),
		})
	}
	for _, ref := range params.Result.Deferred {
		params.Presenter.Status(flow.Notice{
			Kind: flow.NoticeWarning,
			Text: fmt.Sprintf(domain.CleanDeferredNamespaceFmt, ref.Job, rules.NamespaceName(rules.NamespaceNameParams{Config: params.Config, Ref: ref})),
		})
	}
	for _, err := range params.Result.Errs {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
	}
}
