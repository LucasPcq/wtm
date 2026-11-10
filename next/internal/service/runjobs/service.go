// Package runjobs starts jobs of run.toml in a worktree for the commands that do, through wtm's daemon: the facts a start decides from, how each job starts in its worktree, what refuses a start before anything starts, then the start itself (the worktree numbered, each job started in order between its phase's events, its ports checked, a crash caught) and what became of each job. Every code it gives is the asking command's: <command>.<code>.
package runjobs

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The service

// Service starts jobs in the worktrees of one repository. Every field must be set before Observe.
type Service struct {
	Meta      Meta
	EnvFiles  EnvFiles
	Daemon    Daemon
	Listening Listening
	StateDir  string        // wtm's state, where each worktree's job logs go
	CommonDir string        // the repository's common directory, which the jobs' events name
	Settle    time.Duration // how long a start waits for a service to fail once launched, when no port check waits

	Gone    func(ctx context.Context, w worktree.Worktree) (bool, error)                 // whether w's directory is gone (repository.Gone)
	Located func(ctx context.Context, name string, caller domain.Caller) (string, error) // the path a name stands for, through its symbolic links (repository.Located)
	Number  func(ctx context.Context, branch string) (int, error)                        // numbers a worktree with no ordinal, recording it (provision.Service.Number)
	Owing   func() []env.OwedDrop                                                        // the drops removed worktrees owe the shared services (teardown.Service.Owing)
}

// ----------------------------------------------------------------------------
// Observe

// The codes of a repository no job can start in, after the command's name: run.toml does not exist (followed by RunInit), or does not load.
const (
	NotInitialized = ".not_initialized"
	RunInit        = ".init"
	RunInvalid     = ".run_invalid"
)

// commandRunInit sets the run module up in a repository: not migrated yet.
const commandRunInit = "run init"

// ObserveParams are the command observing, whose codes the refusals give, the repository as it observed it (repository.Service.Observe), and where its caller stands.
type ObserveParams struct {
	Command    string
	Repository worktree.Repository
	Cwd        string
}

// Observe reads, on the repository of params, what a start decides from: the meta record of each worktree, which worktrees' directory is gone, where the caller stands, the jobs wtm's daemon runs, never starting it, and the drops owed. A repository with no run.toml is refused with <command>.not_initialized, one whose run.toml does not load with <command>.run_invalid.
func (s Service) Observe(ctx context.Context, params ObserveParams) (Facts, error) {
	repo := params.Repository
	if repo.RunErr != nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: params.Command + RunInvalid, Message: "invalid run.toml: " + repo.RunErr.Error()}
	}
	if !repo.Run.Declared {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: params.Command + NotInitialized, Message: "no run.toml: nothing to start", FollowUp: &domain.FollowUp{Command: commandRunInit, Code: params.Command + RunInit, Message: "declare the jobs"}}
	}

	facts := Facts{Repository: repo, Records: map[string]worktree.Meta{}, RecordErrs: map[string]error{}, Gone: map[string]bool{}}
	var err error
	for _, w := range repo.Checkouts() {
		if facts.Gone[w.Path], err = s.Gone(ctx, w); err != nil {
			return Facts{}, interrupted(ctx, err)
		}
		if w.Branch() == "" {
			continue
		}
		record, err := s.Meta.Load(w.Branch())
		switch {
		case err == nil:
			facts.Records[w.Branch()] = record
		case !errors.Is(err, fs.ErrNotExist):
			facts.RecordErrs[w.Branch()] = err
		}
	}

	cwd, err := s.Located(ctx, params.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, params.Cwd)

	facts.Jobs, facts.JobsErr = s.Daemon.Jobs(ctx)
	facts.Owed = s.Owing()
	if skewed, ok := errors.AsType[*run.Skewed](facts.JobsErr); ok {
		facts.Skewed, facts.JobsErr = skewed, nil
	}
	return facts, ctx.Err()
}

// interrupted is err when it is the read's interruption, else ctx's error, nil unless ctx is done by then (repository.Interrupted).
func interrupted(ctx context.Context, err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	return ctx.Err()
}

// ----------------------------------------------------------------------------
// Before the start

// DaemonUnavailable is the code, after the command's name, of a start that could not begin (wtm's daemon of this build could not be made sure of), or of a stop whose jobs cannot be read.
const DaemonUnavailable = ".daemon_unavailable"

// Ensure makes sure of a daemon of this build, and is why no job could start when it could not, worded under command: a daemon of another build holding jobs, followed by restart, or the daemon not starting.
func (s Service) Ensure(ctx context.Context, command string, restart *domain.FollowUp) *domain.Error {
	err := s.Daemon.Ensure(ctx)
	if err == nil {
		return nil
	}
	if refusal := domain.SkewRefusal(domain.SkewRefusalParams{Command: command, Undone: "nothing started", Err: err, Restart: restart}); refusal != nil {
		return refusal
	}
	return domain.AsError(domain.AsErrorParams{Err: err, Code: command + DaemonUnavailable, Message: "couldn't reach wtm's daemon"})
}

// StopFailed is the code, after the command's name, of the warning of a worktree whose jobs an exclusive start could not stop, and of a worktree a job of which a stop could not stop.
const StopFailed = ".stop_failed"

// StopOthersParams are the command stopping, the other worktrees whose jobs stop, and the operation's correlation id, which wtm's daemon's events carry.
type StopOthersParams struct {
	Command       string
	Others        []Other
	CorrelationID string
}

// StopOthers stops the jobs of each of the other worktrees, its claims let go of, before an exclusive start; each worktree whose jobs could not stop is a warning.
func (s Service) StopOthers(ctx context.Context, params StopOthersParams) []domain.Warning {
	var warnings []domain.Warning
	for _, other := range params.Others {
		if _, err := s.Daemon.StopAll(ctx, run.StopAll{WorkDir: other.Path, CorrelationID: params.CorrelationID}); err != nil {
			warnings = append(warnings, domain.Warning{Code: params.Command + StopFailed, Message: "couldn't stop the jobs of " + other.Worktree + ": " + err.Error(), Params: domain.Params{"worktree": other.Worktree}})
		}
	}
	return warnings
}

// ----------------------------------------------------------------------------
// The start

// The codes of what becomes of a worktree, after the command's name: it could not be numbered, its environment could not be read, a job failed to start or a task failed, a service crashed once started.
const (
	Unnumbered     = ".unnumbered"
	EnvUnavailable = ".env_unavailable"
	JobFailure     = ".job_failed"
	Crashed        = ".crashed"
)

// The codes of the warnings of a worktree, after the command's name: a port nothing answered on, a namespace its record could not keep.
const (
	PortSilent          = ".port_silent"
	NamespaceUnrecorded = ".namespace_unrecorded"
)

// StartParams are the command starting, the facts it planned on, the worktree and its jobs as planned, the profile each job records it was started under (none: no profile), whether a restart was asked (each job's log then continues, a job up stopped first: ActionRestart), whether the ports go unchecked, the operation's correlation id, which the jobs' events carry, and where the events go.
type StartParams struct {
	Command       string
	Facts         Facts
	Target        Target
	Profile       string
	Restart       bool
	NoProbe       bool
	CorrelationID string
	Report        domain.Report
}

// Started is what became of the start in one worktree: its status and why, the error it failed on, its warnings, and each job of its plan with what became of it.
type Started struct {
	Status   domain.Status
	Reason   domain.Reason
	Err      *domain.Error
	Warnings []domain.Warning
	Jobs     []JobResult
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

// JobStatus is what became of one job of the start.
type JobStatus string

// The statuses of a job of the start.
const (
	JobStarted        JobStatus = "started"         // a service launched, or a stack its launcher brought up
	JobAlreadyRunning JobStatus = "already_running" // a service already up, left as it is
	JobJoined         JobStatus = "joined"          // a shared service the worktree holds
	JobDone           JobStatus = "done"            // a task that ran to its end
	JobFailed         JobStatus = "failed"          // a task that exited other than 0, or a job that could not start: the start of its worktree stops there
	JobCrashed        JobStatus = "crashed"         // a service that started then ended before the start did
	JobNotStarted     JobStatus = "not_started"     // a job the start of its worktree never reached
	JobStopped        JobStatus = "stopped"         // a job a restart stopped, cut short before it started it again
)

// JobStatuses returns every JobStatus.
func JobStatuses() []JobStatus {
	return []JobStatus{JobStarted, JobAlreadyRunning, JobJoined, JobDone, JobFailed, JobCrashed, JobNotStarted, JobStopped}
}

// defaultProbe is how long a start waits for the ports its services declare when run.toml does not say.
const defaultProbe = 15 * time.Second

// starting is one worktree's start under way: what it starts from, and what it became.
type starting struct {
	Service
	params   StartParams
	facts    Facts
	target   Target
	report   domain.Report
	started  Started
	launched time.Time // when the last job was launched
	ordinal  int
	env      map[string]string
	shared   *run.Shared
}

// Start starts the jobs of params' target, in order, between its unit's events: it numbers the worktree first when it has no ordinal, reads its environment, starts each job (a task to its end), stops at the first that fails, then checks the ports its services declare and that none of them ended since. Once ctx is cancelled, the jobs the daemon took keep running and the jobs not reached are not started; a worktree reached after a cancel is skipped.
func (s Service) Start(ctx context.Context, params StartParams) Started {
	t, report := params.Target, params.Report
	st := &starting{Service: s, params: params, facts: params.Facts, target: t, report: report, started: Started{Jobs: t.NotStarted()}}
	if ctx.Err() != nil {
		st.started.Status, st.started.Reason = domain.StatusSkipped, domain.ReasonNotReached
		return st.started
	}

	report(domain.UnitStarted{Subject: t.Worktree})
	defer func() { report(domain.UnitFinished{Subject: t.Worktree, Status: st.started.Status}) }()

	if err := st.prepare(ctx); err != nil {
		st.started.Status, st.started.Err = domain.StatusFailed, err
		return st.started
	}
	for i := range t.Jobs {
		if ctx.Err() != nil {
			st.started.Status, st.started.Reason = domain.StatusCancelled, domain.ReasonInterrupted
			return st.started
		}
		if !st.job(ctx, i) {
			return st.started
		}
	}

	st.check(ctx)
	switch {
	case st.started.Status != "":
	case ctx.Err() != nil:
		st.started.Status, st.started.Reason = domain.StatusCancelled, domain.ReasonInterrupted
	default:
		st.started.Status = domain.StatusDone
	}
	if st.started.Status == domain.StatusDone && !slices.ContainsFunc(st.started.Jobs, func(job JobResult) bool { return job.Status != JobAlreadyRunning }) {
		st.started.Reason = domain.ReasonUnchanged
	}
	return st.started
}

// prepare numbers the worktree when it has none, then reads its environment and, for a shared job, the main checkout's.
func (s *starting) prepare(ctx context.Context) *domain.Error {
	ordinal := s.facts.OrdinalOf(s.target)
	if !s.target.Numbered {
		numbered, err := s.Number(ctx, s.target.Branch)
		if err != nil {
			return domain.AsError(domain.AsErrorParams{Err: err, Code: s.params.Command + Unnumbered, Message: "couldn't number " + s.target.Worktree + ": its ports are unknown"})
		}
		ordinal = numbered
		// Its ports and namespaces were unknown to the plan: they are the ordinal's now.
		s.target.Numbered = true
		planned := s.target.Jobs
		s.target = s.facts.Planned(s.target, s.facts.jobsIn(s.target), ordinal)
		for i, job := range s.target.Jobs {
			s.started.Jobs[i].Namespace = job.Namespace
			// A restart is the command's choice, not Planned's: it outlives the numbering.
			if planned[i].Action == ActionRestart {
				s.target.Jobs[i].Action = ActionRestart
			}
		}
	}
	s.ordinal = ordinal

	var err error
	if s.env, err = env.ReadHookEnv(s.facts.jobParams(s.target, ordinal), s.target.Path, s.EnvFiles.Read); err != nil {
		return domain.AsError(domain.AsErrorParams{Err: err, Code: s.params.Command + EnvUnavailable, Message: "couldn't read " + s.target.Worktree + "'s environment"})
	}
	if !slices.ContainsFunc(s.target.Jobs, func(job JobPlan) bool { return s.facts.Shared(job.Name) }) {
		return nil
	}
	main := s.facts.mainTarget()
	mainEnv, err := env.ReadHookEnv(s.facts.jobParams(main, 0), main.Path, s.EnvFiles.Read)
	if err != nil {
		return domain.AsError(domain.AsErrorParams{Err: err, Code: s.params.Command + EnvUnavailable, Message: "couldn't read the main checkout's environment"})
	}
	s.shared = &run.Shared{WorkDir: main.Path, Env: mainEnv, LogDir: run.LogDir(s.StateDir, main.Branch)}
	return nil
}

// job starts the job at index of the worktree between its phase's events, its lines reported as they come, and reports whether the start goes on: a failure, or a cancel, stops it.
func (s *starting) job(ctx context.Context, index int) bool {
	planned, result := s.target.Jobs[index], &s.started.Jobs[index]
	declared, _ := s.facts.Declared(planned.Name)
	spec := s.spec(s.facts.effective(declared))

	var lines tail
	result.Log = s.logFile(spec)
	s.report(domain.PhaseStarted{Subject: s.target.Worktree, Phase: domain.PhaseJob, Job: planned.Name, Log: result.Log})
	// A shared service is restarted by the daemon in place, every hold on it kept: a stop here would let go of the worktree's hold.
	if planned.Action == ActionRestart && !s.facts.Shared(planned.Name) {
		// A stop sent is let finish, as run stop's: a stack stopped half-way is worse than one stopped or not.
		if _, err := s.Daemon.Stop(context.WithoutCancel(ctx), run.Stop{Name: planned.Name, WorkDir: s.target.Path, CorrelationID: s.params.CorrelationID}, nil); err != nil {
			s.report(domain.PhaseFinished{Subject: s.target.Worktree, Phase: domain.PhaseJob, Job: planned.Name, Failed: true})
			s.started.Status, s.started.Err = domain.StatusFailed, s.jobFailure(planned.Name, err, nil)
			result.Status = JobFailed
			// Said running on only once the daemon lists it up: a stop whose answer was lost may have stopped it.
			if now, listed := s.listedNow(planned.Name); listed && now.Up {
				result.Status, result.URL = JobAlreadyRunning, s.facts.urlOf(planned.Name, s.target.Path)
				s.started.Err.Message = planned.Name + " runs on: couldn't stop it to start it again: " + err.Error()
			}
			return false
		}
		if ctx.Err() != nil {
			// Cut short once stopped: started again by nobody, never said crashed.
			s.report(domain.PhaseFinished{Subject: s.target.Worktree, Phase: domain.PhaseJob, Job: planned.Name})
			result.Status = JobStopped
			s.started.Status, s.started.Reason = domain.StatusCancelled, domain.ReasonInterrupted
			return false
		}
		// A restart keeps the profile the job was started under, whoever restarts it.
		if before, listed := s.listing(s.facts.Jobs, planned.Name); listed {
			spec.Profile = cmp.Or(spec.Profile, before.Profile)
		}
	}
	// A restart asked continues the job's log, a job it found crashed or ended included.
	spec.Restart = s.params.Restart || planned.Action == ActionRestart
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
		result.Status, result.URL = JobAlreadyRunning, s.facts.urlOf(planned.Name, s.target.Path)
	case err != nil && ctx.Err() != nil:
		// The start was cut short: the daemon says whether it took it, and what it took keeps running.
		s.started.Status, s.started.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		now, ran := s.ran(planned.Name)
		switch {
		case ran:
			s.became(result, now, declared)
			if result.Status == JobJoined {
				s.recordNamespace(ctx, declared)
			}
		case planned.Action == ActionRestart:
			result.Status = JobStopped
		}
		return false
	case err != nil:
		result.Status, result.Lines = JobFailed, lines
		if refused != nil {
			result.ExitCode = refused.ExitCode
		}
		s.started.Status, s.started.Err = domain.StatusFailed, s.jobFailure(planned.Name, err, result.ExitCode)
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

// ranWithin bounds how long a start cut short asks the daemon whether it ran the job.
const ranWithin = 2 * time.Second

// ran returns the job of name as the daemon now lists it in the worktree, and whether this start ran it: listed, and not as it stood when the start was planned (a task that ran to its end within the start is listed ended, no longer up).
func (s *starting) ran(name string) (run.Job, bool) {
	now, listed := s.listedNow(name)
	before, was := s.listing(s.facts.Jobs, name)
	// A job listed stopped was stopped, by a restart's stop: no start ran it.
	return now, listed && now.State != run.Stopped && (!was || !reflect.DeepEqual(now, before))
}

// listedNow is the job of name as the daemon lists it in the worktree now, asked within ranWithin, and whether it lists it.
func (s *starting) listedNow(name string) (run.Job, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), ranWithin)
	defer cancel()
	jobs, err := s.Daemon.Jobs(ctx)
	if _, skewed := errors.AsType[*run.Skewed](err); err != nil && !skewed {
		return run.Job{}, false
	}
	return s.listing(jobs, name)
}

// listing is the job of name jobs list in the worktree, and whether they list it.
func (s *starting) listing(jobs []run.Job, name string) (run.Job, bool) {
	at := slices.IndexFunc(jobs, func(job run.Job) bool { return job.Name == name && job.WorkDir == s.target.Path })
	if at < 0 {
		return run.Job{}, false
	}
	return jobs[at], true
}

// became is what the job the start ran became, as the daemon lists it now: up, started or joined; a task ended, done or failed by its exit code; a service ended, crashed.
func (s *starting) became(result *JobResult, now run.Job, declared project.Job) {
	result.ExitCode = now.ExitCode
	switch {
	case now.Up && declared.Scope == project.Shared && !s.target.Main:
		result.Status = JobJoined
	case now.Up:
		result.Status, result.ExitCode = JobStarted, nil
	case declared.Kind == project.Task && (now.ExitCode == nil || *now.ExitCode == 0):
		result.Status = JobDone
	case declared.Kind == project.Task:
		result.Status = JobFailed
	default:
		result.Status = JobCrashed
	}
}

func (s *starting) jobFailure(job string, err error, exitCode *int) *domain.Error {
	params := domain.Params{"job": job}
	if exitCode != nil {
		params["exit_code"] = strconv.Itoa(*exitCode)
	}
	return &domain.Error{Kind: domain.KindInternal, Code: s.params.Command + JobFailure, Message: job + " failed: " + err.Error(), Params: params}
}

// spec is what the daemon is asked to start job with: the worktree's environment, its log directory, the names the proxy serves it under, the main checkout of a shared job, and the origin its events name.
func (s *starting) spec(job project.Job) run.Spec {
	spec := run.Spec{
		Job:     job,
		Profile: s.params.Profile,
		WorkDir: s.target.Path,
		Env:     withPorts(s.env, s.facts.jobParams(s.target, s.ordinal).JobPorts(job)),
		Unset:   env.NotInherited(s.facts.Run),
		LogDir:  run.LogDir(s.StateDir, s.target.Branch),
		Origin:  &run.Origin{Root: s.facts.MainPath, CommonDir: s.CommonDir, Branch: s.target.Branch, CorrelationID: s.params.CorrelationID},
		Routes:  s.routes(job),
	}
	if job.Scope == project.Shared && s.shared != nil {
		shared := *s.shared
		shared.Env = withPorts(shared.Env, s.facts.jobParams(s.facts.mainTarget(), 0).JobPorts(job))
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
	repo := filepath.Base(s.facts.MainPath)
	for _, routed := range append([]project.Job{job}, s.facts.children(job)...) {
		host := env.RouteHost(env.RouteHostParams{Job: routed, Slug: s.env[env.WorktreeKey], Project: repo})
		if host != "" {
			routes = append(routes, run.Route{Job: routed.Name, Host: host, Port: routed.URL.Port})
		}
	}
	return routes
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
		s.started.Warnings = append(s.started.Warnings, domain.Warning{Code: s.params.Command + NamespaceUnrecorded, Message: s.target.Worktree + "'s record does not keep its namespace of " + job.Name + ", which clean will not drop: " + err.Error(), Params: domain.Params{"worktree": s.target.Worktree, "job": job.Name}})
	}
}

// check waits for the ports the services started declare to answer, the check run.toml and the request allow, and at least a moment since the last launch for a service that fails at once to end, then asks the daemon which of them ended since: each fails the worktree, crashed; each port nothing answered on is a warning.
func (s *starting) check(ctx context.Context) {
	var probed []int
	for i, job := range s.target.Jobs {
		result := s.started.Jobs[i]
		declared, _ := s.facts.Declared(job.Name)
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
	for i := range s.started.Jobs {
		result := &s.started.Jobs[i]
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
		if s.started.Err == nil {
			s.started.Status, s.started.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: s.params.Command + Crashed, Message: result.Name + " crashed once started", Params: domain.Params{"job": result.Name}}
		}
	}
}

// probeBudget is how long the ports are waited for: run.toml's port_probe_timeout, 15 seconds when it says none, none when it is below 0 or the request skips the check.
func (s *starting) probeBudget() time.Duration {
	switch timeout := s.facts.Run.PortProbeTimeout; {
	case s.params.NoProbe, timeout < 0:
		return 0
	case timeout == 0:
		return defaultProbe
	default:
		return time.Duration(timeout) * time.Second
	}
}

// markPorts records which ports of the services started answered, and warns of each job one of whose ports none did; a job run.toml keeps out of the check (probe = false, binds_no_port) is left unchecked.
func (s *starting) markPorts(answered map[int]bool) {
	for i := range s.started.Jobs {
		result := &s.started.Jobs[i]
		if declared, _ := s.facts.Declared(result.Name); declared.NoProbe || declared.BindsNoPort {
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
			s.started.Warnings = append(s.started.Warnings, domain.Warning{Code: s.params.Command + PortSilent, Message: result.Name + " started, but nothing answers on " + strings.Join(silent, ", "), Params: domain.Params{"worktree": s.target.Worktree, "job": result.Name, "ports": strings.Join(silent, ", ")}})
		}
	}
}

// ----------------------------------------------------------------------------
// Helpers

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
