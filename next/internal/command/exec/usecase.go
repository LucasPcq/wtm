// Package exec runs one shell line in worktrees of a repository, several at once: which worktrees the request settles on, then the line in each, from its root, with the environment its hooks get, and what each printed and how it exited.
package exec

import (
	"cmp"
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "exec"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase runs a shell line in worktrees of one repository: the ports it reaches the repository and the shell through, the CPUs it runs on, and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Meta       Meta
	EnvFiles   EnvFiles
	RunHooks   RunHooks
	Syntax     Syntax
	StateDir   string // wtm's state, where each target's log goes
	CPUs       int    // how many commands run at once when the request does not say
	Now        func() time.Time
	Repository repository.Service
	Operations operation.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per target, in the plan's order, and the run again where it did not pass.
type Result = domain.Result[Detail]

// An Item is what became of the command in one target: its status and why, the error it failed on, its warnings, and its Detail once it started.
type Item = domain.Unit[Detail]

// A Detail is the command's run in one target, as the JSON envelope reports it: where it ran, how it exited (nil when it never exited: stopped by a signal, or never started), how long it took, its log, its last lines as a terminal left them, and with print the whole of what it printed.
type Detail struct {
	Path       string   `json:"path"`
	ExitCode   *int     `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
	Log        string   `json:"log,omitempty"`
	Tail       []string `json:"tail"`
	Output     string   `json:"output,omitempty"`
}

// Excerpt is what the conclusion shows of a target that did not pass: its last lines and its log.
func (d Detail) Excerpt() ([]string, string) {
	return d.Tail, d.Log
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a command that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; under --all, the targets are planned again once their locks are held.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be applied; a dry run ends here with the plan and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on other facts is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 5. Apply between the started and finished events other processes listen to.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides where the request runs its command as the repository stands, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings := Decide(facts, resolved)
	planned.Recap = planned.recap()
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// Apply runs the command in each target, Jobs of them at once, reporting each one's events; a failure stops none of the others. Once ctx is cancelled, the commands running are stopped and the targets not reached yet are skipped. The items keep the plan's order.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	// The commands report from several goroutines; a surface draws one event at a time.
	var mu sync.Mutex
	serial := func(event domain.Event) {
		mu.Lock()
		defer mu.Unlock()
		report(event)
	}

	items := make([]Item, len(plan.Targets))
	slots := make(chan struct{}, plan.Jobs)
	var wg sync.WaitGroup
	for i, t := range plan.Targets {
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
		}
		if ctx.Err() != nil {
			items[i] = Item{Subject: t.Worktree, Status: domain.StatusSkipped, Reason: domain.ReasonNotReached}
			continue
		}
		wg.Go(func() {
			defer func() { <-slots }()
			items[i] = u.run(ctx, plan, t, serial)
		})
	}
	wg.Wait()

	retry := plan.retry(items)
	for _, item := range items {
		if item.Err != nil {
			item.Err.FollowUp = retry
		}
	}
	result := Result{Items: items}
	if retry != nil {
		result.FollowUps = []domain.FollowUp{*retry}
	}
	return result
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may run in and the command, and the request's problems as field errors, the shell's verdict on the command among them.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what exec decides from: the project's config, the repository's snapshot, the meta record of each worktree, which worktrees' directory is gone, where the caller stands, and what the shell says of the command. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Records: map[string]worktree.Meta{}, Gone: map[string]bool{}, CPUs: u.CPUs}
	for _, w := range repo.Checkouts() {
		if facts.Gone[w.Path], err = repository.Gone(ctx, u.Git, w); err != nil {
			return Facts{}, repository.Interrupted(ctx, err)
		}
		if w.Branch() == "" {
			continue
		}
		// A record that cannot be read gives the command no run environment: it runs all the same.
		if record, err := u.Meta.Load(w.Branch()); err == nil {
			facts.Records[w.Branch()] = record
		}
	}

	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, req.Caller.Cwd)

	if strings.TrimSpace(req.Command) != "" {
		facts.Syntax = u.Syntax(ctx, req.Command)
	}
	return facts, ctx.Err()
}

// ----------------------------------------------------------------------------
// The steps of Apply

// The codes of what becomes of a target: the command exited other than 0, it could not start, or its run environment could not be read.
const (
	CodeFailed         = Name + ".failed"
	CodeStartFailed    = Name + ".start_failed"
	CodeEnvUnavailable = Name + ".env_unavailable"
)

// logDir is where each target's log goes under wtm's state, overwritten by each run, named after its branch, or detachedLog and its directory's name on no branch.
const (
	logDir      = "exec"
	detachedLog = "detached-"
)

// run runs the command in one target between its unit's events, its output reported line by line and kept in its log; a target reached after a cancel is skipped.
func (u UseCase) run(ctx context.Context, plan Plan, t Target, report domain.Report) (item Item) {
	item.Subject = t.Worktree
	if ctx.Err() != nil {
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonNotReached
		return item
	}

	report(domain.UnitStarted{Subject: t.Worktree})
	defer func() { report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()

	runEnv, warnings := u.runEnv(plan, t)
	item.Warnings = warnings
	detail := &Detail{Path: t.Path, Tail: []string{}}
	if u.StateDir != "" {
		detail.Log = filepath.Join(u.StateDir, logDir, worktree.StateSegment(cmp.Or(t.Branch, detachedLog+filepath.Base(t.Path)))+".log")
	}
	var lines tail
	var whole strings.Builder

	report(domain.PhaseStarted{Subject: t.Worktree, Phase: domain.PhaseExec, Log: detail.Log})
	started := u.Now()
	err := u.RunHooks(ctx, env.HookChain{
		Hooks:   []project.Hook{{Cmd: plan.Command}},
		WorkDir: t.Path,
		Env:     runEnv,
		Unset:   unset(plan.facts.Run),
		Output: func(line string) {
			lines.add(line)
			if plan.Print {
				whole.WriteString(line + "\n")
			}
			report(domain.Output{Subject: t.Worktree, Line: line})
		},
		Log: detail.Log,
	})
	report(domain.PhaseFinished{Subject: t.Worktree, Phase: domain.PhaseExec, Failed: err != nil})

	detail.DurationMs, detail.Output = u.Now().Sub(started).Milliseconds(), whole.String()
	detail.Tail = append(detail.Tail, lines...)
	item.Detail = detail

	var failure *env.HookFailure
	exited := errors.As(err, &failure) && failure.ExitCode != nil
	switch {
	case err == nil:
		item.Status, detail.ExitCode = domain.StatusDone, new(0)
	case ctx.Err() != nil:
		item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
	case exited:
		detail.ExitCode = failure.ExitCode
		code := strconv.Itoa(*failure.ExitCode)
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: CodeFailed, Message: "exited " + code, Params: domain.Params{"exit_code": code}}
	default:
		item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeStartFailed, Message: "couldn't run the command"})
	}
	return item
}

// runEnv is the run environment the command gets in t, the one its hooks get (its Compose project, its ports): nil when they read none or t has no ordinal, or with a warning when a compose env file cannot be read.
func (u UseCase) runEnv(plan Plan, t Target) (map[string]string, []domain.Warning) {
	record := plan.facts.Records[t.Branch]
	if !plan.facts.readsRun(t) || record.Ordinal == 0 {
		return nil, nil
	}

	job := env.JobParams{Branch: t.Branch, ProjectDir: plan.facts.MainPath, Ordinal: record.Ordinal, Isolation: record.Isolation.Effective(), Run: plan.facts.Run}
	runEnv, err := env.ReadHookEnv(job, t.Path, u.EnvFiles.Read)
	if err != nil {
		return nil, []domain.Warning{{Code: CodeEnvUnavailable, Message: err.Error() + " — the command runs without the worktree's environment", Params: domain.Params{"cause": err.Error()}}}
	}
	return runEnv, nil
}
