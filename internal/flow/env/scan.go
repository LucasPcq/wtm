package env

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// branchScan is what the wizard shows of one worktree before anything is
// written: its drift, the port pass that rides along with the apply, whether it
// still has to adopt its isolation, and what keeping it verbatim puts back.
type branchScan struct {
	files    []domain.EnvFileResult
	ports    domain.EnvPortPlan
	adoption domain.IsolationAdoptionPlan
	restore  []domain.EnvRestoredEntry
	// refused is --isolation turned down for this worktree: the picker offers it
	// disabled rather than letting the apply fail on it.
	refused bool
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
		if err := f.checkIsolation(target{branch: preset}, f.request.Isolation); err != nil {
			return err
		}
		branches = []string{preset}
	}

	for _, branch := range branches {
		scan, err := f.scanBranch(branch)
		if err != nil {
			return err
		}
		f.scans[branch] = scan
	}
	return nil
}

func (f *envFlow) scanBranch(branch string) (branchScan, error) {
	t := target{branch: branch, path: pathOf(f.statuses, branch)}
	adoption, err := f.adoption(t)
	if err != nil {
		return branchScan{}, err
	}
	ctx := f.envContext(branch)
	preview, err := f.planSwitch(planSwitchParams{Target: t, Ctx: ctx, Isolation: domain.IsolationVerbatim})
	if err != nil {
		return branchScan{}, err
	}

	isolation := f.request.Isolation
	refused := f.checkIsolation(t, isolation) != nil
	if refused {
		isolation = ""
	}
	// A worktree still to adopt its isolation is scanned without its port
	// pass: resolving one allocates an ordinal, and whether it gets one is the
	// question the wizard is about to ask.
	var ports envsvc.EnvPortsParams
	if !adoption.Pending || isolation != "" {
		ports, _ = f.resolvePorts(resolvePortsParams{Target: t, Isolation: isolation})
	}

	var reserved []string
	if rules.IsVerbatim(isolation) {
		reserved = restoredKeys(preview.planned)
	}
	if adoption.Pending {
		reserved = append(reserved, domain.WtmOwnedEnvKeys...)
	}
	files, err := envsvc.ComputeEnvDiff(envsvc.ComputeEnvParams{
		Branch:             branch,
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

	scan := branchScan{files: files, adoption: adoption, restore: preview.planned, refused: refused}
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
