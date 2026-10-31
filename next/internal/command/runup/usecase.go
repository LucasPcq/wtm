// Package runup starts a profile of run.toml in worktrees of a repository, several at once: which worktrees and which profile the request settles on, what the other worktrees' jobs become, then each job in order through the run daemon, and what each became.
package runup

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"maps"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `run up`.
const Name = domain.CommandRunUp

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase starts a profile in worktrees of one repository: the ports it reaches the repository, the run daemon and the loopback through, wtm's state (where each worktree's logs go), the repository's common directory (which the jobs' events name), how long it waits for a service to fail once launched when no port check waits, and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Meta       Meta
	EnvFiles   EnvFiles
	Daemon     Daemon
	Listening  Listening
	StateDir   string
	CommonDir  string
	Settle     time.Duration
	Repository repository.Service
	Operations operation.Service
	Provision  provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree, in the plan's order, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of the start in one worktree: its status and why, the error it failed on, its warnings, and its Detail once it began.
type Item = domain.Unit[Detail]

// A Detail is the start in one worktree, as the JSON envelope reports it: where, the profile, and each job of the profile with what became of it.
type Detail struct {
	Path    string      `json:"path"`
	Profile string      `json:"profile"`
	Jobs    []JobResult `json:"jobs"`
}

// A JobResult is what became of one job: its status, its URL, its log, the ports it binds and whether each answered, the namespace of a shared service's data the worktree holds, and for one that failed or crashed its exit code and last lines.
type JobResult struct {
	Name      string          `json:"name"`
	Kind      project.JobKind `json:"kind"`
	Status    JobStatus       `json:"status"`
	URL       string          `json:"url,omitempty"`
	Log       string          `json:"log,omitempty"`
	Ports     []Port          `json:"ports,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
	ExitCode  *int            `json:"exit_code,omitempty"`
	Lines     []string        `json:"lines,omitempty"`
}

// A Port is a port a job declares, by its name, the number it binds, and whether something answered on it (nil when it was not checked).
type Port struct {
	Name      string `json:"name"`
	Port      int    `json:"port"`
	Listening *bool  `json:"listening"`
}

// leftUp reports whether the start left a job up in its worktree.
func (d Detail) leftUp() bool {
	return slices.ContainsFunc(d.Jobs, func(job JobResult) bool {
		return job.Status == JobStarted || job.Status == JobAlreadyRunning || job.Status == JobJoined
	})
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a job that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; the worktrees defaulting to the caller's are planned again once their locks are held.
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

	// 5. Apply between the started and finished events other processes listen to; the jobs' own events carry the caller's correlation ID.
	plan.correlationID = opts.CorrelationID
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result, outcome.Plan = &result, plan.applied(result)
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request starts as the repository and the run daemon stand, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings, err := Decide(facts, resolved, DecideParams{Restart: u.Repository.Restart})
	if err != nil {
		return Plan{}, nil, err
	}
	planned.Recap, planned.Empty = planned.recap(), planned.unchanged()
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// CodeDaemonUnavailable fails a start that could not begin: the run daemon of this build could not be made sure of.
const CodeDaemonUnavailable = Name + ".daemon_unavailable"

// CodeStopFailed warns of a worktree whose jobs an exclusive start could not stop.
const CodeStopFailed = Name + ".stop_failed"

// Apply makes sure of a run daemon of this build, stops the other worktrees' jobs when the plan says so, then starts each worktree's jobs, the worktrees at once, each one's in order; a worktree that fails stops none of the others. Once ctx is cancelled, the jobs the daemon took keep running and the jobs not reached are not started. The items keep the plan's order.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	// The worktrees report from several goroutines; a surface draws one event at a time.
	var mu sync.Mutex
	serial := func(event domain.Event) {
		mu.Lock()
		defer mu.Unlock()
		report(event)
	}

	items := make([]Item, len(plan.Worktrees))
	if err := u.Daemon.Ensure(ctx); err != nil {
		failure := u.daemonFailure(err)
		for i, t := range plan.Worktrees {
			items[i] = Item{Subject: t.Worktree, Status: domain.StatusFailed, Err: failure, Detail: plan.notStarted(t)}
		}
		return Result{Items: items}
	}

	var warnings []domain.Warning
	for _, other := range plan.Others {
		if _, err := u.Daemon.StopAll(ctx, run.StopAll{WorkDir: other.Path, CorrelationID: plan.correlationID}); err != nil {
			warnings = append(warnings, domain.Warning{Code: CodeStopFailed, Message: "couldn't stop the jobs of " + other.Worktree + ": " + err.Error(), Params: domain.Params{"worktree": other.Worktree}})
		}
	}

	var wg sync.WaitGroup
	for i, t := range plan.Worktrees {
		wg.Go(func() { items[i] = u.start(ctx, plan, t, serial) })
	}
	wg.Wait()

	return Result{Items: items, Warnings: warnings, FollowUps: plan.next(items)}
}

// daemonFailure is why no job could start: a daemon of another build holding jobs, followed by its restart, or the daemon not starting.
func (u UseCase) daemonFailure(err error) *domain.Error {
	if refused, ok := errors.AsType[*run.Error](err); ok && (refused.Code == run.CodeSkew || refused.Code == run.CodeLegacy) {
		skew := run.SkewBuild
		if refused.Code == run.CodeLegacy {
			skew = run.SkewLegacy
		}
		failure := skewRefusal(&run.Skewed{Skew: skew}, u.Repository.Restart)
		failure.Message, failure.Cause = "nothing started: "+refused.Message, err
		return failure
	}
	return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeDaemonUnavailable, Message: "couldn't reach the run daemon"})
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may start in, the profiles, whether the others' jobs stop first, whether a job that changes foreign data starts, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// The codes of a repository run up cannot start in: run.toml does not exist (followed by run init), or does not load.
const (
	CodeNotInitialized = Name + ".not_initialized"
	CodeInit           = Name + ".init"
	CodeRunInvalid     = Name + ".run_invalid"
)

// commandRunInit sets the run module up in a repository: not migrated yet.
const commandRunInit = "run init"

// Observe reads what run up decides from: the project's config and its run.toml, the repository's snapshot, the meta record of each worktree, which worktrees' directory is gone, where the caller stands, and the jobs the run daemon runs, never starting it. A repository with no wtm config is refused with CodeConfigNotFound, one with no run.toml with CodeNotInitialized.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}
	if repo.RunErr != nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeRunInvalid, Message: "invalid run.toml: " + repo.RunErr.Error()}
	}
	if !repo.Run.Declared {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeNotInitialized, Message: "no run.toml: nothing to start", FollowUp: &domain.FollowUp{Command: commandRunInit, Code: CodeInit, Message: "declare the jobs"}}
	}

	facts := Facts{Repository: repo, Records: map[string]worktree.Meta{}, RecordErrs: map[string]error{}, Gone: map[string]bool{}}
	for _, w := range repo.Checkouts() {
		if facts.Gone[w.Path], err = repository.Gone(ctx, u.Git, w); err != nil {
			return Facts{}, repository.Interrupted(ctx, err)
		}
		if w.Branch() == "" {
			continue
		}
		record, err := u.Meta.Load(w.Branch())
		switch {
		case err == nil:
			facts.Records[w.Branch()] = record
		case !errors.Is(err, fs.ErrNotExist):
			facts.RecordErrs[w.Branch()] = err
		}
	}

	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, req.Caller.Cwd)

	facts.Jobs, facts.JobsErr = u.Daemon.Jobs(ctx)
	if skewed, ok := errors.AsType[*run.Skewed](facts.JobsErr); ok {
		facts.Skewed, facts.JobsErr = skewed, nil
	}
	return facts, ctx.Err()
}

// ----------------------------------------------------------------------------
// The steps of Apply

// The codes of what becomes of a worktree: it could not be numbered, its environment could not be read, a job failed to start or a task failed, a service crashed once started.
const (
	CodeUnnumbered     = Name + ".unnumbered"
	CodeEnvUnavailable = Name + ".env_unavailable"
	CodeJobFailed      = Name + ".job_failed"
	CodeCrashed        = Name + ".crashed"
)

// The codes of the warnings of a worktree: a port nothing answered on, a namespace its record could not keep.
const (
	CodePortSilent          = Name + ".port_silent"
	CodeNamespaceUnrecorded = Name + ".namespace_unrecorded"
)

// defaultProbe is how long a start waits for the ports its services declare when run.toml does not say.
const defaultProbe = 15 * time.Second

// logsDir is where the jobs' logs go under wtm's state: a directory per worktree, a log per job.
const logsDir = "logs"

// starting is one worktree's start under way: what it starts from, and its item.
type starting struct {
	UseCase
	plan     Plan
	target   Target
	report   domain.Report
	item     Item
	launched time.Time // when the last job was launched
	ordinal  int
	env      map[string]string
	shared   *run.Shared
}

// start starts the jobs of t, in order, between its unit's events: it numbers the worktree first when it has no ordinal, reads its environment, starts each job (a task to its end), stops at the first that fails, then checks the ports its services declare and that none of them ended since.
func (u UseCase) start(ctx context.Context, plan Plan, t Target, report domain.Report) Item {
	s := &starting{UseCase: u, plan: plan, target: t, report: report, item: Item{Subject: t.Worktree, Detail: plan.notStarted(t)}}
	if ctx.Err() != nil {
		s.item.Status, s.item.Reason = domain.StatusSkipped, domain.ReasonNotReached
		return s.item
	}

	report(domain.UnitStarted{Subject: t.Worktree})
	defer func() { report(domain.UnitFinished{Subject: s.item.Subject, Status: s.item.Status}) }()

	if err := s.prepare(ctx); err != nil {
		s.item.Status, s.item.Err = domain.StatusFailed, err
		return s.item
	}
	for i := range t.Jobs {
		if ctx.Err() != nil {
			s.item.Status, s.item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
			return s.item
		}
		if !s.job(ctx, i) {
			return s.item
		}
	}

	s.check(ctx)
	switch {
	case s.item.Status != "":
	case ctx.Err() != nil:
		s.item.Status, s.item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
	default:
		s.item.Status = domain.StatusDone
	}
	if s.item.Status == domain.StatusDone && !slices.ContainsFunc(s.item.Detail.Jobs, func(job JobResult) bool { return job.Status != JobAlreadyRunning }) {
		s.item.Reason = domain.ReasonUnchanged
	}
	return s.item
}

// prepare numbers the worktree when it has none, then reads its environment and, for a shared job, the main checkout's.
func (s *starting) prepare(ctx context.Context) *domain.Error {
	ordinal := s.plan.facts.ordinalOf(s.target)
	if !s.target.Numbered {
		numbered, err := s.Provision.Number(ctx, s.target.Branch)
		if err != nil {
			return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeUnnumbered, Message: "couldn't number " + s.target.Worktree + ": its ports are unknown"})
		}
		ordinal = numbered
		// Its ports and namespaces were unknown to the plan: they are the ordinal's now.
		s.target.Numbered = true
		s.target = s.plan.facts.plan(s.target, s.plan.facts.jobsOf(s.plan.Profile), ordinal)
		for i, job := range s.target.Jobs {
			s.item.Detail.Jobs[i].Namespace = job.Namespace
		}
	}
	s.ordinal = ordinal

	var err error
	if s.env, err = env.ReadHookEnv(s.plan.facts.jobParams(s.target, ordinal), s.target.Path, s.EnvFiles.Read); err != nil {
		return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeEnvUnavailable, Message: "couldn't read " + s.target.Worktree + "'s environment"})
	}
	if !slices.ContainsFunc(s.target.Jobs, func(job JobPlan) bool { return s.plan.facts.shared(job.Name) }) {
		return nil
	}
	main := s.plan.facts.mainTarget()
	mainEnv, err := env.ReadHookEnv(s.plan.facts.jobParams(main, 0), main.Path, s.EnvFiles.Read)
	if err != nil {
		return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeEnvUnavailable, Message: "couldn't read the main checkout's environment"})
	}
	s.shared = &run.Shared{WorkDir: main.Path, Env: mainEnv, LogDir: s.logDir(main.Branch)}
	return nil
}

// job starts the job at index of the worktree between its phase's events, its lines reported as they come, and reports whether the start goes on: a failure, or a cancel, stops it.
func (s *starting) job(ctx context.Context, index int) bool {
	planned, result := s.target.Jobs[index], &s.item.Detail.Jobs[index]
	declared, _ := s.plan.facts.declared(planned.Name)
	spec := s.spec(s.plan.facts.effective(declared))

	var lines tail
	result.Log = s.logFile(spec)
	s.report(domain.PhaseStarted{Subject: s.target.Worktree, Phase: domain.PhaseJob, Job: planned.Name, Log: result.Log})
	started, err := s.Daemon.Start(ctx, spec, func(line run.Line) {
		lines.add(line.Text)
		s.report(domain.Output{Subject: s.target.Worktree, Job: planned.Name, Line: line.Text})
	})
	s.launched = time.Now()
	refused, _ := errors.AsType[*run.Error](err)
	up := refused != nil && refused.Code == run.CodeJobRunning && declared.Kind != project.Task
	s.report(domain.PhaseFinished{Subject: s.target.Worktree, Phase: domain.PhaseJob, Job: planned.Name, Failed: err != nil && !up})

	result.URL, result.Ports = started.URL, resultPorts(planned.Ports)
	switch {
	case up:
		result.Status, result.URL = JobAlreadyRunning, s.plan.facts.urlOf(planned.Name, s.target.Path)
	case err != nil && ctx.Err() != nil:
		// The start was cut short: the daemon says whether it took it, and what it took keeps running.
		s.item.Status, s.item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		if s.took(planned.Name) {
			result.Status = JobStarted
			if planned.Action == ActionJoin {
				result.Status = JobJoined
				s.recordNamespace(ctx, declared)
			}
		}
		return false
	case err != nil:
		result.Status, result.Lines = JobFailed, lines
		if refused != nil {
			result.ExitCode = refused.ExitCode
		}
		s.item.Status, s.item.Err = domain.StatusFailed, jobFailure(planned.Name, err, result.ExitCode)
		return false
	case declared.Kind == project.Task:
		result.Status = JobDone
	case planned.Action == ActionJoin:
		result.Status = JobJoined
		s.recordNamespace(ctx, declared)
	default:
		result.Status = JobStarted
	}
	return true
}

// tookWithin bounds how long a start cut short asks the daemon whether it took it.
const tookWithin = 2 * time.Second

// took reports whether the daemon lists the job of name up in the worktree, as it does once it took its start.
func (s *starting) took(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), tookWithin)
	defer cancel()
	jobs, err := s.Daemon.Jobs(ctx)
	if _, skewed := errors.AsType[*run.Skewed](err); err != nil && !skewed {
		return false
	}
	return slices.Contains(run.UpIn(jobs, s.target.Path), name)
}

func jobFailure(job string, err error, exitCode *int) *domain.Error {
	params := domain.Params{"job": job}
	if exitCode != nil {
		params["exit_code"] = strconv.Itoa(*exitCode)
	}
	return &domain.Error{Kind: domain.KindInternal, Code: CodeJobFailed, Message: job + " failed: " + err.Error(), Params: params}
}

// spec is what the daemon is asked to start job with: the worktree's environment, its log directory, the names the proxy serves it under, the main checkout of a shared job, and the origin its events name.
func (s *starting) spec(job project.Job) run.Spec {
	spec := run.Spec{
		Job:     job,
		Profile: s.plan.Profile,
		WorkDir: s.target.Path,
		Env:     withPorts(s.env, s.plan.facts.jobParams(s.target, s.ordinal).JobPorts(job)),
		Unset:   env.NotInherited(s.plan.facts.Run),
		LogDir:  s.logDir(s.target.Branch),
		Origin:  &run.Origin{Root: s.plan.facts.MainPath, CommonDir: s.CommonDir, Branch: s.target.Branch, CorrelationID: s.plan.correlationID},
		Routes:  s.routes(job),
	}
	if job.Scope == project.Shared && s.shared != nil {
		shared := *s.shared
		shared.Env = withPorts(shared.Env, s.plan.facts.jobParams(s.plan.facts.mainTarget(), 0).JobPorts(job))
		spec.Shared = &shared
	}
	return spec
}

// withPorts is jobEnv with each of ports spelled, a copy.
func withPorts(jobEnv map[string]string, ports map[string]int) map[string]string {
	spelled := maps.Clone(jobEnv)
	for name, port := range ports {
		spelled[name] = strconv.Itoa(port)
	}
	return spelled
}

// routes are the names the proxy serves job under: its own and those of the jobs it runs, each that publishes a url.
func (s *starting) routes(job project.Job) []run.Route {
	var routes []run.Route
	repo := filepath.Base(s.plan.facts.MainPath)
	for _, routed := range append([]project.Job{job}, s.plan.facts.children(job)...) {
		host := env.RouteHost(env.RouteHostParams{Job: routed, Slug: s.env[env.WorktreeKey], Project: repo})
		if host != "" {
			routes = append(routes, run.Route{Job: routed.Name, Host: host, Port: routed.URL.Port})
		}
	}
	return routes
}

// logDir is where the jobs of branch keep their logs; none for a worktree on no branch.
func (s *starting) logDir(branch string) string {
	if branch == "" || s.StateDir == "" {
		return ""
	}
	return filepath.Join(s.StateDir, logsDir, worktree.StateSegment(branch))
}

// logFile is the log the job spec starts writes to, none when it keeps none.
func (s *starting) logFile(spec run.Spec) string {
	dir := spec.LogDir
	if spec.Shared != nil {
		dir = spec.Shared.LogDir
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, spec.Job.Name+".log")
}

// recordNamespace records in the worktree's meta record that it holds a namespace of job's data, which clean drops with it, a cancel notwithstanding: the daemon made it; a record it cannot keep is a warning.
func (s *starting) recordNamespace(ctx context.Context, job project.Job) {
	ctx = context.WithoutCancel(ctx)
	if _, made, _ := env.NamespaceFor(job, s.env); !made || s.target.Branch == "" {
		return
	}
	_, err := s.Meta.Update(ctx, worktree.MetaUpdate{Branch: s.target.Branch, Change: func(meta *worktree.Meta) error {
		if !slices.Contains(meta.Namespaces, job.Name) {
			meta.Namespaces = append(meta.Namespaces, job.Name)
		}
		return nil
	}})
	if err != nil {
		s.item.Warnings = append(s.item.Warnings, domain.Warning{Code: CodeNamespaceUnrecorded, Message: s.target.Worktree + "'s record does not keep its namespace of " + job.Name + ", which clean will not drop: " + err.Error(), Params: domain.Params{"worktree": s.target.Worktree, "job": job.Name}})
	}
}

// check waits for the ports the services started declare to answer, the check run.toml and the request allow, and at least a moment since the last launch for a service that fails at once to end, then asks the daemon which of them ended since: each fails the worktree, crashed; each port nothing answered on is a warning.
func (s *starting) check(ctx context.Context) {
	var probed []int
	for i, job := range s.target.Jobs {
		result := s.item.Detail.Jobs[i]
		declared, _ := s.plan.facts.declared(job.Name)
		if result.Status == JobStarted && declared.Kind != project.Task && !declared.NoProbe && !declared.BindsNoPort {
			probed = append(probed, sortedPorts(job.Ports)...)
		}
	}
	if within := s.probeBudget(); len(probed) > 0 && within > 0 {
		answered := s.Listening(ctx, probed, within)
		// A check cut short says nothing of the ports it did not reach.
		if ctx.Err() != nil {
			return
		}
		s.markPorts(answered)
	}
	sleep(ctx, s.Settle-time.Since(s.launched))
	if ctx.Err() != nil {
		return
	}

	jobs, err := s.Daemon.Jobs(ctx)
	if _, skewed := errors.AsType[*run.Skewed](err); err != nil && !skewed {
		return
	}
	for i := range s.item.Detail.Jobs {
		result := &s.item.Detail.Jobs[i]
		if result.Status != JobStarted {
			continue
		}
		at := slices.IndexFunc(jobs, func(job run.Job) bool { return job.Name == result.Name && job.WorkDir == s.target.Path })
		if at < 0 || jobs[at].Up || jobs[at].State != run.Crashed {
			continue
		}
		result.Status, result.ExitCode = JobCrashed, jobs[at].ExitCode
		if last := jobs[at].LastLine; last != nil {
			result.Lines = []string{last.Text}
		}
		if s.item.Err == nil {
			s.item.Status, s.item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: CodeCrashed, Message: result.Name + " crashed once started", Params: domain.Params{"job": result.Name}}
		}
	}
}

// probeBudget is how long the ports are waited for: run.toml's port_probe_timeout, 15 seconds when it says none, none when it is below 0 or the request skips the check.
func (s *starting) probeBudget() time.Duration {
	switch timeout := s.plan.facts.Run.PortProbeTimeout; {
	case s.plan.NoProbe, timeout < 0:
		return 0
	case timeout == 0:
		return defaultProbe
	default:
		return time.Duration(timeout) * time.Second
	}
}

// markPorts records which ports of the services started answered, and warns of each job one of whose ports none did; a job run.toml keeps out of the check (probe = false, binds_no_port) is left unchecked.
func (s *starting) markPorts(answered map[int]bool) {
	for i := range s.item.Detail.Jobs {
		result := &s.item.Detail.Jobs[i]
		if declared, _ := s.plan.facts.declared(result.Name); declared.NoProbe || declared.BindsNoPort {
			continue
		}
		var silent []string
		for p := range result.Ports {
			port := &result.Ports[p]
			listening := answered[port.Port]
			if result.Status != JobStarted {
				continue
			}
			port.Listening = &listening
			if !listening {
				silent = append(silent, port.Name+" "+strconv.Itoa(port.Port))
			}
		}
		if len(silent) > 0 {
			s.item.Warnings = append(s.item.Warnings, domain.Warning{Code: CodePortSilent, Message: result.Name + " started, but nothing answers on " + strings.Join(silent, ", "), Params: domain.Params{"worktree": s.target.Worktree, "job": result.Name, "ports": strings.Join(silent, ", ")}})
		}
	}
}

// ----------------------------------------------------------------------------
// Helpers

// mainTarget is the main worktree, as a start in it would be planned.
func (f Facts) mainTarget() Target {
	for _, w := range f.Worktrees {
		if w.Main {
			return Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Main: true, Numbered: true}
		}
	}
	return Target{Path: f.MainPath, Main: true, Numbered: true}
}

// resultPorts are the ports of a job's plan as its result reports them, by name, not yet checked.
func resultPorts(ports map[string]int) []Port {
	if len(ports) == 0 {
		return nil
	}
	found := make([]Port, 0, len(ports))
	for _, name := range slices.Sorted(maps.Keys(ports)) {
		found = append(found, Port{Name: name, Port: ports[name]})
	}
	return found
}

func sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}

// tailLines is how many of its last lines a job that failed keeps.
const tailLines = 20

// A tail keeps the last lines a job printed.
type tail []string

func (t *tail) add(line string) {
	*t = append(*t, line)
	if len(*t) > tailLines {
		*t = (*t)[len(*t)-tailLines:]
	}
}
