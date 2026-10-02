// Package exec runs the `wtm exec` flow.
package exec

import (
	"context"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/execsvc"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Branches []string
	All      bool
	Command  string
	Jobs     int
	Print    bool
	// Dir is where the user stands; the worktree holding it is pre-checked.
	Dir string
}

type Outcome struct {
	Command string
	Results []domain.ExecResult
	Elapsed time.Duration
	Aborted bool
}

type ExecProgress struct {
	Branches []string
	Beat     domain.ExecBeat
}

type Presenter interface {
	flow.Presenter
	Progress(ExecProgress)
	Executed(Outcome) error
}

type Params struct {
	Ctx       context.Context
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

type execFlow struct {
	params     Params
	candidates []domain.GitWorktree
	selection  []domain.GitWorktree
	current    string
}

func Run(params Params) (Outcome, error) {
	f := &execFlow{params: params}
	return f.run()
}

func (f *execFlow) run() (Outcome, error) {
	if err := f.load(); err != nil {
		return Outcome{}, err
	}

	answers, err := f.params.Prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) || (err == nil && answers.Value(KeyConfirm) == domain.WizardCancelValue) {
		f.params.Presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	targets, err := rules.ResolveExecTargets(rules.ResolveExecTargetsParams{Candidates: f.candidates, Names: answers.Values(KeySelection)})
	if err != nil {
		return Outcome{}, err
	}

	outcome := f.execute(executeParams{Targets: targets, Command: answers.Value(KeyCommand)})
	if err := f.params.Presenter.Executed(outcome); err != nil {
		return outcome, err
	}
	if len(rules.ExecFailedBranches(outcome.Results)) > 0 {
		return outcome, domain.ErrAborted
	}
	return outcome, nil
}

func (f *execFlow) load() error {
	candidates, err := worktree.ExecCandidates(worktree.ExecCandidatesParams{ProjectDir: f.params.Context.ProjectDir})
	if err != nil {
		return err
	}
	f.candidates = resolvedPaths(candidates)
	f.current = rules.ExecCurrent(rules.ExecCurrentParams{Candidates: f.candidates, Dir: flow.ResolveSymlinks(f.params.Request.Dir)})

	if len(f.params.Request.Branches) == 0 {
		return nil
	}
	selection, err := rules.ResolveExecTargets(rules.ResolveExecTargetsParams{Candidates: f.candidates, Names: f.params.Request.Branches})
	f.selection = selection
	return err
}

// resolvedPaths compares like with like: git spells /private/var where the
// shell may say /var.
func resolvedPaths(candidates []domain.GitWorktree) []domain.GitWorktree {
	resolved := make([]domain.GitWorktree, len(candidates))
	for i, candidate := range candidates {
		candidate.Path = flow.ResolveSymlinks(candidate.Path)
		resolved[i] = candidate
	}
	return resolved
}

type executeParams struct {
	Targets []domain.GitWorktree
	Command string
}

func (f *execFlow) execute(params executeParams) Outcome {
	targets := params.Targets
	branches := make([]string, len(targets))
	execTargets := make([]execsvc.Target, len(targets))
	for i, target := range targets {
		branches[i] = target.Branch
		execTargets[i] = execsvc.Target{
			Branch: target.Branch,
			Path:   target.Path,
			Env: worktree.ExecEnv(worktree.ExecEnvParams{
				Ref:          worktree.WorktreeRef{ProjectDir: f.params.Context.ProjectDir, StateDir: f.params.Context.StateDir, Branch: target.Branch},
				WorktreePath: target.Path,
			}),
			LogPath: rules.ExecLogPath(rules.ExecLogPathParams{StateDir: f.params.Context.StateDir, Branch: target.Branch}),
		}
	}

	begin := time.Now()
	results := execsvc.Run(f.params.Ctx, execsvc.RunParams{
		Command:    params.Command,
		Targets:    execTargets,
		Jobs:       f.params.Request.Jobs,
		KeepOutput: f.params.Request.Print,
		OnBeat: func(beat domain.ExecBeat) {
			f.params.Presenter.Progress(ExecProgress{Branches: branches, Beat: beat})
		},
	})
	return Outcome{Command: params.Command, Results: results, Elapsed: time.Since(begin)}
}
