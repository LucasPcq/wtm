package env

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// branchScan is what the wizard shows of one worktree before anything is
// written: its drift, the port pass that rides along with the apply, and what
// a switch to verbatim puts back.
type branchScan struct {
	files   []domain.EnvFileResult
	ports   domain.EnvPortPlan
	restore []domain.EnvRestoredEntry
}

// scanKey is one worktree under one answer to each mode step: what the recap
// shows follows those answers.
type scanKey struct {
	branch     string
	isolation  domain.Isolation
	addressing domain.Addressing
}

// pickerKey is the scan the picker badges a worktree with: the run as its
// flags ask for it, or as it is for a worktree that refuses them.
func (f *envFlow) pickerKey(branch string) scanKey {
	if f.refused[branch] != "" {
		return scanKey{branch: branch}
	}
	return scanKey{branch: branch, isolation: f.request.Isolation, addressing: f.request.Addressing}
}

// scan runs once, before the first screen, over every worktree the picker may
// offer — the list badges each one with its drift.
func (f *envFlow) scan() error {
	statuses, err := worktree.List(domain.ListParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Config:     f.ctx.Config,
	})
	if err != nil {
		return fmt.Errorf("list worktrees: %w", err)
	}
	f.statuses = statuses

	preset := f.request.Worktree
	branches := branchNames(statuses)
	if preset != "" {
		if pathOf(statuses, preset) == "" {
			return fmt.Errorf("%w: %s", domain.ErrWorktreeNotFound, preset)
		}
		if err := f.checkFlags(target{branch: preset, path: pathOf(statuses, preset)}); err != nil {
			return err
		}
		branches = []string{preset}
	}

	for _, branch := range branches {
		if err := f.checkFlags(target{branch: branch, path: pathOf(statuses, branch)}); err != nil {
			refused := f.refusedFlag(err)
			if refused == "" {
				return err
			}
			f.refused[branch] = refused
		}
		if _, err := f.scanFor(f.pickerKey(branch)); err != nil {
			return err
		}
	}
	return nil
}

func (f *envFlow) scanFor(key scanKey) (branchScan, error) {
	if scan, ok := f.scans[key]; ok {
		return scan, nil
	}
	scan, err := f.scanBranch(key)
	if err != nil {
		return branchScan{}, err
	}
	f.scans[key] = scan
	return scan, nil
}

func (f *envFlow) scanBranch(key scanKey) (branchScan, error) {
	t, err := f.answeredTarget(key.branch)
	if err != nil {
		return branchScan{}, err
	}
	state, err := f.stateOf(t)
	if err != nil {
		return branchScan{}, err
	}
	addressing, err := f.settledAddressing(t, key.addressing)
	if err != nil {
		return branchScan{}, err
	}
	ctx := f.envContext(key.branch)
	preview, err := f.planSwitch(planSwitchParams{Target: t, Ctx: ctx, Isolation: key.isolation})
	if err != nil {
		return branchScan{}, err
	}

	// A worktree that has not adopted isolation, or is about to take ports of
	// its own, is scanned without its port pass: resolving one allocates an
	// ordinal, and whether it gets one is what the run is deciding.
	var ports envsvc.EnvPortsParams
	if !state.adoption.Pending && !state.movesOntoIsolation(key.isolation) {
		ports, _ = f.resolvePorts(resolvePortsParams{Target: t, Isolation: key.isolation, Addressing: addressing})
	}

	reserved := preview.keys()
	if state.adoption.Pending {
		reserved = append(reserved, domain.WtmOwnedEnvKeys...)
	}
	files, err := envsvc.ComputeEnvDiff(envsvc.ComputeEnvParams{
		Branch:             key.branch,
		MainPath:           f.ctx.ProjectDir,
		WorktreePath:       t.path,
		ParentWorktreePath: ctx.parentPath,
		ParentBranch:       ctx.parentBranch,
		Files:              f.ctx.Config.Project.Env.Files,
		Strategy:           ctx.strategy,
		Mode:               f.request.Mode,
		Ports:              ports,
		Reserved:           reserved,
	})
	if err != nil {
		return branchScan{}, err
	}

	scan := branchScan{files: files, restore: preview.planned}
	if !ports.Empty() {
		if scan.ports, err = envsvc.ComputeEnvPorts(ports); err != nil {
			return branchScan{}, err
		}
	}
	return scan, nil
}

func branchNames(statuses []domain.WorktreeStatus) []string {
	out := make([]string, 0, len(statuses))
	for _, s := range statuses {
		out = append(out, s.Branch)
	}
	return out
}
