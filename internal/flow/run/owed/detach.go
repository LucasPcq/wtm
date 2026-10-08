package owed

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
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

func (s Snapshot) holding(branch string) (domain.NamespaceHolding, bool) {
	for _, holding := range s.Holdings {
		if holding.Branch == branch {
			return holding, true
		}
	}
	return domain.NamespaceHolding{}, false
}

type ReadParams struct {
	Context  flow.Context
	Branches []string
}

// Read must run before the removal: the holdings are read from each worktree's
// own state and environment, both gone with it.
func Read(ctx context.Context, params ReadParams) Snapshot {
	cfg, err := runconfig.Load(params.Context.StateDir)
	if err != nil || len(rules.Removable(cfg).Jobs) == 0 {
		return Snapshot{}
	}
	live := liveBranches(ctx, params.Context.ProjectDir)
	var holdings []domain.NamespaceHolding
	for _, branch := range params.Branches {
		holding, found := holdingOf(ctx, holdingParams{Context: params.Context, Config: cfg, Branch: branch, Live: live})
		if found {
			holdings = append(holdings, holding)
		}
	}
	if len(holdings) == 0 {
		return Snapshot{Config: cfg}
	}
	up := rules.SharedJobsUp(rules.SharedJobsUpParams{Jobs: runjobs.Load(ctx), Config: cfg})
	return Snapshot{Config: cfg, Holdings: holdings, Up: up}
}

type holdingParams struct {
	Context flow.Context
	Config  domain.RunConfig
	Branch  string
	Live    map[string][]string
}

// holdingOf keeps only what the worktree actually carved out: one created and
// thrown away without ever starting the stack owes nothing, and running its
// detach would be a DROP DATABASE on a database that never existed.
func holdingOf(ctx context.Context, params holdingParams) (domain.NamespaceHolding, bool) {
	held := worktree.NamespacesOf(worktree.ParentBranchParams{StateDir: params.Context.StateDir, Branch: params.Branch})
	if len(held) == 0 {
		return domain.NamespaceHolding{}, false
	}
	jobs := rules.Removable(rules.JobsHeld(params.Config, held))
	if len(jobs.Jobs) == 0 {
		return domain.NamespaceHolding{}, false
	}
	wt, err := worktree.FindByBranch(ctx, worktree.FindByBranchParams{ProjectDir: params.Context.ProjectDir, Branch: params.Branch})
	if err != nil {
		return domain.NamespaceHolding{}, false
	}
	env, err := seam.JobEnv(ctx, seam.JobEnvParams{ProjectDir: params.Context.ProjectDir, StateDir: params.Context.StateDir, WorkDir: wt.Path, Publisher: params.Context.Publisher})
	if err != nil {
		return domain.NamespaceHolding{}, false
	}
	holding := domain.NamespaceHolding{Branch: params.Branch, WorkDir: wt.Path, Env: env, Config: jobs}
	holding.SharedWith = sharedWith(sharedWithParams{Holding: holding, Live: params.Live})
	return holding, true
}

type sharedWithParams struct {
	Holding domain.NamespaceHolding
	Live    map[string][]string
}

// sharedWith names another live worktree reached under the same slug. A
// namespace is named after the slug, so it is that worktree's too.
func sharedWith(params sharedWithParams) string {
	slug := rules.HoldingRef(params.Holding, "").Worktree
	for _, branch := range params.Live[slug] {
		if branch != params.Holding.Branch {
			return branch
		}
	}
	return ""
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

type DropperParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	// Snapshot is read before the first removal, while the worktrees exist.
	Snapshot  Snapshot
	StartDown bool
	KeepData  bool
}

// Dropper gives back what removed worktrees held, one worktree at a time and
// only once that worktree is gone: a failure anywhere before leaves its data
// where it was. The services it started to do so run until Close.
type Dropper struct {
	params  DropperParams
	up      map[string]bool
	release func()
}

func NewDropper(ctx context.Context, params DropperParams) *Dropper {
	d := &Dropper{params: params, up: params.Snapshot.Up, release: func() {}}
	if params.StartDown && !params.KeepData && len(params.Snapshot.Holdings) > 0 {
		d.bringUpDown(ctx)
	}
	return d
}

func (d *Dropper) Close() { d.release() }

// Drop gives back what branch held, now that its worktree is gone, and says
// which way each namespace went. What could not be dropped is owed to the
// service's next start; what is dropped settles any older debt for it.
func (d *Dropper) Drop(ctx context.Context, branch string) []domain.NamespaceOutcome {
	holding, found := d.params.Snapshot.holding(branch)
	if !found {
		return nil
	}
	if d.params.KeepData {
		return kept(keptParams{Snapshot: d.params.Snapshot, Holding: holding, Reason: domain.CleanKeptByFlag})
	}
	if holding.SharedWith != "" {
		return d.keepShared(holding)
	}

	result := drop(ctx, dropParams{Context: d.params.Context, Presenter: d.params.Presenter, Holding: holding, Up: d.up})
	d.report(result)
	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: d.params.Context.StateDir, Refs: result.Deferred()}); err != nil {
		d.params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
	}
	if err := runjobs.SettleRemovals(runjobs.SettleRemovalsParams{StateDir: d.params.Context.StateDir, Refs: result.Released}); err != nil {
		d.params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
	}
	return outcomesOf(outcomesParams{Config: d.params.Snapshot.Config, Branch: branch, Result: result})
}

func (d *Dropper) keepShared(holding domain.NamespaceHolding) []domain.NamespaceOutcome {
	slug := rules.HoldingRef(holding, "").Worktree
	reason := fmt.Sprintf(domain.CleanNamespaceSharedReasonFmt, holding.SharedWith, slug)
	outcomes := kept(keptParams{Snapshot: d.params.Snapshot, Holding: holding, Reason: reason})
	for _, outcome := range outcomes {
		d.params.Presenter.Status(flow.Notice{
			Kind: flow.NoticeWarning,
			Text: fmt.Sprintf(domain.CleanNamespaceSharedFmt, outcome.Name, holding.SharedWith, slug),
		})
	}
	return outcomes
}

// bringUpDown starts every service the snapshot needs that is down; Close lets
// them all go once the data is dropped.
func (d *Dropper) bringUpDown(ctx context.Context) {
	up := make(map[string]bool, len(d.up))
	for job, isUp := range d.up {
		up[job] = isUp
	}
	var releases []func()
	for _, job := range rules.DownServices(d.params.Snapshot.Held()) {
		var jobRelease func()
		err := d.params.Presenter.Stage(ctx, flow.StageParams{
			Message: fmt.Sprintf(domain.OwedBringUpStageFmt, job),
			Work: func(ctx context.Context) error {
				var bringErr error
				jobRelease, bringErr = BringUp(ctx, BringUpParams{Context: d.params.Context, Config: d.params.Snapshot.Config, Job: job})
				return bringErr
			},
		})
		if err != nil {
			d.params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
			continue
		}
		up[job] = true
		releases = append(releases, jobRelease)
	}
	d.up = up
	d.release = func() {
		for _, jobRelease := range releases {
			jobRelease()
		}
	}
}

type dropParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	Holding   domain.NamespaceHolding
	Up        map[string]bool
}

// drop runs the detach commands under a stage: a DROP DATABASE takes seconds,
// and the line reporting it would otherwise follow a silent pause. They run
// from the project: the worktree's directory is gone by now.
func drop(ctx context.Context, params dropParams) runjobs.RemoveNamespacesResult {
	names := make([]string, 0, len(params.Holding.Config.Jobs))
	for _, job := range params.Holding.Config.Jobs {
		names = append(names, job.Name)
	}
	var result runjobs.RemoveNamespacesResult
	_ = params.Presenter.Stage(ctx, flow.StageParams{
		Message: fmt.Sprintf(domain.DataDroppingFmt, params.Holding.Branch, strings.Join(names, ", ")),
		Work: func(ctx context.Context) error {
			result = runjobs.RemoveWorktreeNamespaces(ctx, runjobs.RemoveNamespacesParams{
				Config:  params.Holding.Config,
				Env:     params.Holding.Env,
				WorkDir: params.Context.ProjectDir,
				Up:      params.Up,
			})
			return nil
		},
	})
	return result
}

func (d *Dropper) report(result runjobs.RemoveNamespacesResult) {
	name := func(ref domain.NamespaceRef) string {
		return rules.NamespaceName(rules.NamespaceNameParams{Config: d.params.Snapshot.Config, Ref: ref})
	}
	for _, ref := range result.Released {
		d.params.Presenter.Status(flow.Notice{Kind: flow.NoticeSuccess, Text: fmt.Sprintf(domain.CleanRemovedNamespaceFmt, name(ref), ref.Job)})
	}
	for _, ref := range result.Down {
		d.params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: fmt.Sprintf(domain.CleanDeferredNamespaceFmt, ref.Job, name(ref))})
	}
	for _, failed := range result.Failed {
		d.params.Presenter.Status(flow.Notice{
			Kind: flow.NoticeWarning,
			Text: fmt.Sprintf(domain.CleanDropFailedFmt, name(failed.Ref), failed.Ref.Job, failed.Err),
		})
	}
}

type outcomesParams struct {
	Config domain.RunConfig
	Branch string
	Result runjobs.RemoveNamespacesResult
}

func outcomesOf(params outcomesParams) []domain.NamespaceOutcome {
	outcome := func(ref domain.NamespaceRef, status domain.NamespaceStatus, reason string) domain.NamespaceOutcome {
		return domain.NamespaceOutcome{
			Branch: params.Branch,
			Job:    ref.Job,
			Name:   rules.NamespaceName(rules.NamespaceNameParams{Config: params.Config, Ref: ref}),
			Status: status,
			Reason: reason,
		}
	}
	var outcomes []domain.NamespaceOutcome
	for _, ref := range params.Result.Released {
		outcomes = append(outcomes, outcome(ref, domain.NamespaceDropped, ""))
	}
	for _, ref := range params.Result.Down {
		outcomes = append(outcomes, outcome(ref, domain.NamespaceDeferred, fmt.Sprintf(domain.NamespaceServiceDownFmt, ref.Job)))
	}
	for _, failed := range params.Result.Failed {
		outcomes = append(outcomes, outcome(failed.Ref, domain.NamespaceDeferred, failed.Err.Error()))
	}
	return outcomes
}

type keptParams struct {
	Snapshot Snapshot
	Holding  domain.NamespaceHolding
	Reason   string
}

func kept(params keptParams) []domain.NamespaceOutcome {
	outcomes := make([]domain.NamespaceOutcome, 0, len(params.Holding.Config.Jobs))
	for _, job := range params.Holding.Config.Jobs {
		outcomes = append(outcomes, domain.NamespaceOutcome{
			Branch: params.Holding.Branch,
			Job:    job.Name,
			Name:   rules.NamespaceName(rules.NamespaceNameParams{Config: params.Snapshot.Config, Ref: rules.HoldingRef(params.Holding, job.Name)}),
			Status: domain.NamespaceKept,
			Reason: params.Reason,
		})
	}
	return outcomes
}
