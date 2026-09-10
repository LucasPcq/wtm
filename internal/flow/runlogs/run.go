package runlogs

import (
	"context"
	"fmt"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/detect"
)

type Outcome struct {
	// WorkDir and Worktree name the worktree this run acted on: its path as git
	// spells it, and the branch a recap shows. Both are empty for a run whose
	// caller never said.
	WorkDir  string
	Worktree string
	// Profile names the profile this run started, empty when the caller started
	// a job list of its own. A surface that cannot say which profile it just
	// brought up leaves the reader to guess between several (LUC-208).
	Profile string
	Results []domain.JobActionResult
	// Started names the jobs left running when the sequence ended — after an
	// abort, the ones nothing tore down.
	Started []string
	// Completed names the tasks that ran to the end.
	Completed []string
	// NotStarted names the jobs the sequence never reached.
	NotStarted []string
	// Failed names the job that ended the sequence early, FailedStep placing it
	// among Steps. No Failed means every job was reached.
	Failed     string
	FailedStep int
	Steps      int
	// FailedOutput is what the job that ended the sequence had written, raw. A
	// surface that never showed it live — machine output, a CI log, an agent
	// reading JSON — has nothing else to say why the run stopped, and the
	// daemon's Reason alone ("task migrate failed: exit status 1") does not.
	FailedOutput []byte
	// FailedExitCode is the code the daemon reported for it, nil when it never
	// got as far as running.
	FailedExitCode *int
	// Probes is what the port check observed, empty when no Prober was installed
	// or when the sequence aborted before there was anything to check.
	Probes []domain.PortProbe
	// Crashed are the jobs that were accepted and had died before the sequence
	// ended. They are not an abort — the run went on — but they are the one
	// thing a "started" line must never be printed over.
	Crashed []domain.JobExit
}

func (o Outcome) Aborted() bool { return o.Failed != "" }

// Recorded reports that the run got far enough to have an account of itself. A
// zero Outcome is a run that never gave one — a surface detached from before
// the first job, or a refusal ahead of it — and it must not replace the account
// a surface already has.
func (o Outcome) Recorded() bool { return o.Steps > 0 || o.Aborted() }

// Outcomes is what a run over several worktrees concluded, one entry per
// worktree in the order they were selected. A run over one is an Outcomes of
// one: the surfaces read the arity rather than branching on a mode.
type Outcomes []Outcome

// Aborted reports that at least one worktree stopped short. The worktrees are
// isolated by construction, so one aborting says nothing about the others —
// only about the exit code the command owes its caller.
func (o Outcomes) Aborted() bool {
	for _, outcome := range o {
		if outcome.Aborted() {
			return true
		}
	}
	return false
}

// Recorded reports that at least one worktree gave an account of itself, which
// is what makes a recap worth printing.
func (o Outcomes) Recorded() bool {
	for _, outcome := range o {
		if outcome.Recorded() {
			return true
		}
	}
	return false
}

// One is the single outcome of a run over one worktree, and the zero value for
// any other arity. It is what the surfaces whose shape follows the arity read
// (LUC-198).
func (o Outcomes) One() Outcome {
	if len(o) != 1 {
		return Outcome{}
	}
	return o[0]
}

type RunParams struct {
	Service Service
	Sink    Sink
	Jobs    []domain.JobConfig
	// Declared is every job run.toml holds, handed over by the surface the way
	// BaseOwners is: a runner publishes the names of the jobs it starts itself,
	// and only the declaration knows who they are. Empty falls back to Jobs.
	Declared []domain.JobConfig
	// Profile is what the surface resolved Jobs from, carried through to the
	// Outcome so a recap can name it.
	Profile string
	WorkDir string
	// Worktree is WorkDir's branch, carried through to the Outcome and to every
	// Event so a merged report can name where a step happened.
	Worktree string
	LogDir   string
	// Env is what every job in this run learns about the worktree it belongs to,
	// resolved by the surface — the only side that can ask git.
	Env map[string]string
	// Prober checks that the ports the jobs declared were actually bound. Nil
	// skips the check entirely, which is what --no-probe and a zero budget do.
	Prober Prober
	// Project names the repository, the last segment of every route. Empty
	// leaves the jobs on their own ports.
	Project string
	// ProxyPort is where the proxy serves those routes. Zero means it is off,
	// and a job's URL is then its own address.
	ProxyPort int
	// PortAddressed says this worktree's .env still spells its addresses as
	// ports; see runner.portAddressed.
	PortAddressed bool
	// NextConfig reads a job's next.config.*, so the run can say what a Next
	// project is missing before its own name reaches it. Nil skips the check.
	NextConfig NextConfigLookup
	// BaseOwners names the worktree bound to a base port, so a silent resolved
	// port is not blamed on a command that read its variable fine. The run does
	// not build it: it reads neither the repository's config nor the daemon's
	// index across worktrees.
	BaseOwners map[int]string
	// Shared is where this run's shared jobs run: the main checkout, with its
	// own environment and log directory. Resolved by the surface, the only side
	// that can ask git which worktree is the main one. Nil leaves a shared job
	// with nowhere to run, and the daemon refuses it rather than starting one
	// instance per worktree.
	Shared *domain.SharedJobContext
}

// Run starts a profile's jobs in their declared order and reports each step to
// the Sink. A job that fails ends the sequence, leaving what is already running
// alone — the fix-and-retry loop keeps docker and databases warm — and that
// partial state is the Outcome, not an error: whether it exits non-zero, and
// what it prints, belongs to the surface.
//
// Cancelling ctx stops the reporting, not the jobs. A surface the user detached
// from is gone, and there is nobody left to emit to; what the daemon is running
// keeps running, the sequence stops where it stands, and the jobs it never
// reached come back as NotStarted with the context's error.
func Run(ctx context.Context, params RunParams) (Outcome, error) {
	if params.Service == nil {
		return Outcome{}, domain.ErrRunServiceRequired
	}

	r := &runner{
		ctx:           ctx,
		service:       params.Service,
		sink:          params.Sink,
		jobs:          params.Jobs,
		declared:      params.Declared,
		profile:       params.Profile,
		workDir:       params.WorkDir,
		worktree:      params.Worktree,
		logDir:        params.LogDir,
		shared:        params.Shared,
		env:           params.Env,
		prober:        params.Prober,
		project:       params.Project,
		proxyPort:     params.ProxyPort,
		portAddressed: params.PortAddressed,
		nextConfig:    params.NextConfig,
		baseOwners:    params.BaseOwners,
	}
	if r.nextConfig == nil {
		r.nextConfig = func(job domain.JobConfig) (string, string) {
			return detect.NextConfig(detect.NextConfigParams{WorkDir: params.WorkDir, Cwd: job.Cwd})
		}
	}
	if r.sink == nil {
		r.sink = noSink{}
	}
	return r.run(), ctx.Err()
}

type runner struct {
	ctx      context.Context
	service  Service
	sink     Sink
	jobs     []domain.JobConfig
	declared []domain.JobConfig
	profile  string
	workDir  string
	worktree string
	logDir   string
	env      map[string]string
	prober   Prober
	// project and proxyPort together decide whether a job's URL is its name or
	// its port; both come from the surface, which is the side that reads config.
	project   string
	proxyPort int
	// portAddressed says this worktree's .env still spells its addresses as
	// ports, so a started job is announced under the port it binds. The route is
	// registered all the same: the name works the moment `wtm env` settles it.
	portAddressed bool
	nextConfig    NextConfigLookup
	baseOwners    map[int]string
	shared        *domain.SharedJobContext
	// servedPort is what the daemon answered its proxy is really on, and
	// noticedProxy records that the run has already explained a refusal — the
	// fact belongs to the run, not to each job that would repeat it.
	servedPort   int
	noticedProxy bool

	// probeTargets are the started services that declared ports, kept in start
	// order so the check runs once, at the end, when everything is up.
	probeTargets []probeTarget
	results      []domain.JobActionResult
	started      []string
	completed    []string
	// captured is what the job being started has written so far, kept only until
	// the next job starts: an abort is the one moment it has to be readable.
	captured []byte
	// crashed are the jobs the daemon accepted and that were gone by the end of
	// the sequence.
	crashed []domain.JobExit
}

func (r *runner) run() Outcome {
	for i := range r.jobs {
		if r.ctx.Err() != nil {
			return r.detached(i)
		}

		job := r.jobs[i]
		r.emit(Event{Phase: PhaseStarting, Job: job.Name, Step: i + 1})

		r.captured = nil
		routes := rules.JobRoutes(rules.JobRoutesParams{
			Config:   domain.RunConfig{Jobs: r.declaredJobs()},
			Job:      job,
			Worktree: r.env[domain.EnvWorktree],
			Project:  r.project,
		})
		host := rules.JobOwnRoute(routes, job.Name)

		result, err := r.service.Start(r.ctx, StartRequest{
			Job:     job,
			WorkDir: r.workDir,
			LogDir:  r.logDir,
			Env:     r.env,
			Routes:  routes,
			Shared:  r.shared,
			OnOutput: func(chunk []byte) {
				r.captured = append(r.captured, chunk...)
				r.emit(Event{Phase: PhaseOutput, Job: job.Name, Kind: job.Kind, Step: i + 1, Chunk: chunk})
			},
		})
		if err != nil {
			// A read the detach itself broke says nothing about the job: the
			// daemon took the request and is running it. Naming it among the ones
			// not started is the one thing the report must not do — it is what
			// made leaving look like it had killed the job.
			if r.ctx.Err() != nil {
				return r.detached(i + 1)
			}
			return r.abort(abortParams{Index: i, Job: job, Reason: err.Error()})
		}

		// A repeat start of a service is what the caller asked for — the job is
		// up — so it counts as started. A task is a step to run, not a state to
		// reach: one the daemon refuses has not run.
		if result.PublicPort > 0 {
			r.servedPort = result.PublicPort
		}
		r.noticeProxyRefused(i + 1)

		alreadyRunning := result.Refused && job.Kind != domain.JobKindTask && rules.IsAlreadyRunning(result.Message)
		if result.Refused && !alreadyRunning {
			return r.abort(abortParams{Index: i, Job: job, Reason: result.Message, ExitCode: result.ExitCode})
		}

		if job.Kind == domain.JobKindTask {
			r.completed = append(r.completed, job.Name)
			held := r.heldURLs(job, routes, result.Ports)
			r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionDone, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host}), Held: held})
			r.emit(Event{Phase: PhaseDone, Job: job.Name, Step: i + 1, Ports: result.Ports, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host}), Held: held})
			continue
		}

		r.started = append(r.started, job.Name)
		held := r.heldURLs(job, routes, result.Ports)
		r.results = append(r.results, domain.JobActionResult{Name: job.Name, Status: r.startedStatus(job), URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host}), Held: held})
		if rules.ShouldProbeJob(rules.ShouldProbeJobParams{Kind: job.Kind, Ports: result.Ports, Probe: job.Probe}) {
			r.probeTargets = append(r.probeTargets, probeTarget{job: job.Name, resolved: result.Ports})
		}
		r.emit(Event{Phase: PhaseStarted, Job: job.Name, Step: i + 1, AlreadyRunning: alreadyRunning, Ports: result.Ports, URL: r.jobURL(jobURLParams{Job: job, Ports: result.Ports, Host: host}), Held: held, DevOrigins: r.devOrigins(job, host)})
	}

	// The probe dials first because its wait is also the time a job needs to die:
	// a service that binds a busy port is accepted, and only exits once it has
	// tried. The daemon is asked for the truth after that wait, and a job found
	// gone has its own port report withdrawn — an exit code says more than a
	// silent port ever could.
	probes := r.probe()
	r.settleStarted()
	probes = r.reportProbes(probes)

	outcome := r.outcome()
	outcome.Probes = probes
	r.emit(Event{Phase: PhaseReady, Outcome: outcome})
	return outcome
}

// awaitExits reads the daemon's view of this worktree, waiting a moment for a
// service to fail if nothing else has waited yet. A process that cannot bind
// its port exits within instants of being spawned, but not before the call that
// spawned it returns — and a run whose probe was switched off has otherwise
// nothing between the two.
func (r *runner) awaitExits() (map[string]domain.JobInfo, bool) {
	deadline := time.Now().Add(r.settleWindow())
	for {
		live, err := r.liveJobs()
		if err != nil {
			return nil, false
		}
		// A daemon that knows none of these jobs has nothing to settle, and
		// waiting on it would only delay the run.
		if r.anyExited(live) || !r.anyKnown(live) || !time.Now().Before(deadline) {
			return live, true
		}
		select {
		case <-r.ctx.Done():
			return live, true
		case <-time.After(domain.JobSettleInterval):
		}
	}
}

// settleWindow is nothing at all when the port check has already waited: the
// jobs have had their time, and asking for more would delay every healthy run.
func (r *runner) settleWindow() time.Duration {
	if r.prober != nil && len(r.probeTargets) > 0 {
		return 0
	}
	return domain.JobSettleWindow
}

func (r *runner) liveJobs() (map[string]domain.JobInfo, error) {
	infos, err := r.service.List(r.workDir)
	if err != nil {
		return nil, err
	}
	live := make(map[string]domain.JobInfo, len(infos))
	for _, info := range infos {
		if info.WorkDir == r.workDir {
			live[info.Name] = info
		}
	}
	return live, nil
}

func (r *runner) anyKnown(live map[string]domain.JobInfo) bool {
	for _, name := range r.started {
		if _, known := live[name]; known {
			return true
		}
	}
	return false
}

func (r *runner) anyExited(live map[string]domain.JobInfo) bool {
	for _, name := range r.started {
		info, known := live[name]
		if known && !rules.IsJobUp(info.Status) {
			return true
		}
	}
	return false
}

// reportProbes says what the check found, once it is known which jobs are still
// there to be said anything about.
func (r *runner) reportProbes(probes []domain.PortProbe) []domain.PortProbe {
	gone := make(map[string]bool, len(r.crashed))
	for _, exit := range r.crashed {
		gone[exit.Job] = true
	}

	kept := make([]domain.PortProbe, 0, len(probes))
	byJob := map[string][]domain.PortProbe{}
	order := make([]string, 0, len(probes))
	for _, probe := range probes {
		if gone[probe.Job] {
			continue
		}
		if _, seen := byJob[probe.Job]; !seen {
			order = append(order, probe.Job)
		}
		byJob[probe.Job] = append(byJob[probe.Job], probe)
		kept = append(kept, probe)
	}
	for _, job := range order {
		r.emit(Event{Phase: PhaseProbed, Job: job, Probes: byJob[job]})
	}
	return kept
}

// settleStarted re-reads the daemon's view of the jobs this run started. The
// daemon answers a spawn, not a life: a service that binds a busy port is
// accepted and dead a moment later, and the run used to conclude "started" over
// a process that had already exited. What is no longer up is moved out of
// Started and its port is no longer probed — its exit code says more than any
// port ever could.
func (r *runner) settleStarted() {
	if len(r.started) == 0 {
		return
	}
	live, found := r.awaitExits()
	if !found {
		return
	}

	started := make([]string, 0, len(r.started))
	for _, name := range r.started {
		info, known := live[name]
		if !known || rules.IsJobUp(info.Status) {
			started = append(started, name)
			continue
		}
		r.crashed = append(r.crashed, domain.JobExit{Job: name, Status: info.Status, ExitCode: info.ExitCode})
		r.markCrashed(name)
		r.emit(Event{Phase: PhaseCrashed, Job: name, Reason: string(info.Status), ExitCode: info.ExitCode})
	}
	r.started = started
}

// markCrashed corrects the result this run already recorded for the job.
func (r *runner) markCrashed(name string) {
	for i := range r.results {
		if r.results[i].Name == name {
			r.results[i].Status = domain.JobActionCrashed
			return
		}
	}
}

// devOrigins is only ever asked when the proxy actually serves the job: under
// its own port, Next has nothing to allow.
// declaredJobs is what the runner relation is read against: everything run.toml
// holds, or the jobs of this run alone when the surface said nothing.
func (r *runner) declaredJobs() []domain.JobConfig {
	if len(r.declared) > 0 {
		return r.declared
	}
	return r.jobs
}

func (r *runner) devOrigins(job domain.JobConfig, host string) []domain.DevOriginFix {
	if r.nextConfig == nil || host == "" || r.servedPort == 0 {
		return nil
	}
	path, source := r.nextConfig(job)
	if !rules.NeedsDevOrigins(rules.NeedsDevOriginsParams{Job: job, ConfigSource: source}) {
		return nil
	}
	return []domain.DevOriginFix{{
		Job:    job.Name,
		Config: path,
		Line:   fmt.Sprintf(domain.DevOriginsFixFmt, job.Name, rules.DevOriginsPattern(r.servedPort), path),
	}}
}

type jobURLParams struct {
	Job   domain.JobConfig
	Ports map[string]int
	Host  string
}

// jobURL answers with the port the daemon says it is really serving, never the
// one this run asked for: a name nothing serves is worse than a port.
func (r *runner) jobURL(params jobURLParams) string {
	return rules.JobURL(rules.JobURLParams{
		Job:        params.Job,
		Ports:      params.Ports,
		Host:       params.Host,
		PublicPort: r.publicPort(),
	})
}

// heldURLs is where each job this one runs answers, for the surfaces that show
// one line per started job: the apps behind a runner are subprocesses, so this
// is the only line their addresses can appear on.
//
// The port comes from what the daemon answered it bound, never from the
// declaration: the runner was given its children's ports, and reporting a
// number it did not bind is the one thing a "started" line must not do.
func (r *runner) heldURLs(job domain.JobConfig, routes []domain.JobRoute, ports map[string]int) []domain.JobURLEntry {
	var held []domain.JobURLEntry
	for _, route := range routes {
		if route.Job == job.Name {
			continue
		}
		url := rules.JobOrigin(rules.JobOriginParams{
			Host:       route.Host,
			PublicPort: r.publicPort(),
			DirectPort: ports[route.Port],
		})
		if url == "" {
			continue
		}
		held = append(held, domain.JobURLEntry{Job: route.Job, URL: url})
	}
	return held
}

// publicPort is what a name announces in this run, zero for a worktree whose
// .env still spells its addresses as ports — where the port is the entrance
// that works.
func (r *runner) publicPort() int {
	if r.portAddressed {
		return 0
	}
	return r.servedPort
}

// noticeProxyRefused explains, once, why the names this run promised are not
// being served. Emitting it per job would bury the one fact it carries.
func (r *runner) noticeProxyRefused(step int) {
	if r.noticedProxy || r.proxyPort == 0 || r.servedPort != 0 {
		return
	}
	r.noticedProxy = true
	r.emit(Event{Phase: PhaseNotice, Step: step, Notice: fmt.Sprintf(domain.ProxyUnavailableFmt, r.proxyPort)})
}

type probeTarget struct {
	job string
	// resolved is what the daemon answered it bound, base plus offset. Read back
	// rather than recomputed, so the number injected and the number checked
	// cannot drift apart.
	resolved map[string]int
}

// probe checks every started service's declared ports in one pass, once they
// are all up: a service started first would otherwise be checked before the one
// it waits on has even been asked to start.
func (r *runner) probe() []domain.PortProbe {
	if r.prober == nil || len(r.probeTargets) == 0 || r.ctx.Err() != nil {
		return nil
	}

	offset := rules.PortOffsetFromEnv(r.env)
	var wanted []int
	for _, target := range r.probeTargets {
		wanted = append(wanted, rules.PortsToDial(rules.DiagnosePortProbesParams{
			Resolved: target.resolved,
			Offset:   offset,
		})...)
	}

	listening := r.prober.Listening(r.ctx, rules.DedupePorts(wanted), func(answered map[int]bool) bool {
		return r.settled(answered, offset)
	})

	var probes []domain.PortProbe
	for _, target := range r.probeTargets {
		found := rules.DiagnosePortProbes(rules.DiagnosePortProbesParams{
			Job:        target.job,
			Resolved:   target.resolved,
			Listening:  listening,
			Offset:     offset,
			BaseOwners: r.baseOwners,
		})
		probes = append(probes, found...)
	}
	return probes
}

// settled ends the poll as soon as every declared port answers — the healthy
// case, which must not wait out the budget.
func (r *runner) settled(answered map[int]bool, offset int) bool {
	for _, target := range r.probeTargets {
		if !rules.AllPortsListening(rules.DiagnosePortProbes(rules.DiagnosePortProbesParams{
			Job:       target.job,
			Resolved:  target.resolved,
			Listening: answered,
			Offset:    offset,
		})) {
			return false
		}
	}
	return true
}

type abortParams struct {
	Index    int
	Job      domain.JobConfig
	Reason   string
	ExitCode *int
}

func (r *runner) abort(params abortParams) Outcome {
	r.results = append(r.results, domain.JobActionResult{
		Name: params.Job.Name, Status: domain.JobActionError, Message: params.Reason,
	})
	r.emit(Event{Phase: PhaseFailed, Job: params.Job.Name, Step: params.Index + 1, Reason: params.Reason})

	outcome := r.outcome()
	outcome.Failed = params.Job.Name
	outcome.FailedStep = params.Index + 1
	outcome.NotStarted = jobNames(r.jobs[params.Index+1:])
	outcome.FailedOutput = r.captured
	outcome.FailedExitCode = params.ExitCode

	r.emit(Event{
		Phase: PhaseAborted, Job: params.Job.Name, Step: params.Index + 1,
		Reason: params.Reason, Outcome: outcome,
	})
	return outcome
}

// detached is the sequence stopping because nobody is watching any more. It is
// not an abort: nothing failed, nothing is torn down, and the jobs already up
// stay up.
// detached is the run giving up its account of itself, index being the first
// job it can honestly say it never reached. A detach caught before the daemon
// was asked leaves the job at index unstarted; one caught while the request was
// in flight does not — the daemon has it, and it is already running.
func (r *runner) detached(index int) Outcome {
	outcome := r.outcome()
	outcome.NotStarted = jobNames(r.jobs[min(index, len(r.jobs)):])
	return outcome
}

// emit drops what it is given once the run is detached: the surface it reported
// to is gone, and a phase it cannot show is noise on a dead channel.
func (r *runner) emit(event Event) {
	if r.ctx.Err() != nil {
		return
	}
	event.Steps = len(r.jobs)
	event.WorkDir = r.workDir
	event.Worktree = r.worktree
	r.sink.Emit(event)
}

func (r *runner) outcome() Outcome {
	return Outcome{
		WorkDir:   r.workDir,
		Worktree:  r.worktree,
		Profile:   r.profile,
		Results:   r.results,
		Started:   r.started,
		Completed: r.completed,
		Crashed:   r.crashed,
		Steps:     len(r.jobs),
	}
}

func jobNames(jobs []domain.JobConfig) []string {
	if len(jobs) == 0 {
		return nil
	}
	names := make([]string, len(jobs))
	for i, job := range jobs {
		names[i] = job.Name
	}
	return names
}

// startedStatus tells joining a shared service apart from starting one. Only the
// main checkout ever spawns the process; every other worktree attaches to it,
// and saying "started" in each of them read as one service per worktree.
func (r *runner) startedStatus(job domain.JobConfig) string {
	if rules.IsShared(job) && r.shared != nil && r.workDir != r.shared.WorkDir {
		return domain.JobActionAttached
	}
	return domain.JobActionStarted
}
