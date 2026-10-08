package runjobs

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// Held is where the apps of each runner up among jobs answer. The daemon is
// machine-wide, so each worktree is read against its own repository; one whose
// project or environment cannot be read is left out rather than guessed.
func Held(ctx context.Context, jobs []domain.JobInfo) domain.HeldAddresses {
	held := domain.HeldAddresses{}
	for dir, names := range rules.UpJobsByWorkDir(jobs) {
		if entries := heldIn(ctx, dir, names); len(entries) > 0 {
			held[dir] = entries
		}
	}
	return held
}

func heldIn(ctx context.Context, dir string, names []string) map[string][]domain.JobURLEntry {
	commonDir, err := infra.GitCommonDir(ctx, infra.GitCommonDirParams{Dir: dir})
	if err != nil {
		return nil
	}
	stateDir := filepath.Join(commonDir, domain.StateDirName)
	run, err := runconfig.Load(stateDir)
	if err != nil || !rules.AnyRunner(run, names) {
		return nil
	}
	cfg, err := config.Load(config.LoadParams{StateDir: stateDir})
	if err != nil {
		return nil
	}
	projectDir, err := worktree.MainCheckout(ctx, worktree.MainCheckoutParams{ProjectDir: dir})
	if err != nil {
		return nil
	}
	branch, err := worktree.CurrentBranch(ctx, worktree.CurrentBranchParams{Dir: dir})
	if err != nil {
		return nil
	}
	addresses := Addresses(ctx, AddressesParams{
		ProjectDir: projectDir,
		StateDir:   stateDir,
		Config:     run,
		Branches:   []string{branch},
		EnvFiles:   cfg.Project.Env.Files,
		Global:     cfg.Global,
	})
	return rules.HeldOf(rules.HeldOfParams{Addresses: addresses.ByBranch[branch], Jobs: names})
}
