package env

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type runPassParams struct {
	Target   target
	Adoption domain.IsolationAdoptionPlan
	// Isolation is the one this run settles the worktree on, empty to keep the
	// recorded one.
	Isolation domain.Isolation
	Reserved  []string
}

// envRunPass is the run half of a `wtm env`: the port and owned-value pass, and
// what it says about the worktree's isolation.
type envRunPass struct {
	ports    envsvc.EnvPortsParams
	reserved []string
	warnings []string
	adoption domain.IsolationAdoption
	branch   string
}

// runPass settles the run values of a worktree that chose its isolation, and
// leaves a worktree that never did exactly as it is: no port moved, no compose
// project written, no ordinal allocated — only the keys are reconciled.
func (f *envFlow) runPass(params runPassParams) envRunPass {
	pass := envRunPass{branch: params.Target.branch, reserved: params.Reserved}
	if params.Adoption.Pending {
		pass.reserved = append(pass.reserved, domain.WtmOwnedEnvKeys...)
	}
	if params.Adoption.Pending && params.Isolation == "" {
		pass.warnings = []string{rules.IsolationNotAdoptedWarning(params.Target.branch)}
		pass.adoption = domain.IsolationNotAdopted
		return pass
	}
	if params.Adoption.Pending {
		pass.adoption = domain.IsolationAdopted
	}
	if rules.IsVerbatim(params.Isolation) {
		return pass
	}
	pass.ports, pass.warnings = f.resolvePorts(resolvePortsParams{Target: params.Target, Isolation: params.Isolation})
	return pass
}

// decorate reports the pass on the result. A worktree left on its source's
// values has no isolation to report: calling it isolated would be the one
// thing it is not.
func (p envRunPass) decorate(ctx flow.Context, result domain.EnvSyncResult) domain.EnvSyncResult {
	result.Warnings = append(result.Warnings, p.warnings...)
	result.IsolationAdoption = p.adoption
	if p.adoption != domain.IsolationNotAdopted {
		result.Isolation = worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: ctx.ProjectDir, StateDir: ctx.StateDir, Branch: p.branch})
	}
	return result
}

type resolvePortsParams struct {
	Target    target
	Isolation domain.Isolation
}

// resolvePorts gathers the [[env_port]] links and the offset this worktree
// binds on. A run.toml that cannot be used skips only the port pass, and the
// warning says why: the keys never depend on it.
func (f *envFlow) resolvePorts(params resolvePortsParams) (envsvc.EnvPortsParams, []string) {
	branch := params.Target.branch
	if err := runconfig.Check(runconfig.CheckParams{StateDir: f.ctx.StateDir, EnvFiles: f.ctx.Config.Project.Env.Files}); err != nil {
		return envsvc.EnvPortsParams{}, []string{rules.PortsNotSettledWarning(rules.PortsNotSettledWarningParams{
			Cause:                 err.Error(),
			PortsNotSettledParams: rules.PortsNotSettledParams{Branch: branch, RunConfig: true},
		})}
	}
	ports, err := worktree.ResolveEnvPorts(worktree.ResolveEnvPortsParams{
		ProjectDir:   f.ctx.ProjectDir,
		StateDir:     f.ctx.StateDir,
		Branch:       branch,
		WorktreePath: params.Target.path,
		EnvFiles:     f.ctx.Config.Project.Env.Files,
		Global:       f.ctx.Config.Global,
		Isolation:    params.Isolation,
	})
	if err != nil {
		return envsvc.EnvPortsParams{}, []string{rules.PortsNotSettledWarning(rules.PortsNotSettledWarningParams{
			Cause:                 err.Error(),
			PortsNotSettledParams: rules.PortsNotSettledParams{Branch: branch},
		})}
	}
	return ports, nil
}

type planSwitchParams struct {
	Target    target
	Ctx       envContext
	Isolation domain.Isolation
}

// envSwitch is a run settling a worktree on an isolation.
type envSwitch struct {
	ref       worktree.WorktreeRef
	isolation domain.Isolation
	restore   envsvc.OwnedRestoreParams
	planned   []domain.EnvRestoredEntry
	// warning names why the switch cannot be made: the keys wtm owns are read
	// from run.toml, and one it cannot read leaves nothing to put back.
	warning string
}

func (f *envFlow) planSwitch(params planSwitchParams) (envSwitch, error) {
	sw := envSwitch{ref: f.ref(params.Target.branch), isolation: params.Isolation}
	if !rules.IsVerbatim(params.Isolation) {
		return sw, nil
	}

	keys, err := worktree.OwnedEnvKeys(worktree.OwnedEnvKeysParams{StateDir: f.ctx.StateDir, EnvFiles: f.ctx.Config.Project.Env.Files})
	if err != nil {
		sw.warning = rules.IsolationNotSwitchedWarning(rules.IsolationNotSwitchedParams{Branch: params.Target.branch, Isolation: params.Isolation, Cause: err.Error()})
		return sw, nil
	}
	sw.restore = envsvc.OwnedRestoreParams{
		MainPath:           f.ctx.ProjectDir,
		WorktreePath:       params.Target.path,
		ParentWorktreePath: params.Ctx.parentPath,
		Strategy:           params.Ctx.strategy,
		Files:              f.ctx.Config.Project.Env.Files,
		Keys:               keys,
	}
	sw.planned, err = envsvc.PlanOwnedRestore(sw.restore)
	return sw, err
}

func (s envSwitch) keys() []string {
	return restoredKeys(s.planned)
}

type switchOutcome struct {
	restored []domain.EnvRestoredEntry
	changed  bool
	warning  string
}

func (o switchOutcome) decorate(result domain.EnvSyncResult) domain.EnvSyncResult {
	result.Restored = o.restored
	result.IsolationChanged = o.changed
	if o.warning != "" {
		result.Warnings = append(result.Warnings, o.warning)
	}
	return result
}

func restoredKeys(entries []domain.EnvRestoredEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}
