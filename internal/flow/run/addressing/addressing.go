// Package addressing is `wtm run addressing` — switching the mode and settling
// the .env files on it — and the warning the run flows put under the URLs they hand
// out: the worktrees whose .env still answers on ports while their jobs are
// published by name.
package addressing

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	Context  flow.Context
	WorkDirs []string
}

// State is what one reading of the worktrees' .env files tells a run: which of
// them are answering on their ports rather than on the names they publish, and
// the lines saying so. Both come from the same plan, so a surface can never
// hand out an address one half of this disagrees with.
type State struct {
	// PortAddressed keys the worktrees whose .env still spells ports, by the
	// work dir a surface holds them under.
	PortAddressed map[string]bool
	// Warnings is one line per unsettled worktree, and the reason once. Empty
	// when every worktree of the run is settled.
	Warnings []string
}

// Read resolves both in one pass over the worktrees' .env files.
func Read(params Params) State {
	found := drifts(params)
	lines := make([]rules.AddressingDriftParams, 0, len(found))
	for _, drift := range found {
		lines = append(lines, drift.AddressingDriftParams)
	}

	state := State{Warnings: rules.AddressingDriftLines(lines)}
	for _, drift := range found {
		if rules.AddressedByPort(drift.Plan) {
			if state.PortAddressed == nil {
				state.PortAddressed = map[string]bool{}
			}
			state.PortAddressed[drift.WorkDir] = true
		}
	}
	return state
}

// Lines are the warning as a surface that draws its own band takes it: the run
// view shows them while it holds the terminal, where a notice printed after it
// reaches a reader who has already followed the URL.
func Lines(params Params) []string {
	return Read(params).Warnings
}

// Notice is what a surface shows, and false when every worktree of the run is
// settled. Anything that stops a worktree's plan from being read — an invalid
// run.toml, a link naming a file .wtm.toml does not configure, an unreadable
// .env — yields no warning rather than an error: this is about an address, the
// run is about processes.
func Notice(params Params) (flow.Notice, bool) {
	lines := Lines(params)
	if len(lines) == 0 {
		return flow.Notice{}, false
	}
	return flow.Notice{
		Kind:  flow.NoticeWarning,
		Text:  domain.AddressingDriftTitle,
		Lines: lines,
	}, true
}

// drift pairs a worktree's plan with the work dir a surface knows it by, which
// the branch alone cannot answer for.
type drift struct {
	rules.AddressingDriftParams
	WorkDir string
}

func drifts(params Params) []drift {
	drifts := make([]drift, 0, len(params.WorkDirs))
	for _, dir := range params.WorkDirs {
		branch := target.BranchOf(dir)
		if branch == "" {
			continue
		}
		plan, err := planOf(planOfParams{Context: params.Context, Branch: branch, Path: dir})
		if err != nil {
			continue
		}
		drifts = append(drifts, drift{
			AddressingDriftParams: rules.AddressingDriftParams{Worktree: branch, Plan: plan},
			WorkDir:               dir,
		})
	}
	return drifts
}

type planOfParams struct {
	Context    flow.Context
	Branch     string
	Path       string
	Addressing domain.Addressing
}

// planOf numbers the worktree the first time its ports are planned, as reading
// them always did, so the allocation is published.
func planOf(params planOfParams) (domain.EnvPortPlan, error) {
	var plan domain.EnvPortPlan
	err := ordinal.Retry(ordinal.RetryParams{
		Context: params.Context,
		Branch:  params.Branch,
		Do: func() error {
			resolved, planErr := worktree.EnvPortPlanFor(worktree.ResolveEnvPortsParams{
				ProjectDir:   params.Context.ProjectDir,
				StateDir:     params.Context.StateDir,
				Branch:       params.Branch,
				WorktreePath: params.Path,
				EnvFiles:     params.Context.Config.Project.Env.Files,
				Global:       params.Context.Config.Global,
				Addressing:   params.Addressing,
			})
			plan = resolved
			return planErr
		},
	})
	return plan, err
}
