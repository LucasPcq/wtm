package provision

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/wtm/env"

	"github.com/LucasPcq/wtm/next/internal/wtm"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

func (p *provisioning) add(ctx context.Context) (Provisioned, *wtm.Error) {
	shielded, release := p.Deps.Shield(ctx)
	defer release()
	if err := p.do(shielded, phase{name: "worktree", run: p.addWorktree}); err != nil {
		return Provisioned{}, stepFailed("worktree", err)
	}
	meta := p.meta()
	if err := p.do(shielded, phase{name: "env", run: p.copyEnv}); err != nil {
		return Provisioned{}, p.undo(shielded, stepFailed("env", err))
	}
	if err := p.do(shielded, phase{name: "meta", run: func(ctx context.Context) error { return p.record(ctx, meta) }}); err != nil {
		return Provisioned{}, p.undo(shielded, stepFailed("meta", err))
	}
	return Provisioned{Meta: meta}, nil
}

func stepFailed(step string, err error) *wtm.Error {
	return wtm.AsError(wtm.AsErrorParams{Err: err, Code: "unit.step_failed", Message: step + " failed", Params: wtm.Params{"step": step}})
}

func (p *provisioning) addWorktree(ctx context.Context) error {
	return p.Deps.Git.AddWorktree(ctx, worktree.Addition{Path: p.Worktree.Path, Checkout: p.Worktree.Checkout})
}

func (p *provisioning) meta() worktree.Meta {
	return worktree.Meta{
		SourceBranch: p.Worktree.Source,
		CreatedAt:    p.Deps.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  p.Worktree.Strategy,
		Isolation:    p.Worktree.Isolation,
	}
}

func (p *provisioning) copyEnv(ctx context.Context) error {
	if len(p.Project.EnvFiles) == 0 {
		return nil
	}
	copied, err := p.Deps.EnvFiles.Copy(ctx, env.CopyRequest{
		Strategy:   p.Worktree.Strategy,
		Files:      p.Project.EnvFiles,
		TargetDir:  p.Worktree.Path,
		MainPath:   p.Project.MainPath,
		ParentPath: p.Worktree.ParentPath,
	})
	if err != nil {
		return err
	}
	for _, warning := range copied.Warnings {
		p.warn(copyWarning(warning))
	}
	p.unlocked = p.unlocked || copied.Unlocked
	return nil
}

func copyWarning(warning env.CopyWarning) wtm.Warning {
	switch w := warning.(type) {
	case env.NoTemplate:
		return wtm.Warning{Code: "provision.env_no_template", Message: "no template found for " + w.Target + ", skipping", Params: wtm.Params{"target": w.Target}}
	case env.SourceMissing:
		return wtm.Warning{Code: "provision.env_source_missing", Message: "file " + w.Target + " not found in " + w.Dir + ", skipping", Params: wtm.Params{"target": w.Target, "dir": w.Dir}}
	case env.ParentFallback:
		return wtm.Warning{Code: "provision.env_parent_fallback", Message: "file " + w.Target + " not found in the parent worktree, copied from main", Params: wtm.Params{"target": w.Target}}
	}
	return wtm.Warning{}
}

func (p *provisioning) record(ctx context.Context, meta worktree.Meta) error {
	written, err := p.Deps.Meta.Update(ctx, worktree.MetaUpdate{
		Branch: p.Worktree.Branch,
		Change: func(stored *worktree.Meta) error {
			*stored = meta
			return nil
		},
	})
	p.unlocked = p.unlocked || written.Unlocked
	return err
}
