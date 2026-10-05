// Package extract runs the `wtm extract` flow.
package extract

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/create"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Source string
	Files  []string
	To     string
	From   string
	// Keep is --keep; KeepSet says the flag was given, so the mode is not asked.
	Keep        bool
	KeepSet     bool
	FastForward bool
	// OnConflict is --on-conflict, empty when it was not given.
	OnConflict string
	Isolation  domain.Isolation
}

// Outcome is Nothing when there was nothing to extract — no worktree with
// changes, or a source without any — and Result otherwise.
type Outcome struct {
	Result  domain.ExtractResult
	Nothing error
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	Extracted(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &extractFlow{
		runCtx:    ctx,
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		changes:   map[string][]domain.ExtractFile{},
		paths:     map[string]string{},
	}
	return f.run()
}

type extractFlow struct {
	runCtx    context.Context
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter
	// statuses back the pickers, listed only when one is shown.
	statuses []domain.WorktreeStatus
	// changes and paths are per source branch, filled as each is looked at.
	changes map[string][]domain.ExtractFile
	paths   map[string]string
	// creates is the target the flags name when no worktree holds it yet.
	creates bool
	create  create.Embedded
	// update replaces the divergence lookup of the create steps, for a test.
	update func(flow.Answers) decide.SourceUpdatePrompt
	target func(string) domain.BranchTarget
}

func (f *extractFlow) run() (Outcome, error) {
	if f.request.Source == "" && !f.prompter.Interactive() {
		return Outcome{}, domain.ErrExtractSourceRequired
	}
	if f.request.Source == "" || f.request.To == "" {
		if err := f.listWorktrees(); err != nil {
			return Outcome{}, err
		}
	}

	if f.request.Source == "" {
		if len(f.dirty()) == 0 {
			return f.conclude(Outcome{Nothing: domain.ErrNoDirtyWorktrees})
		}
	} else {
		files, err := f.sourceChanges(f.request.Source)
		if err != nil {
			return Outcome{}, err
		}
		if len(files) == 0 {
			return f.conclude(Outcome{Nothing: domain.ErrNoChangesToExtract, Result: domain.ExtractResult{SourceBranch: f.request.Source}})
		}
		// A --files matching no change is refused before a recap the run could
		// never carry out.
		if len(f.request.Files) > 0 {
			if _, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{Available: files, Paths: f.request.Files}); err != nil {
				return Outcome{}, err
			}
		}
	}

	if f.request.To != "" {
		_, err := worktree.FindByBranch(f.runCtx, worktree.FindByBranchParams{ProjectDir: f.ctx.ProjectDir, Branch: f.request.To})
		f.creates = err != nil
	}
	if f.creates && f.request.From == f.request.To {
		return Outcome{}, fmt.Errorf(domain.BranchOwnParentFmt, f.request.To, domain.FlagFrom)
	}
	f.create = f.embed(f.runCtx)
	if err := f.create.CheckBranch(); err != nil {
		return Outcome{}, err
	}

	answers, err := f.prompter.Ask(f.session(f.runCtx))
	if errors.Is(err, domain.ErrUserAborted) {
		return f.abort()
	}
	if err != nil {
		return Outcome{}, err
	}
	return f.extract(answers)
}

func (f *extractFlow) listWorktrees() error {
	return f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.ExtractScanLoading,
		Work: func(ctx context.Context) error {
			statuses, err := worktree.List(ctx, domain.ListParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Config: f.ctx.Config})
			if err != nil {
				return fmt.Errorf("list worktrees: %w", err)
			}
			f.statuses = statuses
			return nil
		},
	})
}

// dirty are the worktrees with uncommitted changes, the only valid sources.
func (f *extractFlow) dirty() []domain.WorktreeStatus {
	dirty := make([]domain.WorktreeStatus, 0, len(f.statuses))
	for _, status := range f.statuses {
		if status.IsDirty {
			dirty = append(dirty, status)
		}
	}
	return dirty
}

// sourceChanges resolves a source given by name, as --to is: an exact branch.
func (f *extractFlow) sourceChanges(branch string) ([]domain.ExtractFile, error) {
	wt, err := worktree.FindByBranch(f.runCtx, worktree.FindByBranchParams{ProjectDir: f.ctx.ProjectDir, Branch: branch})
	if err != nil {
		return nil, fmt.Errorf(domain.ExtractSourceNotFoundFmt, branch, err)
	}
	f.paths[branch] = wt.Path
	var files []domain.ExtractFile
	err = f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.ExtractScanLoading,
		Work: func(ctx context.Context) error {
			var listErr error
			files, listErr = worktree.ListChanges(ctx, worktree.ListChangesParams{WorktreePath: wt.Path})
			return listErr
		},
	})
	if err != nil {
		return nil, err
	}
	f.changes[branch] = files
	return files, nil
}

// pickedChanges lists a source picked in the wizard, once.
func (f *extractFlow) pickedChanges(branch string) ([]domain.ExtractFile, error) {
	if files, ok := f.changes[branch]; ok {
		return files, nil
	}
	path := f.statusPath(branch)
	if path == "" {
		return nil, nil
	}
	files, err := worktree.ListChanges(f.runCtx, worktree.ListChangesParams{WorktreePath: path})
	if err != nil {
		return nil, err
	}
	f.paths[branch] = path
	f.changes[branch] = files
	return files, nil
}

func (f *extractFlow) statusPath(branch string) string {
	for _, status := range f.statuses {
		if status.Branch == branch {
			return status.Path
		}
	}
	return ""
}

func (f *extractFlow) abort() (Outcome, error) {
	f.presenter.Notice(flow.AbortedNotice)
	return Outcome{Aborted: true}, nil
}

func (f *extractFlow) conclude(outcome Outcome) (Outcome, error) {
	return outcome, f.presenter.Extracted(outcome)
}

type target struct {
	path     string
	branch   string
	envPorts domain.EnvPortPlan
	warnings []string
}

func (f *extractFlow) extract(answers flow.Answers) (Outcome, error) {
	source := answers.Value(KeySource)
	available, err := f.pickedChanges(source)
	if err != nil {
		return Outcome{}, err
	}
	selected, err := rules.SelectExtractFiles(rules.SelectExtractFilesParams{Available: available, Paths: answers.Values(KeyFiles)})
	if err != nil {
		return Outcome{}, err
	}

	dest, proceed, err := f.resolveTarget(answers)
	if err != nil {
		return Outcome{}, err
	}
	if !proceed {
		return f.abort()
	}

	mode, proceed := f.conflictMode(conflictModeParams{SourcePath: f.paths[source], Target: dest, Selected: selected})
	if !proceed {
		return f.abort()
	}

	result, err := worktree.Extract(f.runCtx, domain.ExtractParams{
		SourcePath:   f.paths[source],
		SourceBranch: source,
		TargetPath:   dest.path,
		TargetBranch: dest.branch,
		Files:        selected,
		Keep:         answers.Value(KeyMode) == modeKeep,
		ConflictMode: mode,
	})
	if err != nil {
		return Outcome{}, err
	}
	result.Warnings = dest.warnings
	result.EnvPorts = dest.envPorts
	result.Isolation = worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: dest.branch})
	return f.conclude(Outcome{Result: result})
}

// resolveTarget is the one point a worktree comes into existence in this flow,
// before its hooks run — and where provisionOne publishes worktree.created.
func (f *extractFlow) resolveTarget(answers flow.Answers) (target, bool, error) {
	if !f.createsTarget(answers) {
		return f.existingTarget(answers.Value(KeyTarget))
	}
	created, proceed, err := f.create.Provision(create.ProvisionParams{Answers: answers, Prompter: f.prompter, Presenter: f.presenter})
	if err != nil || !proceed {
		return target{}, proceed, err
	}
	return target{path: created.Path, branch: created.Branch, envPorts: created.EnvPorts, warnings: created.Warnings}, true, nil
}

// existingTarget is a worktree the extraction did not create, so --isolation had
// nothing to answer; saying so is all that is left to do with it.
func (f *extractFlow) existingTarget(branch string) (target, bool, error) {
	wt, err := worktree.FindByBranch(f.runCtx, worktree.FindByBranchParams{ProjectDir: f.ctx.ProjectDir, Branch: branch})
	if err != nil {
		return target{}, false, err
	}
	dest := target{path: wt.Path, branch: wt.Branch}
	warning := rules.IsolationIgnoredWarning(rules.IsolationIgnoredParams{
		Branch:    wt.Branch,
		Requested: f.request.Isolation,
		Current:   worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: wt.Branch}),
	})
	warnings := rules.CreationFlagsIgnoredWarnings(rules.CreationFlagsIgnoredParams{Branch: wt.Branch, Given: f.creationFlags()})
	if warning != "" {
		warnings = append(warnings, warning)
	}
	for _, text := range warnings {
		f.presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: text})
	}
	dest.warnings = warnings
	return dest, true, nil
}

func (f *extractFlow) creationFlags() []string {
	var given []string
	if f.request.From != "" {
		given = append(given, domain.FlagFrom)
	}
	if f.request.FastForward {
		given = append(given, domain.FlagFF)
	}
	return given
}

type conflictModeParams struct {
	SourcePath string
	Target     target
	Selected   []domain.ExtractFile
}

// conflictMode is asked after the recap on purpose: the conflicts depend on the
// selection and on the disk, a target created a moment ago included. proceed is
// false when the user declined to write conflict markers.
func (f *extractFlow) conflictMode(params conflictModeParams) (mode string, proceed bool) {
	conflicts := worktree.ConflictingFiles(f.runCtx, domain.ConflictCheckParams{
		SourcePath: params.SourcePath,
		TargetPath: params.Target.path,
		Files:      params.Selected,
	})
	switch {
	case len(conflicts) == 0:
		return domain.OnConflictAbort, true
	case f.request.OnConflict != "":
		return f.request.OnConflict, true
	case !f.prompter.Interactive():
		return domain.OnConflictAbort, true
	}
	resolve, err := f.prompter.Confirm(flow.ConfirmParams{
		Title:       fmt.Sprintf(domain.ExtractConflictTitleFmt, params.Target.branch),
		Description: fmt.Sprintf(domain.ExtractConflictDescriptionFmt, joinConflicts(conflicts), params.Target.branch, params.Target.branch, params.Target.branch),
	})
	if err != nil || !resolve {
		return "", false
	}
	return domain.OnConflictResolve, true
}

func joinConflicts(files []string) string {
	if len(files) == 1 {
		return files[0] + " was"
	}
	return strings.Join(files, ", ") + " were"
}
