package addressing

import (
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

const (
	stepMode   = "addressing"
	stepSettle = "settle"
	settleYes  = "yes"
	settleNo   = "no"
)

type SwitchPresenter interface {
	flow.Presenter
	Switched(SwitchOutcome) error
}

type SwitchRequest struct {
	// Mode is the positional; empty leaves the step to be asked.
	Mode    domain.Addressing
	KeepEnv bool
	Config  domain.RunConfig
}

type SwitchParams struct {
	Context   flow.Context
	Request   SwitchRequest
	Prompter  flow.Prompter
	Presenter SwitchPresenter
}

type SwitchOutcome struct {
	Previous domain.Addressing
	Current  domain.Addressing
	Changed  bool
	// Settled and Pending are branches: the worktrees whose .env this run moved
	// onto the addressing, and the ones it left out of step.
	Settled []string
	Pending []string
	Aborted bool
}

// Switch sets run.toml's addressing, then settles every worktree whose .env
// spells the other one. The second half runs even when the mode did not change:
// a worktree left out of step by an earlier switch is the same debt.
func Switch(params SwitchParams) (SwitchOutcome, error) {
	if err := validMode(params.Request.Mode); err != nil {
		return SwitchOutcome{}, err
	}

	previous := rules.EffectiveAddressing(params.Request.Config)
	pending := pendingByMode{ctx: params.Context, cache: map[string][]domain.GitWorktree{}}
	answers, err := params.Prompter.Ask(flow.Session{
		ErrLabel: domain.CmdAddressing,
		Steps:    []flow.Step{modeStep(previous), settleStep(&pending)},
		Presets:  presets(params.Request),
	})
	if errors.Is(err, domain.ErrUserAborted) {
		params.Presenter.Notice(flow.AbortedNotice)
		return SwitchOutcome{Aborted: true}, nil
	}
	if err != nil {
		return SwitchOutcome{}, err
	}

	outcome := SwitchOutcome{Previous: previous, Current: domain.Addressing(answers.Value(stepMode))}
	if outcome.Current != previous {
		cfg := params.Request.Config
		cfg.Addressing = outcome.Current
		if err := runconfig.Save(runconfig.SaveParams{StateDir: params.Context.StateDir, Config: cfg}); err != nil {
			return SwitchOutcome{}, err
		}
		outcome.Changed = true
	}

	worktrees := pending.of(string(outcome.Current))
	if answers.Value(stepSettle) != settleYes {
		outcome.Pending = branchesOf(worktrees)
		return outcome, params.Presenter.Switched(outcome)
	}
	outcome = settle(params, outcome, worktrees)
	return outcome, params.Presenter.Switched(outcome)
}

func validMode(mode domain.Addressing) error {
	switch mode {
	case "", domain.AddressingNames, domain.AddressingPorts:
		return nil
	}
	return fmt.Errorf(domain.AddressingInvalidFmt, mode, domain.AddressingNames, domain.AddressingPorts)
}

func presets(request SwitchRequest) flow.Answers {
	answers := flow.Answers{}
	if request.Mode != "" {
		answers = answers.With(stepMode, flow.Answer{Value: string(request.Mode)})
	}
	if request.KeepEnv {
		answers = answers.With(stepSettle, flow.Answer{Value: settleNo})
	}
	return answers
}

func modeStep(current domain.Addressing) flow.Step {
	options := make([]flow.Option, 0, 2)
	for _, mode := range rules.AddressingChoices(current) {
		options = append(options, flow.Option{Label: rules.AddressingLabel(mode), Value: string(mode)})
	}
	return flow.Step{
		Kind:        flow.StepSelect,
		Key:         stepMode,
		Label:       domain.AddressingStepName,
		Title:       domain.AddressingStepTitle,
		Description: domain.AddressingStepDesc,
		Options:     options,
		Arg:         true,
		Summarize:   func(answer flow.Answer) string { return answer.Value },
	}
}

// settleStep resolves to yes when nobody can be asked, as the port pass of a
// create does: the values rewritten are only the ones wtm links to an address,
// and switching back rewrites them again.
func settleStep(pending *pendingByMode) flow.Step {
	return flow.Step{
		Kind:  flow.StepSelect,
		Key:   stepSettle,
		Label: domain.AddressingSettleStepName,
		Flag:  domain.FlagKeepEnv,
		Skip: func(answers flow.Answers) (bool, string) {
			if len(pending.of(answers.Value(stepMode))) == 0 {
				return true, domain.AddressingSettleNothing
			}
			return false, ""
		},
		Build: func(answers flow.Answers) (flow.StepContent, error) {
			worktrees := pending.of(answers.Value(stepMode))
			count := rules.WorktreeCountLabel(len(worktrees))
			return flow.StepContent{
				Title:       fmt.Sprintf(domain.AddressingSettleTitleFmt, count),
				Description: fmt.Sprintf(domain.AddressingSettleDescFmt, count, strings.Join(branchesOf(worktrees), ", ")),
				Options: []flow.Option{
					{Label: domain.AddressingSettleYes, Value: settleYes},
					{Label: domain.AddressingSettleNo, Value: settleNo},
				},
			}, nil
		},
		Resolve: func(flow.Answers) (flow.Answer, error) {
			return flow.Answer{Value: settleYes}, nil
		},
		Summarize: func(answer flow.Answer) string { return answer.Value },
	}
}

func settle(params SwitchParams, outcome SwitchOutcome, worktrees []domain.GitWorktree) SwitchOutcome {
	once := &statusOnce{SwitchPresenter: params.Presenter, seen: map[string]bool{}}
	for _, wt := range worktrees {
		_, err := envports.Settle(envports.Params{
			Context:      params.Context,
			Branch:       wt.Branch,
			WorktreePath: wt.Path,
			Rewrite:      true,
			Presenter:    once,
		})
		if err != nil {
			params.Presenter.Status(flow.Notice{
				Kind: flow.NoticeWarning,
				Text: fmt.Sprintf(domain.AddressingSettleFailedFmt, wt.Branch, err),
			})
			outcome.Pending = append(outcome.Pending, wt.Branch)
			continue
		}
		outcome.Settled = append(outcome.Settled, wt.Branch)
	}
	return outcome
}

// statusOnce drops a notice already shown. The port pass reports what the
// machine does — the proxy's port in every address — once per worktree it
// settles, and a property of the machine is said once.
type statusOnce struct {
	SwitchPresenter
	seen map[string]bool
}

func (s *statusOnce) Status(notice flow.Notice) {
	key := notice.Text + "\x00" + strings.Join(notice.Lines, "\x00")
	if s.seen[key] {
		return
	}
	s.seen[key] = true
	s.SwitchPresenter.Status(notice)
}

// pendingByMode is the worktrees each mode would move, read once per mode: the
// step asks it to decide whether to show itself and again to title itself, and
// going back to change the mode must not read every .env a third time.
type pendingByMode struct {
	ctx   flow.Context
	cache map[string][]domain.GitWorktree
}

func (p *pendingByMode) of(mode string) []domain.GitWorktree {
	if cached, ok := p.cache[mode]; ok {
		return cached
	}
	found := outOfStep(p.ctx, domain.Addressing(mode))
	p.cache[mode] = found
	return found
}

// outOfStep reads each worktree's plan under the mode given rather than the one
// run.toml holds: the question is asked before anything is written.
func outOfStep(ctx flow.Context, mode domain.Addressing) []domain.GitWorktree {
	all, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: ctx.ProjectDir})
	if err != nil {
		return nil
	}
	var pending []domain.GitWorktree
	for _, wt := range all {
		if wt.Branch == "" {
			continue
		}
		plan, planErr := worktree.EnvPortPlanFor(worktree.ResolveEnvPortsParams{
			ProjectDir:   ctx.ProjectDir,
			StateDir:     ctx.StateDir,
			Branch:       wt.Branch,
			WorktreePath: wt.Path,
			EnvFiles:     ctx.Config.Project.Env.Files,
			Global:       ctx.Config.Global,
			Addressing:   mode,
		})
		if planErr != nil || len(rules.EnvPortRewrites(plan)) == 0 {
			continue
		}
		pending = append(pending, wt)
	}
	return pending
}

func branchesOf(worktrees []domain.GitWorktree) []string {
	branches := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		branches = append(branches, wt.Branch)
	}
	return branches
}
