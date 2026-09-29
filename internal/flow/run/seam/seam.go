// Package seam binds a run flow to the daemon holding a worktree's jobs: the
// board a surface lists, the environment those jobs are given, and the start
// sequence a surface drives. It assumes the daemon is up — opening one is the
// caller's business, because only it knows the proxy port.
package seam

import (
	"context"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/portprobe"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	ProjectDir string
	StateDir   string
	// WorkDir is the worktree this seam is about, as git spells it.
	WorkDir string
	// Jobs are what the board lists. `run up` passes the profile it resolved, so
	// the view shows the run rather than every job run.toml declares beside it,
	// with the previous run's log behind each (LUC-208); `run logs` passes them
	// all, which is what it is for.
	Jobs []domain.JobConfig
	// Declared is every job run.toml holds, not just the ones this run starts.
	// A port is owned by whichever job binds it, and the job holding the base
	// port may well be one this profile never names. Empty falls back to Jobs.
	Declared []domain.JobConfig
	// ProxyPort is the port the run proxy is configured to bind. It is only ever
	// reported — it is what the "proxy unavailable" notice names — and must not
	// be used to build an address: a redirection installed on the 80 makes the
	// port a name answers on differ from the port the proxy binds.
	ProxyPort int
	// PublicPort is the port a published name actually answers on, which is what
	// every address this seam hands out is built from. Zero leaves the jobs on
	// their own ports. Kept apart from ProxyPort because the two diverge exactly
	// when the redirection is installed, and a board built from the bind port
	// then announces urls nobody can reach.
	PublicPort int
	// ProbeBudget is how long the port check may dial for; zero or NoProbe skips
	// it entirely.
	ProbeBudget time.Duration
	NoProbe     bool
}

type Seam struct {
	service    runlogs.Service
	board      runlogs.Board
	workDir    string
	worktree   string
	logDir     string
	env        map[string]string
	envErr     error
	prober     runlogs.Prober
	project    string
	proxyPort  int
	publicPort int
	projectDir string
	stateDir   string
	jobs       []domain.JobConfig
	declared   []domain.JobConfig
	shared     *domain.SharedJobContext
}

func Open(params Params) Seam {
	branch := target.BranchOf(params.WorkDir)
	logDir := logDirOf(params.StateDir, branch)
	service := runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()})
	env, envErr := JobEnv(JobEnvParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		WorkDir:    params.WorkDir,
	})
	// Resolved once, and only when something declares a shared job: it costs a
	// git worktree list plus a full environment resolution for the main
	// checkout, and every run command opens a seam.
	shared := sharedContext(params)
	return Seam{
		service: service,
		board: runlogs.NewBoard(runlogs.BoardParams{
			Service:      service,
			Jobs:         params.Jobs,
			WorkDir:      params.WorkDir,
			Worktree:     branch,
			LogDir:       logDir,
			SharedLogDir: sharedLogDirOf(shared),
			Addresses:    boardAddresses(boardAddressParams{Params: params, Env: env}),
			// Read once, here: the board is the side that knows the log directory,
			// and every surface over it then reads the same trace rather than
			// listing its own idea of what this worktree has run.
			Logged:         process.LoggedJobs(logDir),
			SharedWorktree: sharedWorktreeOf(sharedWorktreeParams{Shared: shared, WorkDir: params.WorkDir}),
			Env:            env,
		}),
		workDir:    params.WorkDir,
		worktree:   branch,
		logDir:     logDir,
		env:        env,
		envErr:     envErr,
		jobs:       params.Jobs,
		declared:   declaredOf(params),
		prober:     newProber(params.ProbeBudget, params.NoProbe),
		project:    filepath.Base(params.ProjectDir),
		proxyPort:  params.ProxyPort,
		publicPort: params.PublicPort,
		projectDir: params.ProjectDir,
		stateDir:   params.StateDir,
		shared:     shared,
	}
}

// sharedContext is where this repository's shared jobs run. Resolved here, once
// per seam, because it is the one place that may ask git which worktree is the
// main one — and it is deliberately nil rather than a guess when there is none:
// the daemon then refuses a shared job instead of running one per worktree.
// sharedLogDirOf is where the repository's shared services persist their output.
// Empty when there is no main checkout to run one in.
func sharedLogDirOf(shared *domain.SharedJobContext) string {
	if shared == nil {
		return ""
	}
	return shared.LogDir
}

// A project declaring no shared job pays nothing — the git calls below would
// otherwise be added to every single run command, `run ps` included.
func sharedContext(params Params) *domain.SharedJobContext {
	if !rules.AnySharedJob(declaredOf(params)) {
		return nil
	}
	main, err := worktree.MainCheckout(worktree.MainCheckoutParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return nil
	}
	env, err := JobEnv(JobEnvParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir, WorkDir: main})
	if err != nil {
		return nil
	}
	return &domain.SharedJobContext{
		WorkDir: main,
		Env:     env,
		LogDir:  logDirOf(params.StateDir, target.BranchOf(main)),
	}
}

type sharedWorktreeParams struct {
	Shared  *domain.SharedJobContext
	WorkDir string
}

// sharedWorktreeOf names where this worktree's shared services run, empty when
// they run here.
func sharedWorktreeOf(params sharedWorktreeParams) string {
	if params.Shared == nil || params.Shared.WorkDir == params.WorkDir {
		return ""
	}
	return target.BranchOf(params.Shared.WorkDir)
}

func (s Seam) Board() runlogs.Board { return s.board }
func (s Seam) Worktree() string     { return s.worktree }

type StartParams struct {
	Profile string
	Jobs    []domain.JobConfig
}

// Starter is the start sequence as a surface drives it: it draws first, then
// calls what this returns.
func (s Seam) Starter(params StartParams) runlogs.StartFunc {
	return func(ctx context.Context, sink runlogs.Sink) (runlogs.Outcomes, error) {
		outcome, err := s.run(ctx, sink, params)
		return runlogs.Outcomes{outcome}, err
	}
}

func (s Seam) run(ctx context.Context, sink runlogs.Sink, params StartParams) (runlogs.Outcome, error) {
	return s.start(ctx, s.recording(recordingParams{Sink: sink, Jobs: params.Jobs}), params)
}

type recordingParams struct {
	Sink runlogs.Sink
	Jobs []domain.JobConfig
}

// recording remembers each namespace the moment its service reports started,
// not once the whole sequence is over: it is the only durable trace that this
// worktree holds one, `clean` reads it to give back exactly what exists, and a
// run interrupted after the create would otherwise leave a database nothing
// will ever drop. A verbatim worktree carves nothing — the daemon refused to.
func (s Seam) recording(params recordingParams) runlogs.Sink {
	sink := params.Sink
	if sink == nil {
		sink = discard{}
	}
	if s.shared == nil || rules.IsVerbatim(domain.Isolation(s.env[domain.EnvIsolation])) {
		return sink
	}
	carving := rules.NamespaceJobs(params.Jobs)
	if len(carving) == 0 {
		return sink
	}
	return namespaceRecorder{inner: sink, jobs: carving, stateDir: s.stateDir, worktree: s.worktree}
}

type discard struct{}

func (discard) Emit(runlogs.Event) {}

type namespaceRecorder struct {
	inner    runlogs.Sink
	jobs     []string
	stateDir string
	worktree string
}

func (r namespaceRecorder) Emit(event runlogs.Event) {
	r.inner.Emit(event)
	if event.Phase != runlogs.PhaseStarted || !slices.Contains(r.jobs, event.Job) {
		return
	}
	err := worktree.RecordNamespaces(worktree.RecordNamespacesParams{StateDir: r.stateDir, Branch: r.worktree, Jobs: []string{event.Job}})
	if err == nil {
		return
	}
	r.inner.Emit(runlogs.Event{
		Phase:    runlogs.PhaseWarning,
		Job:      event.Job,
		WorkDir:  event.WorkDir,
		Worktree: event.Worktree,
		Notice:   fmt.Sprintf(domain.NamespaceRecordFailedFmt, event.Job, err),
	})
}

func (s Seam) start(ctx context.Context, sink runlogs.Sink, params StartParams) (runlogs.Outcome, error) {
	if s.envErr != nil {
		return runlogs.Outcome{}, s.envErr
	}
	return runlogs.Run(ctx, runlogs.RunParams{
		BaseOwners:     s.baseOwners(),
		Service:        s.service,
		Sink:           sink,
		Jobs:           params.Jobs,
		Declared:       s.declared,
		Profile:        params.Profile,
		WorkDir:        s.workDir,
		Worktree:       s.worktree,
		LogDir:         s.logDir,
		Env:            s.env,
		Prober:         s.prober,
		Project:        s.project,
		ProxyPort:      s.proxyPort,
		PublicPort:     s.publicPort,
		Shared:         s.shared,
		SharedWorktree: sharedWorktreeOf(sharedWorktreeParams{Shared: s.shared, WorkDir: s.workDir}),
	})
}

// declaredOf is every job run.toml holds, falling back to the ones this run
// lists when the surface named no wider set. Three readings need it — which
// worktree owns a base port, which children a runner publishes, and where those
// children answer — and all three are wrong when they only see the jobs of this
// run: `run up` lists a profile, and a runner's children are rarely in it.
func declaredOf(params Params) []domain.JobConfig {
	if len(params.Declared) > 0 {
		return params.Declared
	}
	return params.Jobs
}

// baseOwners names the worktree bound to each declared base port, so the probe
// does not blame a command for a port the main checkout holds. An unreachable
// daemon yields nothing, which restores the older message rather than refusing
// the run: a diagnosis never blocks a start.
func (s Seam) baseOwners() map[int]string {
	running, err := s.service.List("")
	if err != nil {
		return nil
	}
	return rules.BasePortOwners(rules.BasePortOwnersParams{
		SelfWorkDir: s.workDir,
		Jobs:        s.declared,
		Running:     running,
		Holders:     holdersOf(running),
	})
}

// holdersOf names each worktree the daemon has something up in. The branch is
// looked up here rather than carried by the daemon, which must never run git.
func holdersOf(running []domain.JobInfo) []rules.PortHolder {
	seen := make(map[string]bool, len(running))
	holders := make([]rules.PortHolder, 0, len(running))
	for _, info := range running {
		if info.WorkDir == "" || seen[info.WorkDir] {
			continue
		}
		seen[info.WorkDir] = true
		holders = append(holders, rules.PortHolder{
			WorkDir:  info.WorkDir,
			Worktree: target.BranchOf(info.WorkDir),
		})
	}
	return holders
}

type boardAddressParams struct {
	Params Params
	Env    map[string]string
}

// boardAddresses is where each declared job answers in this worktree. The seam
// computes it because it is the side holding the worktree's offset and the
// proxy's port; every surface then reads the same figures off the board rather
// than deriving its own.
func boardAddresses(params boardAddressParams) map[string]domain.JobAddress {
	publicPort := params.Params.PublicPort
	return rules.WorktreeJobAddresses(rules.WorktreeJobAddressesParams{
		Config:     domain.RunConfig{Jobs: declaredOf(params.Params)},
		PortOffset: rules.PortOffsetFromEnv(params.Env),
		Worktree:   params.Env[domain.EnvWorktree],
		Project:    filepath.Base(params.Params.ProjectDir),
		PublicPort: publicPort,
	})
}

type LogDirParams struct {
	StateDir string
	WorkDir  string
}

// LogDir resolves where the daemon persists this worktree's job logs. The
// branch is looked up here rather than passed along by the daemon, which must
// never run git; a worktree with no branch, or one git cannot name, persists
// nothing rather than sharing another's directory.
func LogDir(params LogDirParams) string {
	return logDirOf(params.StateDir, target.BranchOf(params.WorkDir))
}

func logDirOf(stateDir, branch string) string {
	if branch == "" {
		return ""
	}
	return rules.WorktreeLogDir(rules.WorktreeLogDirParams{StateDir: stateDir, Branch: branch})
}

type JobEnvParams struct {
	ProjectDir string
	StateDir   string
	WorkDir    string
}

// JobEnv resolves the worktree-scoped environment handed to every job of this
// run. It fails rather than degrade: a worktree with no offset and no name is
// one whose jobs would bind the main checkout's ports.
func JobEnv(params JobEnvParams) (map[string]string, error) {
	env, err := worktree.JobEnv(worktree.JobEnvParams{
		ProjectDir: params.ProjectDir,
		StateDir:   params.StateDir,
		Dir:        params.WorkDir,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w", domain.ErrWorktreeEnvUnresolved, params.WorkDir, err)
	}
	return env, nil
}

type RequireEnvParams struct {
	ProjectDir string
	StateDir   string
	WorkDirs   []string
}

// RequireEnv refuses a start before anything is started when one of its
// worktrees has no environment to give its jobs, or has yet to choose its
// isolation. The choice is checked first: resolving the environment allocates
// the ordinal the choice is about.
func RequireEnv(params RequireEnvParams) error {
	for _, dir := range params.WorkDirs {
		if err := requireIsolationChosen(requireChosenParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir, WorkDir: dir}); err != nil {
			return err
		}
		if _, err := JobEnv(JobEnvParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir, WorkDir: dir}); err != nil {
			return err
		}
	}
	return nil
}

type requireChosenParams struct {
	ProjectDir string
	StateDir   string
	WorkDir    string
}

func requireIsolationChosen(params requireChosenParams) error {
	branch := target.BranchOf(params.WorkDir)
	if branch == "" {
		return nil
	}
	plan, err := worktree.IsolationAdoptionFor(worktree.IsolationAdoptionParams{
		Ref:          worktree.WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: branch},
		WorktreePath: params.WorkDir,
	})
	if err != nil || !plan.Pending {
		return nil
	}
	return fmt.Errorf("%w: %s", domain.ErrIsolationAdoptionPending, fmt.Sprintf(domain.RunIsolationAdoptionPendingFmt, branch, branch))
}

// dialProber is this side of the runlogs.Prober seam: the run says which ports
// to check, here owns the budget and the socket.
type dialProber struct{ budget time.Duration }

func (p dialProber) Listening(ctx context.Context, ports []int, settled func(map[int]bool) bool) map[int]bool {
	return portprobe.Poll(ctx, portprobe.PollParams{Ports: ports, Budget: p.budget, Settled: settled})
}

// newProber returns nil when the check is switched off, which is what the run
// reads to skip it entirely.
func newProber(budget time.Duration, disabled bool) runlogs.Prober {
	if disabled || budget <= 0 {
		return nil
	}
	return dialProber{budget: budget}
}

// SequenceParams is the hand-over from a run to the surface watching it: what
// the surface lists, what it calls when it is ready to report, and what to call
// the run in a header.
type SequenceParams struct {
	Board runlogs.Board
	// Profile and Job name the run, exactly one of them set: a profile for
	// `run up`, a job for `run start`.
	Profile string
	Job     string
	// Worktrees names the branches this run covers, in the order they were
	// selected. A surface reads its length to know whether to say which worktree
	// a line belongs to at all.
	Worktrees []string
	Start     runlogs.StartFunc
	// Warnings are what the run has to say about the addresses it publishes.
	// The flow reads them; each surface renders them once, where it can be seen
	// — a band inside the view, a callout beside a stream.
	Warnings []string
	// Inline says the run blocks until it ends — a task, whose output belongs to
	// the scrollback rather than to a screen given back when it exits. It is the
	// flow that knows, because it is the flow that resolved the job.
	Inline bool
}

// Watcher is the half of a run's Presenter that shows the start. It is the one
// thing a run cannot report through Stage: the surface has to be drawing before
// the first job is asked for, so it is the surface that calls Start.
type Watcher interface {
	Sequence(SequenceParams) (runlogs.Outcomes, error)
}

type ProxyPortsParams struct {
	Global domain.GlobalConfig
	Run    domain.RunConfig
}

// ProxyPorts is what a seam is opened with: the port the proxy binds, and the
// one a published name answers on. Both are zero under ports addressing, which
// leaves every address this run hands out on the job's own port.
type ProxyPorts struct {
	Bind   int
	Public int
}

func ProxyPortsFor(params ProxyPortsParams) ProxyPorts {
	bind := rules.RunProxyPort(rules.RunProxyPortParams{Run: params.Run, Global: params.Global})
	return ProxyPorts{Bind: bind, Public: process.PublicProxyPort(bind)}
}
