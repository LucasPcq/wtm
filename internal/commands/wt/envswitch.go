package wt

import (
	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type planSwitchParams struct {
	cfg       shared.ConfigResult
	target    envTarget
	ctx       envContext
	isolation domain.Isolation
}

// envSwitch is a `wtm env` run settling a worktree on an isolation. It is
// recorded last, once the .env is in line with it: a run that fails or is
// cancelled leaves the worktree recorded as it was.
type envSwitch struct {
	ref       worktree.WorktreeRef
	isolation domain.Isolation
	restore   envsvc.OwnedRestoreParams
	planned   []domain.EnvRestoredEntry
	// warning names why the switch cannot be made: the keys wtm owns are read
	// from run.toml, and one it cannot read leaves nothing to put back.
	warning string
}

func planSwitch(params planSwitchParams) (envSwitch, error) {
	sw := envSwitch{ref: worktreeRef(params.cfg, params.target.branch), isolation: params.isolation}
	if !rules.IsVerbatim(params.isolation) {
		return sw, nil
	}

	keys, err := worktree.OwnedEnvKeys(worktree.OwnedEnvKeysParams{StateDir: params.cfg.StateDir, EnvFiles: params.cfg.Config.Project.Env.Files})
	if err != nil {
		sw.warning = rules.IsolationNotSwitchedWarning(rules.IsolationNotSwitchedParams{Branch: params.target.branch, Isolation: params.isolation, Cause: err.Error()})
		return sw, nil
	}
	sw.restore = envsvc.OwnedRestoreParams{
		MainPath:           params.cfg.ProjectDir,
		WorktreePath:       params.target.path,
		ParentWorktreePath: params.ctx.ParentPath,
		Strategy:           params.ctx.Strategy,
		Files:              params.cfg.Config.Project.Env.Files,
		Keys:               keys,
	}
	sw.planned, err = envsvc.PlanOwnedRestore(sw.restore)
	return sw, err
}

func (s envSwitch) keys() []string {
	return restoredKeys(s.planned)
}

// envSwitchOutcome is what the switch did, for the report.
type envSwitchOutcome struct {
	restored []domain.EnvRestoredEntry
	changed  bool
	warning  string
}

func (s envSwitch) apply() (envSwitchOutcome, error) {
	if s.isolation == "" || s.warning != "" {
		return envSwitchOutcome{warning: s.warning}, nil
	}

	var outcome envSwitchOutcome
	if rules.IsVerbatim(s.isolation) {
		restored, err := envsvc.ApplyOwnedRestore(s.restore)
		if err != nil {
			return envSwitchOutcome{}, err
		}
		outcome.restored = restored
	}

	before := worktree.RecordedIsolation(s.ref)
	if err := worktree.SetIsolation(worktree.SetIsolationParams{Ref: s.ref, Isolation: s.isolation}); err != nil {
		return envSwitchOutcome{}, err
	}
	outcome.changed = worktree.RecordedIsolation(s.ref) != before
	return outcome, nil
}

func (o envSwitchOutcome) decorate(result domain.EnvSyncResult) domain.EnvSyncResult {
	result.Restored = o.restored
	result.IsolationChanged = o.changed
	if o.warning != "" {
		result.Warnings = append(result.Warnings, o.warning)
	}
	return result
}

// checkIsolation refuses an isolation the worktree cannot take before anything
// is written for it.
func checkIsolation(cfg shared.ConfigResult, target envTarget, isolation domain.Isolation) error {
	if isolation == "" {
		return nil
	}
	return worktree.CheckIsolation(worktree.SetIsolationParams{Ref: worktreeRef(cfg, target.branch), Isolation: isolation})
}

func restoredKeys(entries []domain.EnvRestoredEntry) []string {
	keys := make([]string, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry.Key)
	}
	return keys
}
