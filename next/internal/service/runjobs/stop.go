package runjobs

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// What a stop decides from

// Running is what a stop decides from, as the command stopping read it: the project's config, its run.toml (with why it does not load), the repository's snapshot, the worktrees whose directory is gone, where the caller stands, where each name asked as a path leads, and the jobs wtm's daemon runs (with a daemon of another build that answered, or Unknown: none answers and its index holds processes whose fate only a daemon can tell).
type Running struct {
	worktree.Repository
	Gone    map[string]bool   // by path
	Cwd     string            // where the caller stands, through its symbolic links
	Located map[string]string // where each name asked that is no worktree's name leads, from where the caller stands, as git spells the worktree root it is; whether or not something is there
	Jobs    []run.Job
	Skewed  *run.Skewed
	Unknown bool
}

// ----------------------------------------------------------------------------
// The reads of a stop

// Stops reads what a stop decides from, for run down and run stop: wtm's daemon, whether a worktree's directory is gone, the path a name stands for, and where each name asked as a path leads. Every field must be set.
type Stops struct {
	Daemon  StopDaemon
	Gone    func(ctx context.Context, w worktree.Worktree) (bool, error)                                                              // repository.Gone
	Located func(ctx context.Context, name string, caller domain.Caller) (string, error)                                              // repository.Located
	Locate  func(ctx context.Context, worktrees []worktree.Worktree, names []string, caller domain.Caller) (map[string]string, error) // repository.Locate
}

// StopDaemon is wtm's daemon as a stop asks it: the jobs it runs, one of this build made sure of to reach the processes its index holds, and a job stopped.
type StopDaemon interface {
	Lister
	Stopper
	Ensure(ctx context.Context) error
}

// ObserveStopParams are the command observing, whose codes the refusals give, the repository as it observed it (repository.Service.Observe), the worktrees the request names, where its caller stands, and the follow-up that restarts a daemon of another build.
type ObserveStopParams struct {
	Command    string
	Repository worktree.Repository
	Names      []string
	Caller     domain.Caller
	Restart    *domain.FollowUp
}

// Observe reads, on the repository of params, what a stop decides from: where each name asked as a path leads, which worktrees' directory is gone, where the caller stands, and the jobs wtm's daemon runs, never starting it: with none answering and processes in its index, the jobs are Unknown. A daemon that cannot be read is refused with <command>.daemon_skew (another protocol, followed by its restart) or <command>.daemon_unavailable.
func (s Stops) Observe(ctx context.Context, params ObserveStopParams) (Running, error) {
	repo := params.Repository
	running := Running{Repository: repo, Gone: map[string]bool{}}
	var err error
	if running.Located, err = s.Locate(ctx, repo.Worktrees, params.Names, params.Caller); err != nil {
		return Running{}, err
	}
	for _, w := range repo.Checkouts() {
		if running.Gone[w.Path], err = s.Gone(ctx, w); err != nil {
			return Running{}, interrupted(ctx, err)
		}
	}

	cwd, err := s.Located(ctx, params.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Running{}, err
	}
	running.Cwd = cmp.Or(cwd, params.Caller.Cwd)

	if running, err = running.Listed(ctx, s.Daemon); err != nil {
		return Running{}, daemonFailure(params, err, "couldn't read the jobs wtm's daemon runs")
	}
	return running, ctx.Err()
}

// Reach is Observe for a plan: a daemon that died leaving processes in its index is started to take them over, they being what the stop must reach, then the facts are read again; one started that still cannot tell is refused with <command>.daemon_unavailable. A form never reaches: it only observes.
func (s Stops) Reach(ctx context.Context, params ObserveStopParams) (Running, error) {
	running, err := s.Observe(ctx, params)
	if err != nil || !running.Unknown {
		return running, err
	}
	if err := s.Daemon.Ensure(ctx); err != nil {
		return Running{}, daemonFailure(params, err, "couldn't start wtm's daemon to reach the jobs its index holds")
	}
	if running, err = s.Observe(ctx, params); err != nil {
		return Running{}, err
	}
	if running.Unknown {
		return Running{}, &domain.Error{Kind: domain.KindInternal, Code: params.Command + DaemonUnavailable, Message: "wtm's daemon started cannot tell which of the jobs its index holds are up"}
	}
	return running, nil
}

// daemonFailure is why a stop's jobs cannot be reached: a daemon of another protocol, or one holding jobs the client cannot go on with, followed by its restart; else the daemon unavailable, saying message.
func daemonFailure(params ObserveStopParams, err error, message string) *domain.Error {
	if refusal := domain.SkewRefusal(domain.SkewRefusalParams{Command: params.Command, Undone: "nothing stopped", Err: err, Restart: params.Restart}); refusal != nil {
		return refusal
	}
	return domain.AsError(domain.AsErrorParams{Err: err, Code: params.Command + DaemonUnavailable, Message: message})
}

// Listed is r with the jobs daemon runs, never starting it: a daemon of another build that answered is Skewed, none answering with processes in its index is Unknown; it fails with what else kept the daemon from answering.
func (r Running) Listed(ctx context.Context, daemon Lister) (Running, error) {
	jobs, err := daemon.Jobs(ctx)
	r.Jobs, r.Skewed, r.Unknown = jobs, nil, false
	if skewed, ok := errors.AsType[*run.Skewed](err); ok {
		r.Skewed, err = skewed, nil
	}
	if errors.Is(err, run.ErrUnknown) {
		r.Unknown, err = true, nil
	}
	return r, err
}

// Standing is the worktree the caller stands in, by its name; none when it stands in none.
func (r Running) Standing() string {
	worktrees := r.standingWorktrees()
	if at := worktree.Holding(worktrees, r.Cwd, func(w worktree.Worktree) string { return w.Path }); at >= 0 {
		return worktrees[at].Name()
	}
	return ""
}

func (r Running) standingWorktrees() []worktree.Worktree {
	return slices.DeleteFunc(r.MainFirst(), func(w worktree.Worktree) bool { return r.Gone[w.Path] })
}

// WorktreeOptions are the worktrees a stop may reach: each one whose directory stands, the main one first, the caller's tagged current, each with what it runs.
func (r Running) WorktreeOptions() []domain.Option {
	worktrees := r.standingWorktrees()
	return domain.RunningIn(domain.WorktreeOptions(worktrees, domain.WorktreeQuery{Cwd: r.Cwd}), worktrees, r.Jobs)
}

// nameAt is how the worktree at path is named: its name, else the path itself (a worktree removed whose jobs still run).
func (r Running) nameAt(path string) string {
	if at := slices.IndexFunc(r.Worktrees, func(w worktree.Worktree) bool { return w.Path == path }); at >= 0 {
		return r.Worktrees[at].Name()
	}
	return path
}

// ----------------------------------------------------------------------------
// The worktrees a stop reaches

// A StopTarget is a worktree whose jobs stop: its name, its branch (none on no branch, or for a worktree removed), its root as git spells it, whether git no longer lists it (Removed: a worktree removed whose jobs still run, named by its path), and the jobs the stop reaches.
type StopTarget struct {
	Worktree string    `json:"worktree"`
	Branch   string    `json:"branch,omitempty"`
	Path     string    `json:"path"`
	Removed  bool      `json:"removed,omitempty"`
	Jobs     []StopJob `json:"jobs"`
}

// A StopJob is how one job of a worktree stops: its kind, what of it the daemon holds, its fate, the worktrees that keep holding a shared service it releases, the command a stack stops with as the daemon recorded it, the main checkout a shared service runs in, and, for a shared service's own run whose every hold the stop lets go of, the worktrees that lose it (HeldBy).
type StopJob struct {
	Name        string          `json:"name"`
	Kind        project.JobKind `json:"kind,omitempty"`
	Hold        run.Hold        `json:"hold"`
	Fate        run.Fate        `json:"fate"`
	KeptBy      []string        `json:"kept_by,omitempty"`
	StopCommand string          `json:"stop_command,omitempty"`
	SharedIn    string          `json:"shared_in,omitempty"`
	HeldBy      []string        `json:"held_by,omitempty"`
}

// Shared reports whether the job is a hold on a shared service, let go of after the worktree's own jobs.
func (j StopJob) Shared() bool {
	return j.SharedIn != ""
}

// service is the shared service the job holds: its name in the main checkout it runs in.
func (j StopJob) service() string {
	return j.SharedIn + " " + j.Name
}

// Entries decides each worktree names asks a stop of: a StopTarget, or the problem of its entry: blank, given twice (under one spelling or two), or naming neither a worktree of the repository (by its name, else the path it leads to) nor a worktree removed whose jobs still run.
func (r Running) Entries(names []string) (targets []StopTarget, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		target, found := r.target(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name) || found && slices.ContainsFunc(targets, func(t StopTarget) bool { return t.Path == target.Path }):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found:
			targets = append(targets, target)
		case r.unattributed(r.Located[name]):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree: wtm's daemon runs jobs there without saying which repository they were started for", Params: domain.Params{"value": name}})
		default:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		}
	}
	return targets, problems
}

// target is the worktree name names, by its name, else the path it leads to: one of the repository, or one removed whose jobs still run.
func (r Running) target(name string) (StopTarget, bool) {
	located := r.Located[name]
	if at := slices.IndexFunc(r.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name || located != "" && w.Path == located }); at >= 0 {
		w := r.Worktrees[at]
		return StopTarget{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path}, true
	}
	if r.removed(located) {
		return StopTarget{Worktree: located, Path: located, Removed: true}, true
	}
	return StopTarget{}, false
}

// removed reports whether path is a worktree of this repository git no longer lists whose jobs still run: the daemon runs a job there started for this repository. A path of another repository's is never one, the daemon's listing being the machine's.
func (r Running) removed(path string) bool {
	return path != "" && slices.ContainsFunc(r.Jobs, func(job run.Job) bool { return job.WorkDir == path && job.Repo == r.MainPath })
}

// unattributed reports whether wtm's daemon runs a job at path whose start named no repository (indexed by a wtm that recorded none): a stop cannot tell it is this repository's.
func (r Running) unattributed(path string) bool {
	return path != "" && slices.ContainsFunc(r.Jobs, func(job run.Job) bool { return job.WorkDir == path && job.Repo == "" })
}

// Every is each worktree of the repository wtm's daemon lists a job of, the main one first, then each removed one whose jobs still run, by path.
func (r Running) Every() []StopTarget {
	var targets []StopTarget
	for _, w := range r.MainFirst() {
		if slices.ContainsFunc(r.Jobs, func(job run.Job) bool { return job.WorkDir == w.Path }) {
			targets = append(targets, StopTarget{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path})
		}
	}

	var removed []string
	for _, job := range r.Jobs {
		listed := slices.ContainsFunc(r.Worktrees, func(w worktree.Worktree) bool { return w.Path == job.WorkDir })
		if !listed && !slices.Contains(removed, job.WorkDir) && r.removed(job.WorkDir) {
			removed = append(removed, job.WorkDir)
		}
	}
	slices.Sort(removed)
	for _, path := range removed {
		targets = append(targets, StopTarget{Worktree: path, Path: path, Removed: true})
	}
	return targets
}

// Stopping is each of targets with what their stop does to each of their jobs, names' alone when names are set (nil: every job): decided over every target at once (run.StoppingIn), so the holds of several on one shared service are let go of together.
func (r Running) Stopping(targets []StopTarget, names []string) []StopTarget {
	dirs := make([]string, 0, len(targets))
	for _, t := range targets {
		dirs = append(dirs, t.Path)
	}
	byDir := map[string][]run.Stopping{}
	for _, s := range run.StoppingIn(r.Jobs, dirs, names) {
		byDir[s.Job.WorkDir] = append(byDir[s.Job.WorkDir], s)
	}

	planned := make([]StopTarget, 0, len(targets))
	for _, t := range targets {
		t.Jobs = []StopJob{}
		for _, s := range byDir[t.Path] {
			t.Jobs = append(t.Jobs, r.stopJob(s))
		}
		planned = append(planned, t)
	}
	return planned
}

func (r Running) stopJob(s run.Stopping) StopJob {
	job := s.Job
	planned := StopJob{Name: job.Name, Kind: job.Kind, Hold: job.Hold, Fate: s.Fate, StopCommand: job.StopCommand}
	for _, holder := range s.KeptBy {
		planned.KeptBy = append(planned.KeptBy, r.nameAt(holder))
	}
	if job.SharedDir != "" {
		planned.SharedIn = r.nameAt(job.SharedDir)
	}
	return planned
}

// FatesApproximate is the code, after the command's name, of a stop's plan a daemon of another build answered that does not say whether the main checkout holds a shared service: the fates of the shared services are then approximate.
const FatesApproximate = ".fates_approximate"

// StopWarningsParams are the command stopping, the worktrees it stops the jobs of, and the follow-up that restarts a daemon of another build.
type StopWarningsParams struct {
	Command string
	Targets []StopTarget
	Restart *domain.FollowUp
}

// StopWarnings are what a stop of params' targets cannot see to: a daemon of another build answered (domain.SkewWarning), and, when one of the targets holds a shared service whether the main checkout holds which cannot be told (run.Job.MainHoldsUnknown), that the fates are approximate.
func (r Running) StopWarnings(params StopWarningsParams) []domain.Warning {
	if r.Skewed == nil {
		return nil
	}
	warnings := []domain.Warning{domain.SkewWarning(domain.SkewWarningParams{Command: params.Command, Skewed: r.Skewed, Restart: params.Restart})}
	if r.approximate(params.Targets) {
		warnings = append(warnings, domain.Warning{Code: params.Command + FatesApproximate, Message: "a daemon of another build does not say whether the main checkout holds a shared service: which ones stop and which run on is approximate"})
	}
	return warnings
}

func (r Running) approximate(targets []StopTarget) bool {
	return slices.ContainsFunc(r.Jobs, func(service run.Job) bool {
		return service.Up && service.MainHoldsUnknown && slices.ContainsFunc(r.Jobs, func(hold run.Job) bool {
			return hold.Up && hold.Name == service.Name && hold.SharedDir == service.SharedDir && slices.ContainsFunc(targets, func(t StopTarget) bool { return t.Path == hold.WorkDir })
		})
	})
}

// ----------------------------------------------------------------------------
// The recap

// StopWords is how a recap names one job a stop reaches: its name, and what the stop does to it unless it just stops.
func StopWords(job StopJob) string {
	switch {
	case job.Fate == run.FateRelease:
		return job.Name + " (released, kept by " + strings.Join(job.KeptBy, ", ") + ")"
	case job.Fate == run.FateClear:
		return job.Name + " (crashed, cleared)"
	case job.Shared():
		return job.Name + " (stops in " + job.SharedIn + ": no worktree keeps it)"
	}
	return job.Name
}

// ----------------------------------------------------------------------------
// The stop

// StopStatus is what became of one job of a stop.
type StopStatus string

// The statuses of a job of a stop.
const (
	StopStopped    StopStatus = "stopped"     // the job was up and is stopped, a shared service by its last hold included
	StopReleased   StopStatus = "released"    // the worktree let go of a shared service others still hold, which runs on
	StopCleared    StopStatus = "cleared"     // a job that had crashed, cleared from the listing
	StopNotRunning StopStatus = "not_running" // a job not up there: ended since the plan, or one asked that never ran
	StopFailure    StopStatus = "failed"      // a stop the daemon could not make: the job runs on
	StopNotReached StopStatus = "not_reached" // a stop never sent: the run was cancelled first
)

// StopStatuses returns every StopStatus.
func StopStatuses() []StopStatus {
	return []StopStatus{StopStopped, StopReleased, StopCleared, StopNotRunning, StopFailure, StopNotReached}
}

// A StopDetail is, as the JSON envelope reports it, the stop in one worktree: where, and each job it reached with what became of it.
type StopDetail struct {
	Path string       `json:"path"`
	Jobs []StopResult `json:"jobs"`
}

// A StopResult is what became of one job: its status and, for a stop that failed, the daemon's reason.
type StopResult struct {
	Name    string     `json:"name"`
	Status  StopStatus `json:"status"`
	Message string     `json:"message,omitempty"`
}

// A Stopped is what became of the stop in one worktree: its status and why, the error it failed on, and its StopDetail.
type Stopped = domain.Unit[StopDetail]

// StopParams are the command stopping, whose codes the items give, the worktrees whose jobs stop as planned, the operation's correlation id, which the daemon's job.stopped events carry, and where the events go.
type StopParams struct {
	Command       string
	Targets       []StopTarget
	CorrelationID string
	Report        domain.Report
}

// Stop stops the jobs of each of params' targets, the worktrees at once, each between its unit's events: its own jobs one after the other, then its holds on shared services, as the daemon's own stop of a worktree does, each between the events of its phase, the lines a stack's stop command prints reported in it; a stop sent is let finish whatever ctx, and once ctx is cancelled the stops not sent are not reached. A worktree whose stop fails stops none of the others; one reaching no job is unchanged, with no event. Each job reads what the daemon answered, the holds on one shared service let go of together each stopped once one of them stopped it. The items keep the targets' order.
func Stop(ctx context.Context, daemon Stopper, params StopParams) []Stopped {
	// The worktrees report from several goroutines; a surface draws one event at a time.
	var mu sync.Mutex
	serial := func(event domain.Event) {
		mu.Lock()
		defer mu.Unlock()
		params.Report(event)
	}
	in := params
	in.Report = serial

	items := make([]Stopped, len(params.Targets))
	var wg sync.WaitGroup
	for i, t := range params.Targets {
		wg.Go(func() { items[i] = stopIn(ctx, daemon, in, t) })
	}
	wg.Wait()

	return settled(params.Targets, items)
}

// NotReached is the detail of t before its stop: each job of its plan, none stopped.
func (t StopTarget) NotReached() *StopDetail {
	detail := &StopDetail{Path: t.Path, Jobs: make([]StopResult, 0, len(t.Jobs))}
	for _, job := range t.Jobs {
		detail.Jobs = append(detail.Jobs, StopResult{Name: job.Name, Status: StopNotReached})
	}
	return detail
}

func stopIn(ctx context.Context, daemon Stopper, params StopParams, t StopTarget) Stopped {
	item := Stopped{Subject: t.Worktree, Detail: t.NotReached()}
	switch {
	case ctx.Err() != nil:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonNotReached
		return item
	case len(t.Jobs) == 0:
		item.Status, item.Reason = domain.StatusDone, domain.ReasonUnchanged
		return item
	}

	params.Report(domain.UnitStarted{Subject: t.Worktree})
	defer func() { params.Report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()

	// Its own jobs first, then its holds: whether a shared service stops depends on the holds the others leave.
	for _, shared := range []bool{false, true} {
		for i, job := range t.Jobs {
			if job.Shared() == shared {
				item.Detail.Jobs[i] = stopJob(ctx, daemon, params, t, job)
			}
		}
	}

	item = finished(params.Command, item)
	return item
}

func stopJob(ctx context.Context, daemon Stopper, params StopParams, t StopTarget, job StopJob) StopResult {
	if ctx.Err() != nil {
		return StopResult{Name: job.Name, Status: StopNotReached}
	}

	params.Report(domain.PhaseStarted{Subject: t.Worktree, Phase: domain.PhaseJob, Job: job.Name})
	// A stop sent is let finish: a stack stopped half-way is worse than one stopped or not.
	outcome, err := daemon.Stop(context.WithoutCancel(ctx), run.Stop{Name: job.Name, WorkDir: t.Path, CorrelationID: params.CorrelationID}, func(line run.Line) {
		params.Report(domain.Output{Subject: t.Worktree, Job: job.Name, Line: line.Text})
	})
	params.Report(domain.PhaseFinished{Subject: t.Worktree, Phase: domain.PhaseJob, Job: job.Name, Failed: err != nil})
	if err != nil {
		return StopResult{Name: job.Name, Status: StopFailure, Message: err.Error()}
	}
	return StopResult{Name: job.Name, Status: statusOf(job.Fate, outcome)}
}

// statusOf is what a stop that answered outcome did to a job planned to meet fate: what the daemon answered, a crash it cleared (answered not running) said cleared.
func statusOf(fate run.Fate, outcome run.Outcome) StopStatus {
	switch {
	case outcome == run.OutcomeStopped:
		return StopStopped
	case fate == run.FateClear:
		return StopCleared
	case outcome == run.OutcomeNotRunning:
		return StopNotRunning
	case outcome == run.OutcomeReleased:
		return StopReleased
	}
	return StopStopped
}

// finished is item once each of its stops ended: failed naming the jobs that would not stop, cancelled when the run was cut short before every stop was sent, else done.
func finished(command string, item Stopped) Stopped {
	var failed, reasons []string
	for _, job := range item.Detail.Jobs {
		if job.Status == StopFailure {
			failed = append(failed, job.Name)
			reasons = append(reasons, job.Name+": "+job.Message)
		}
	}
	switch {
	case len(failed) > 0:
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: command + StopFailed, Message: "couldn't stop " + strings.Join(reasons, "; "), Params: domain.Params{"jobs": strings.Join(failed, ", ")}}
	case slices.ContainsFunc(item.Detail.Jobs, func(job StopResult) bool { return job.Status == StopNotReached }):
		item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
	default:
		item.Status = domain.StatusDone
	}
	return item
}

// settled is items once the holds the run let go of together are told apart: of the holds on one shared service planned to stop it, whichever lets go first answers released, the service stopping at the last; each reads stopped once one of them answered stopped. A service someone else came to hold meanwhile was released by them all.
func settled(targets []StopTarget, items []Stopped) []Stopped {
	stopped := map[string]bool{}
	for i, t := range targets {
		for j, job := range t.Jobs {
			if job.Shared() && items[i].Detail.Jobs[j].Status == StopStopped {
				stopped[job.service()] = true
			}
		}
	}
	for i, t := range targets {
		for j, job := range t.Jobs {
			if result := &items[i].Detail.Jobs[j]; job.Shared() && job.Fate == run.FateStop && result.Status == StopReleased && stopped[job.service()] {
				result.Status = StopStopped
			}
		}
	}
	return items
}

// StoppedOrReleased reports whether the stop in one worktree stopped or released a job, what a start again follows.
func StoppedOrReleased(item Stopped) bool {
	return slices.ContainsFunc(item.Detail.Jobs, func(job StopResult) bool { return job.Status == StopStopped || job.Status == StopReleased })
}

// ProfileOf is the profile t's jobs were started under: that of its own jobs, else that of the holds on shared services its stop reaches when those are all it runs (never a shared service it runs for the others alone); none for jobs started alone.
func (r Running) ProfileOf(t StopTarget) string {
	profile := run.ProfileIn(r.Jobs, t.Path)
	for _, job := range r.Jobs {
		if job.Up && job.WorkDir == t.Path && slices.ContainsFunc(t.Jobs, func(stopped StopJob) bool { return stopped.Name == job.Name }) {
			profile = cmp.Or(profile, job.Profile)
		}
	}
	return profile
}
