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

// errOrdinalUnreadable is why no ordinal is given when another worktree's
// record cannot be read: its ordinal might be the one picked.
var errOrdinalUnreadable = errors.New("cannot read a live worktree's ordinal")

// needsOrdinal reports whether the worktree is numbered: its ports derive from
// its ordinal, or its hooks read the run environment.
func (p *provisioning) needsOrdinal() bool {
	return p.Project.RunErr == nil && (p.portsNeedOrdinal() || p.hooksReadRun())
}

// portsNeedOrdinal reports whether the worktree's ports derive from its ordinal.
func (p *provisioning) portsNeedOrdinal() bool {
	return env.NeedsOrdinal(p.Worktree.Answers.isolation, p.kept)
}

// hooksReadRun reports whether the on_create hooks run with the worktree's run
// environment.
func (p *provisioning) hooksReadRun() bool {
	return len(p.Project.OnCreate) > 0 && env.HooksReadRun(p.Project.Run, p.Worktree.Answers.isolation)
}

// number gives the worktree a free ordinal. Failing to find one stops the run
// only when ctx is cancelled; otherwise the worktree stays unnumbered, with a
// warning when its ports needed the ordinal.
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

	provisioned.Meta.Ordinal = ordinal
	return nil
}

// allocate picks the lowest ordinal no other worktree holds and records it in
// the worktree's meta.json, under the ordinals' lock so two runs cannot pick
// the same one.
func (p *provisioning) allocate(ctx context.Context) (int, error) {
	var ordinal int
	held, err := p.Service.Meta.LockOrdinals(ctx, func() error {
		taken, err := p.taken(ctx)
		if err != nil {
			return err
		}

		ordinal = worktree.FreeOrdinal(taken)
		written, err := p.Service.Meta.Update(ctx, worktree.MetaUpdate{Branch: p.Worktree.Branch, Change: func(meta *worktree.Meta) error {
			meta.Ordinal = ordinal
			return nil
		}})
		p.unlocked = p.unlocked || written.Unlocked
		return err
	})
	p.unlocked = p.unlocked || !held
	return ordinal, err
}

// taken returns the ordinals the other linked worktrees hold; one without a
// record holds none. It fails with errOrdinalUnreadable when a record exists
// but cannot be read.
func (p *provisioning) taken(ctx context.Context) ([]int, error) {
	worktrees, err := p.Service.Git.Worktrees(ctx)
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	holders, err := p.holders(worktrees)
	if err != nil {
		return nil, err
	}

	var taken []int
	for _, branch := range holders {
		meta, err := p.Service.Meta.Load(branch)
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

// holders returns the branches whose record may hold another worktree's
// ordinal: those of the linked worktrees, and, when one of them is detached
// and so names no branch, every recorded branch no worktree lists, since its
// record may be the detached one's.
func (p *provisioning) holders(worktrees []worktree.Worktree) ([]string, error) {
	var holders []string
	listed := map[string]bool{p.Worktree.Branch: true}
	detached := false
	for _, w := range worktrees {
		branch := w.Branch()
		_, isDetached := w.State.(worktree.Detached)
		detached = detached || !w.Main && isDetached
		if w.Main || branch == "" || listed[branch] {
			continue
		}
		listed[branch] = true
		holders = append(holders, branch)
	}
	if !detached {
		return holders, nil
	}

	recorded, err := p.Service.Meta.List()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errOrdinalUnreadable, err)
	}
	for _, branch := range recorded {
		if !listed[branch] {
			holders = append(holders, branch)
		}
	}
	return holders, nil
}

// settlesPorts reports whether the ports phase runs: run.toml links values into
// the worktree's env files, or there is a warning to give about them.
func (p *provisioning) settlesPorts() bool {
	switch {
	case p.Project.RunErr != nil || len(p.ignored) > 0:
		return true
	case p.Worktree.Answers.isolation == project.Verbatim:
		return false
	case len(p.kept.EnvPorts) > 0 || len(p.kept.EnvValues) > 0:
		return true
	}
	return len(env.OwnedTargets(p.kept, p.Project.EnvFiles)) > 0
}

// settle writes the values run.toml links into the worktree's env files and
// records them in provisioned. What keeps them from being settled is a
// warning, not a failure, except a cancelled ctx.
func (p *provisioning) settle(ctx context.Context, provisioned *Provisioned) error {
	if p.Project.RunErr != nil {
		p.warn(p.notSettled(notSettled{Cause: p.Project.RunErr, InRunToml: true}))
		return nil
	}

	for _, link := range p.ignored {
		p.warn(ignoredWarning(link))
	}

	if p.Worktree.Answers.isolation == project.Verbatim {
		return nil
	}

	// The compose project name never depends on the ordinal nor on the links:
	// it is written even when the ports cannot be.
	run := p.Project.Run
	if p.portsNeedOrdinal() && provisioned.Meta.Ordinal == 0 {
		run = ownedOnly(run)
	}
	writes, err := p.resolve(ctx, run, provisioned.ordinal())
	if err != nil {
		_, inRunToml := errors.AsType[*env.ValueError](err)
		p.warn(p.notSettled(notSettled{Cause: err, InRunToml: inRunToml}))
		writes, err = p.resolve(ctx, ownedOnly(run), provisioned.ordinal())
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
	if plan.NamesUnserved() {
		p.warn(p.namesUnserved())
	}
	return nil
}

// resolve returns the writes run links into the worktree's env files.
func (p *provisioning) resolve(ctx context.Context, run project.Run, ordinal *int) (env.Writes, error) {
	job := p.jobParams(ordinal)
	job.Run = run
	return env.Resolve(env.ResolveParams{
		JobParams:  job,
		Files:      p.Project.EnvFiles,
		Proxy:      p.Project.Global.Proxy,
		PublicPort: p.Service.PublicPort(ctx, p.Project.Global.ProxyPort()),
	})
}

// ownedOnly returns run without its links, which leaves only the values wtm
// owns to write.
func ownedOnly(run project.Run) project.Run {
	run.EnvPorts, run.EnvValues = nil, nil
	return run
}

// jobParams is what the worktree's job environment derives from; a nil ordinal
// leaves it at 0.
func (p *provisioning) jobParams(ordinal *int) env.JobParams {
	params := env.JobParams{Branch: p.Worktree.Branch, ProjectDir: p.Project.MainPath, Isolation: p.Worktree.Answers.isolation, Run: p.Project.Run}

	if ordinal != nil {
		params.Ordinal = *ordinal
	}
	return params
}

// apply edits each env file writes touches and returns what was settled,
// computed from the lines each file held when it was edited.
func (p *provisioning) apply(ctx context.Context, writes env.Writes) (env.Plan, error) {
	read := map[string][]dotenv.Line{}
	for _, file := range writes.Files() {
		written, err := p.Service.EnvFiles.Edit(ctx, dotenv.Edit{
			Path:   filepath.Join(p.Worktree.Path, file),
			Create: writes.Creates(file),
			Edit: func(lines []dotenv.Line) []dotenv.Line {
				read[file] = lines
				return writes.Edit(file, lines)
			},
		})
		if err != nil {
			return env.Plan{}, err
		}
		p.unlocked = p.unlocked || written.Unlocked
	}

	return writes.Settled(read), nil
}

// notSettled is why the worktree's ports were not settled; InRunToml when the
// fix is in run.toml.
type notSettled struct {
	Cause     error
	InRunToml bool
}

// notSettled is the warning that the worktree's ports were not settled, with
// the follow-up that settles them once the cause is fixed.
func (p *provisioning) notSettled(n notSettled) domain.Warning {
	code, fix := "provision.ports_not_settled", "what kept them"
	if n.InRunToml {
		code, fix = "provision.ports_not_settled_run", "run.toml"
	}

	branch, cause := p.Worktree.Branch, n.Cause.Error()
	followUp := p.Service.Settle(branch)
	followUp.Code, followUp.Message, followUp.Params = "provision.settle_ports", "settle the ports once "+fix+" is fixed", domain.Params{"branch": branch}
	return domain.Warning{
		Code:     code,
		Message:  cause + " — ports not settled",
		Params:   domain.Params{"branch": branch, "cause": cause},
		FollowUp: &followUp,
	}
}

// namesUnserved is the warning that the project asks for named addresses the
// proxy, off on this machine, cannot serve, with the follow-up that says what
// serves them.
func (p *provisioning) namesUnserved() domain.Warning {
	followUp := p.Service.ProxyStatus()
	followUp.Code, followUp.Message = "provision.proxy_status", "reports what serves names"
	return domain.Warning{
		Code:     "provision.ports_not_named",
		Message:  "this project asks for named addresses: the run proxy is off on this machine, so ports were written instead",
		Params:   domain.Params{"branch": p.Worktree.Branch},
		FollowUp: &followUp,
	}
}

// ignoredWarning is the warning that a run.toml link names a file that is not
// a configured env file.
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

// runHooks runs the on_create hooks in the worktree, their variables expanded,
// reporting each line they print and logging it to the hook log.
func (p *provisioning) runHooks(ctx context.Context, provisioned Provisioned) error {
	vars := env.HookVars{Worktree: p.Worktree.Path, Branch: p.Worktree.Branch, Root: p.Project.MainPath, FromBranch: p.Worktree.Source}
	hooks := make([]project.Hook, 0, len(p.Project.OnCreate))
	for _, configured := range p.Project.OnCreate {
		hooks = append(hooks, vars.Hook(configured))
	}

	return p.Service.RunHooks(ctx, env.HookChain{
		Hooks:   hooks,
		WorkDir: p.Worktree.Path,
		Env:     p.hookEnv(provisioned.ordinal()),
		Unset:   env.JobKeys(),
		Output:  func(line string) { p.Report(domain.Output{Subject: p.Worktree.Branch, Line: line}) },
		Log:     p.hookLog,
	})
}

// hookEnv is the run environment the hooks get, read from the worktree's
// compose env files; nil when they read none, or with a warning when a file
// cannot be read. An unnumbered worktree's hooks get only its compose project
// name, with a warning: its ports and offset would be main's.
func (p *provisioning) hookEnv(ordinal *int) map[string]string {
	if !p.hooksReadRun() {
		return nil
	}

	var compose [][]dotenv.Line
	for _, path := range env.ComposeEnvPaths(p.Project.Run) {
		lines, err := p.Service.EnvFiles.Read(filepath.Join(p.Worktree.Path, path))
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

	hookEnv := env.HookEnv(p.jobParams(ordinal), compose)
	if ordinal != nil {
		return hookEnv
	}

	p.warn(domain.Warning{
		Code:    "provision.hook_env_unnumbered",
		Message: "no ordinal for " + p.Worktree.Branch + " — on_create hooks run without its ports",
		Params:  domain.Params{"branch": p.Worktree.Branch},
	})
	if name := hookEnv[env.ComposeProjectKey]; name != "" {
		return map[string]string{env.ComposeProjectKey: name}
	}
	return nil
}
