package provision

import (
	"context"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	envcase "github.com/LucasPcq/wtm/internal/rules/runenv"
	"github.com/LucasPcq/wtm/internal/service/hooks"
	"github.com/LucasPcq/wtm/internal/service/identity"
	"github.com/LucasPcq/wtm/internal/service/runenv"
)

func (u unit) allocate(ctx context.Context, report kernel.Reporter, detail Detail) (Detail, error) {
	claim, err := identity.Allocate(ctx, identity.AllocateParams{Git: u.Ports.Git, Meta: u.Ports.Meta, StateDir: u.Ports.StateDir, Branch: u.Worktree.Branch})
	if err != nil && ctx.Err() != nil {
		return detail, err
	}
	if err != nil {
		if u.portsNeedOrdinal() {
			report.Warn(u.notSettled(notSettledParams{Code: CodePortsNotSettled, Cause: err}))
		}
		return detail, nil
	}
	detail.Ordinal, detail.Allocated = &claim.Ordinal, claim.Allocated
	warnUnlocked(warnUnlockedParams{Report: report, Unlocked: claim.Unlocked})
	return detail, nil
}

func (u unit) portsNeedOrdinal() bool {
	return envcase.PortsNeedOrdinal(envcase.PortsNeedOrdinalParams{Case: u.envCase(), Run: u.kept})
}

func (u unit) settle(ctx context.Context, report kernel.Reporter, detail Detail) (Detail, error) {
	if u.Project.RunErr != nil {
		report.Warn(u.notSettled(notSettledParams{Code: CodePortsNotSettledRun, Cause: u.Project.RunErr}))
		return detail, nil
	}
	for _, link := range u.ignored {
		report.Warn(ignoredWarning(link))
	}
	if _, verbatim := u.envCase().(envcase.Verbatim); verbatim || (u.portsNeedOrdinal() && detail.Ordinal == nil) {
		return detail, nil
	}
	writes, err := runenv.Env(runenv.EnvParams{
		Params:       u.runParams(detail),
		WorktreePath: u.Worktree.Path,
		EnvFiles:     u.Project.EnvFiles,
		Proxy:        u.Project.Global.Proxy,
		PublicPort:   u.publicPort(ctx),
	})
	if err != nil {
		report.Warn(u.notSettled(notSettledParams{Code: CodePortsNotSettled, Cause: err}))
		return detail, nil
	}
	applied, err := runenv.Apply(ctx, runenv.ApplyParams{Files: u.Ports.Env, Writes: writes})
	if err != nil && ctx.Err() != nil {
		return detail, err
	}
	if err != nil {
		report.Warn(u.notSettled(notSettledParams{Code: CodePortsNotSettled, Cause: err}))
		return detail, nil
	}
	warnUnlocked(warnUnlockedParams{Report: report, Unlocked: applied.Unlocked})
	detail.EnvPorts = rules.RedactEnvPortPlan(applied.Plan)
	return detail, nil
}

func (u unit) publicPort(ctx context.Context) int {
	if u.Ports.PublicPort == nil || (len(u.kept.EnvPorts) == 0 && len(u.kept.EnvValues) == 0) {
		return 0
	}
	return u.Ports.PublicPort(ctx, rules.ProxyPort(u.Project.Global))
}

func (u unit) runParams(detail Detail) runenv.Params {
	ordinal := 0
	if detail.Ordinal != nil {
		ordinal = *detail.Ordinal
	}
	return runenv.Params{
		ProjectDir: u.Project.MainPath,
		Run:        u.Project.Run,
		Identity:   runenv.Identity{Branch: u.Worktree.Branch, Ordinal: ordinal, Isolation: u.isolation()},
		Files:      u.Ports.Env,
	}
}

func (u unit) hooks(ctx context.Context, report kernel.Reporter, detail Detail) (Detail, error) {
	return detail, hooks.Execute(ctx, hooks.ExecuteParams{
		Hooks:   u.Project.OnCreate,
		WorkDir: u.Worktree.Path,
		Vars:    rules.TemplateVars{Worktree: u.Worktree.Path, Branch: u.Worktree.Branch, Root: u.Project.MainPath, FromBranch: u.Worktree.Source},
		Env:     u.hookEnv(detail),
		Output:  report.Output,
	})
}

func (u unit) hookEnv(detail Detail) map[string]string {
	if !u.hooksReadRun() || detail.Ordinal == nil {
		return nil
	}
	env, err := runenv.HookEnv(runenv.HookEnvParams{Params: u.runParams(detail), WorktreePath: u.Worktree.Path})
	if err != nil {
		return nil
	}
	return env
}

type notSettledParams struct {
	Code  kernel.Code
	Cause error
}

func (u unit) notSettled(params notSettledParams) kernel.Warning {
	return kernel.Warning{Code: params.Code, Params: kernel.Params{ParamBranch: u.Worktree.Branch, ParamCause: params.Cause.Error()}}
}

func ignoredWarning(link envcase.Ignored) kernel.Warning {
	code := CodeEnvPortIgnored
	if link.Kind == envcase.LinkValue {
		code = CodeEnvValueIgnored
	}
	return kernel.Warning{Code: code, Params: kernel.Params{kernel.ParamKey: link.Key, ParamFile: link.File}}
}
