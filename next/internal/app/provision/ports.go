package provision

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/jobenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
)

func (p *provisioning) settlesPorts() bool {
	switch {
	case p.Project.RunErr != nil || len(p.ignored) > 0:
		return true
	case p.isolation() == run.Verbatim:
		return false
	case len(p.kept.EnvPorts) > 0 || len(p.kept.EnvValues) > 0:
		return true
	}
	return len(settle.OwnedTargets(settle.OwnedTargetsParams{Run: p.kept, Files: p.Project.EnvFiles})) > 0
}

func (p *provisioning) settle(ctx context.Context, provisioned *Provisioned) error {
	if p.Project.RunErr != nil {
		p.warn(p.notSettled(notSettled{Cause: p.Project.RunErr, InRunToml: true}))
		return nil
	}
	for _, link := range p.ignored {
		p.warn(ignoredWarning(link))
	}
	if p.isolation() == run.Verbatim || p.portsNeedOrdinal() && provisioned.Ordinal == nil {
		return nil
	}
	writes, err := settle.Resolve(settle.ResolveParams{
		Params:     p.jobParams(provisioned.Ordinal),
		Files:      p.Project.EnvFiles,
		Proxy:      p.Project.Global.Proxy,
		PublicPort: p.publicPort(ctx),
	})
	if _, inRunToml := errors.AsType[*settle.ValueError](err); inRunToml {
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
	provisioned.EnvPorts = settle.Redact(plan)
	return nil
}

func (p *provisioning) jobParams(ordinal *int) jobenv.Params {
	params := jobenv.Params{Branch: p.Worktree.Branch, ProjectDir: p.Project.MainPath, Isolation: p.isolation(), Run: p.Project.Run}
	if ordinal != nil {
		params.Ordinal = *ordinal
	}
	return params
}

func (p *provisioning) publicPort(ctx context.Context) int {
	if len(p.kept.EnvPorts) == 0 && len(p.kept.EnvValues) == 0 {
		return 0
	}
	return p.Deps.PublicPort(ctx, p.Project.Global.ProxyPort())
}

func (p *provisioning) apply(ctx context.Context, writes settle.Writes) (settle.Plan, error) {
	read := map[string][]dotenv.Line{}
	for _, file := range writes.Files() {
		written, err := p.Deps.EnvFiles.Edit(ctx, dotenv.Edit{
			Path:   filepath.Join(p.Worktree.Path, file),
			Create: writes.Creates(file),
			Edit: func(lines []dotenv.Line) []dotenv.Line {
				read[file] = lines
				return writes.Edit(settle.EditParams{File: file, Lines: lines})
			},
		})
		if err != nil {
			return settle.Plan{}, err
		}
		p.unlocked = p.unlocked || written.Unlocked
	}
	return writes.Settled(read), nil
}

type notSettled struct {
	Cause     error
	InRunToml bool
}

func (p *provisioning) notSettled(n notSettled) wtm.Warning {
	code, fix := "provision.ports_not_settled", "this"
	if n.InRunToml {
		code, fix = "provision.ports_not_settled_run", "run.toml"
	}
	branch, cause := p.Worktree.Branch, n.Cause.Error()
	return wtm.Warning{
		Code:    code,
		Message: cause + " — ports not settled — run `wtm env " + branch + "` once " + fix + " is fixed",
		Params:  wtm.Params{"branch": branch, "cause": cause},
	}
}

func ignoredWarning(link settle.Ignored) wtm.Warning {
	code := "provision.env_port_ignored"
	if link.Kind == settle.LinkValue {
		code = "provision.env_value_ignored"
	}
	return wtm.Warning{
		Code:    code,
		Message: string(link.Kind) + " " + link.Key + " in " + link.File + " ignored: not a configured env file — add it to [env] in config.toml or drop the link",
		Params:  wtm.Params{"key": link.Key, "file": link.File},
	}
}
