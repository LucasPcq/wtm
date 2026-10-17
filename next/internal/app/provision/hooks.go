package provision

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/wtm/env"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

func (p *provisioning) runHooks(ctx context.Context, provisioned Provisioned) error {
	vars := env.HookVars{Worktree: p.Worktree.Path, Branch: p.Worktree.Branch, Root: p.Project.MainPath, FromBranch: p.Worktree.Source}
	hooks := make([]project.Hook, 0, len(p.Project.OnCreate))
	for _, configured := range p.Project.OnCreate {
		hooks = append(hooks, vars.Hook(configured))
	}
	return p.Deps.RunHooks(ctx, env.HookChain{
		Hooks:   hooks,
		WorkDir: p.Worktree.Path,
		Env:     p.hookEnv(provisioned.Ordinal),
		Unset:   env.JobKeys(),
		Output:  func(line string) { p.Report(wtm.Output{Subject: p.Worktree.Branch, Line: line}) },
		Log:     p.hookLog,
	})
}

func (p *provisioning) hookEnv(ordinal *int) map[string]string {
	if !p.hooksReadRun() || ordinal == nil {
		return nil
	}
	var compose [][]dotenv.Line
	for _, path := range env.ComposeEnvPaths(p.Project.Run) {
		lines, err := p.Deps.EnvFiles.Read(filepath.Join(p.Worktree.Path, path))
		if err != nil {
			p.warn(wtm.Warning{
				Code:    "provision.hook_env_unavailable",
				Message: err.Error() + " — on_create hooks run without the worktree's environment",
				Params:  wtm.Params{"cause": err.Error()},
			})
			return nil
		}
		compose = append(compose, lines)
	}
	return env.HookEnv(env.HookEnvParams{JobParams: p.jobParams(ordinal), ComposeEnv: compose})
}
