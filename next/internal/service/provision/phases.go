package provision

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

var errOrdinalUnreadable = errors.New("cannot read a live worktree's ordinal")

func (p *provisioning) needsOrdinal() bool {
	return p.Project.RunErr == nil && (p.portsNeedOrdinal() || p.hooksReadRun())
}

func (p *provisioning) portsNeedOrdinal() bool {
	return env.NeedsOrdinal(env.NeedsOrdinalParams{Isolation: p.Worktree.Isolation, Run: p.kept})
}

func (p *provisioning) hooksReadRun() bool {
	return len(p.Project.OnCreate) > 0 && env.HooksReadRun(env.HooksReadRunParams{Run: p.Project.Run, Isolation: p.Worktree.Isolation})
}

func (p *provisioning) number(ctx context.Context, provisioned *Provisioned) error {
	ordinal, err := p.allocate(ctx)
	if err != nil && ctx.Err() != nil {
		return err
	}
	if err != nil {
		if p.portsNeedOrdinal() {
			p.warn(p.notSettled(notSettled{Cause: err}))
		}
		return nil
	}
	provisioned.Ordinal = &ordinal
	return nil
}

func (p *provisioning) allocate(ctx context.Context) (int, error) {
	var ordinal int
	held, err := p.Deps.Meta.LockOrdinals(ctx, func() error {
		taken, err := p.taken(ctx)
		if err != nil {
			return err
		}
		ordinal = worktree.FreeOrdinal(taken)
		written, err := p.Deps.Meta.Update(ctx, worktree.MetaUpdate{Branch: p.Worktree.Branch, Change: func(meta *worktree.Meta) error {
			meta.Ordinal = ordinal
			return nil
		}})
		p.unlocked = p.unlocked || written.Unlocked
		return err
	})
	p.unlocked = p.unlocked || !held
	return ordinal, err
}

func (p *provisioning) taken(ctx context.Context) ([]int, error) {
	worktrees, err := p.Deps.Git.Worktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}
	var taken []int
	for _, listed := range worktrees {
		branch := listed.Branch()
		if listed.Main || branch == "" || branch == p.Worktree.Branch {
			continue
		}
		meta, err := p.Deps.Meta.Load(branch)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %s: %w", errOrdinalUnreadable, branch, err)
		}
		taken = append(taken, meta.Ordinal)
	}
	return taken, nil
}

func (p *provisioning) settlesPorts() bool {
	switch {
	case p.Project.RunErr != nil || len(p.ignored) > 0:
		return true
	case p.Worktree.Isolation == project.Verbatim:
		return false
	case len(p.kept.EnvPorts) > 0 || len(p.kept.EnvValues) > 0:
		return true
	}
	return len(env.OwnedTargets(env.OwnedTargetsParams{Run: p.kept, Files: p.Project.EnvFiles})) > 0
}

func (p *provisioning) settle(ctx context.Context, provisioned *Provisioned) error {
	if p.Project.RunErr != nil {
		p.warn(p.notSettled(notSettled{Cause: p.Project.RunErr, InRunToml: true}))
		return nil
	}
	for _, link := range p.ignored {
		p.warn(ignoredWarning(link))
	}
	if p.Worktree.Isolation == project.Verbatim || p.portsNeedOrdinal() && provisioned.Ordinal == nil {
		return nil
	}
	writes, err := env.Resolve(env.ResolveParams{
		JobParams:  p.jobParams(provisioned.Ordinal),
		Files:      p.Project.EnvFiles,
		Proxy:      p.Project.Global.Proxy,
		PublicPort: p.Deps.PublicPort(ctx, p.Project.Global.ProxyPort()),
	})
	if _, inRunToml := errors.AsType[*env.ValueError](err); inRunToml {
		p.warn(p.notSettled(notSettled{Cause: err, InRunToml: true}))
		return nil
	}
	if err != nil {
		p.warn(p.notSettled(notSettled{Cause: err}))
		return nil
	}
	plan, err := p.apply(ctx, writes)
	if err != nil && ctx.Err() != nil {
		return err
	}
	if err != nil {
		p.warn(p.notSettled(notSettled{Cause: err}))
		return nil
	}
	provisioned.EnvPorts = env.Redact(plan)
	return nil
}

func (p *provisioning) jobParams(ordinal *int) env.JobParams {
	params := env.JobParams{Branch: p.Worktree.Branch, ProjectDir: p.Project.MainPath, Isolation: p.Worktree.Isolation, Run: p.Project.Run}
	if ordinal != nil {
		params.Ordinal = *ordinal
	}
	return params
}

func (p *provisioning) apply(ctx context.Context, writes env.Writes) (env.Plan, error) {
	read := map[string][]dotenv.Line{}
	for _, file := range writes.Files() {
		written, err := p.Deps.EnvFiles.Edit(ctx, dotenv.Edit{
			Path:   filepath.Join(p.Worktree.Path, file),
			Create: writes.Creates(file),
			Edit: func(lines []dotenv.Line) []dotenv.Line {
				read[file] = lines
				return writes.Edit(env.EditParams{File: file, Lines: lines})
			},
		})
		if err != nil {
			return env.Plan{}, err
		}
		p.unlocked = p.unlocked || written.Unlocked
	}
	return writes.Settled(read), nil
}

type notSettled struct {
	Cause     error
	InRunToml bool
}

func (p *provisioning) notSettled(n notSettled) domain.Warning {
	code, fix := "provision.ports_not_settled", "this"
	if n.InRunToml {
		code, fix = "provision.ports_not_settled_run", "run.toml"
	}
	branch, cause := p.Worktree.Branch, n.Cause.Error()
	return domain.Warning{
		Code:    code,
		Message: cause + " — ports not settled — run `wtm env " + branch + "` once " + fix + " is fixed",
		Params:  domain.Params{"branch": branch, "cause": cause},
	}
}

func ignoredWarning(link env.Ignored) domain.Warning {
	code := "provision.env_port_ignored"
	if link.Kind == env.LinkValue {
		code = "provision.env_value_ignored"
	}
	return domain.Warning{
		Code:    code,
		Message: string(link.Kind) + " " + link.Key + " in " + link.File + " ignored: not a configured env file — add it to [env] in config.toml or drop the link",
		Params:  domain.Params{"key": link.Key, "file": link.File},
	}
}

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
		Output:  func(line string) { p.Report(domain.Output{Subject: p.Worktree.Branch, Line: line}) },
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
			p.warn(domain.Warning{
				Code:    "provision.hook_env_unavailable",
				Message: err.Error() + " — on_create hooks run without the worktree's environment",
				Params:  domain.Params{"cause": err.Error()},
			})
			return nil
		}
		compose = append(compose, lines)
	}
	return env.HookEnv(env.HookEnvParams{JobParams: p.jobParams(ordinal), ComposeEnv: compose})
}
