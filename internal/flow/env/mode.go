package env

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// modeState is how a worktree runs today: what the isolation and addressing
// steps offer to keep, and whether either has anything to offer.
type modeState struct {
	isMain   bool
	applies  bool
	recorded domain.Isolation
	adoption domain.IsolationAdoptionPlan
	// project is run.toml's addressing, empty when it cannot be read; current is
	// the mode the main checkout's .env spells, and matters says the two modes
	// would write it differently.
	project domain.Addressing
	current domain.Addressing
	matters bool
}

func (s modeState) asksIsolation() bool {
	return !s.isMain && s.applies
}

func (s modeState) asksAddressing() bool {
	return s.isMain && s.matters
}

// movesOntoIsolation is a run giving the worktree ports of its own it did not
// have: resolving them before the apply would allocate its ordinal on a
// preview.
func (s modeState) movesOntoIsolation(isolation domain.Isolation) bool {
	if isolation != domain.IsolationIsolated {
		return false
	}
	return s.adoption.Pending || rules.IsVerbatim(s.recorded)
}

func (f *envFlow) stateOf(t target) (modeState, error) {
	if state, ok := f.modes[t.branch]; ok {
		return state, nil
	}
	state, err := f.readMode(t)
	if err != nil {
		return modeState{}, err
	}
	f.modes[t.branch] = state
	return state, nil
}

func (f *envFlow) answeredState(answers flow.Answers) (modeState, error) {
	t, err := f.answeredTarget(answers.Value(KeyWorktree))
	if err != nil {
		return modeState{}, err
	}
	return f.stateOf(t)
}

func (f *envFlow) readMode(t target) (modeState, error) {
	ref := f.ref(t.branch)
	isMain, err := worktree.IsMain(f.runCtx, ref)
	if err != nil {
		return modeState{}, err
	}
	adoption, err := f.adoption(t)
	if err != nil {
		return modeState{}, err
	}
	state := modeState{isMain: isMain, recorded: worktree.RecordedIsolation(ref), adoption: adoption}

	// An unreadable run.toml asks nothing: the port pass is skipped and its
	// warning says why.
	cfg, err := runconfig.Load(f.ctx.StateDir)
	if err != nil {
		return state, nil
	}
	state.applies = rules.IsolationApplies(cfg)
	state.project = rules.EffectiveAddressing(cfg)
	state.current = state.project
	if !isMain || state.project != domain.AddressingNames {
		return state, nil
	}

	names, namesErr := f.planUnder(planUnderParams{Target: t, Addressing: domain.AddressingNames})
	ports, portsErr := f.planUnder(planUnderParams{Target: t, Addressing: domain.AddressingPorts})
	if namesErr != nil || portsErr != nil {
		return state, nil
	}
	state.current, state.matters = rules.MainAddressing(rules.MainAddressingParams{Names: names, Ports: ports})
	return state, nil
}

type planUnderParams struct {
	Target     target
	Addressing domain.Addressing
}

func (f *envFlow) planUnder(params planUnderParams) (domain.EnvPortPlan, error) {
	var plan domain.EnvPortPlan
	err := ordinal.Retry(f.runCtx, ordinal.RetryParams{
		Context: f.ctx,
		Branch:  func() string { return params.Target.branch },
		Do: func() error {
			resolved, planErr := worktree.EnvPortPlanFor(f.runCtx, worktree.ResolveEnvPortsParams{
				ProjectDir:   f.ctx.ProjectDir,
				StateDir:     f.ctx.StateDir,
				Branch:       params.Target.branch,
				WorktreePath: params.Target.path,
				EnvFiles:     f.ctx.Config.Project.Env.Files,
				Global:       f.ctx.Config.Global,
				Addressing:   params.Addressing,
			})
			plan = resolved
			return planErr
		},
	})
	return plan, err
}

// settledAddressing is the mode the run writes the worktree's linked values
// in: the one asked for, else, on the main checkout, the one its .env already
// spells. Empty is run.toml's, which every other worktree follows.
func (f *envFlow) settledAddressing(t target, requested domain.Addressing) (domain.Addressing, error) {
	state, err := f.stateOf(t)
	if err != nil {
		return "", err
	}
	if err := rules.ValidateEnvAddressing(rules.EnvAddressingParams{Requested: requested, Project: state.project, IsMain: state.isMain}); err != nil {
		return "", err
	}
	if requested != "" || !state.isMain {
		return requested, nil
	}
	return state.current, nil
}

// checkFlags refuses a flag the worktree cannot take, before anything is
// written for it.
func (f *envFlow) checkFlags(t target) error {
	if err := f.checkIsolation(t, f.request.Isolation); err != nil {
		return err
	}
	_, err := f.settledAddressing(t, f.request.Addressing)
	return err
}

// refusedFlag is the value a picker badges a worktree with when checkFlags
// turned it down, empty for any other failure.
func (f *envFlow) refusedFlag(err error) string {
	switch {
	case errors.Is(err, domain.ErrIsolationMain):
		return string(f.request.Isolation)
	case errors.Is(err, domain.ErrEnvAddressingMainOnly), errors.Is(err, domain.ErrEnvAddressingProjectPorts):
		return string(f.request.Addressing)
	}
	return ""
}
