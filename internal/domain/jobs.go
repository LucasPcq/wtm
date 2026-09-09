package domain

import "time"

// JobKind distinguishes a long-running service from a one-shot task.
type JobKind string

const (
	// JobKindService runs long-running. With a Stop command, it is treated as
	// detached (docker compose up -d pattern); without one, tracked by PID
	// and killed via SIGTERM.
	JobKindService JobKind = "service"

	// JobKindTask is a one-shot script that must exit 0 before the profile
	// continues. Output is streamed live, the job is removed on exit.
	JobKindTask JobKind = "task"
)

// JobScope says whether a job has one instance per worktree or one for the
// whole repository. Empty is per-worktree, which keeps every run.toml written
// before this existed reading unchanged.
type JobScope string

const (
	JobScopePerWorktree JobScope = ""
	JobScopeShared      JobScope = "shared"
)

// JobTenantConfig is the worktree's slice of a shared service. It names one
// tenant and never a list: four keycloak realms are one tenant, whose internal
// shape belongs to the attach script rather than to wtm.
type JobTenantConfig struct {
	Name   string            `toml:"name"             json:"name"`
	Attach string            `toml:"attach,omitempty" json:"attach,omitempty"`
	Detach string            `toml:"detach,omitempty" json:"detach,omitempty"`
	Env    map[string]string `toml:"env,omitempty"    json:"env,omitempty"`
}

// JobURLConfig is a job's [[job]].url table: which of its declared ports speaks
// HTTP, and the host label it is published under. Port names a key of Ports, not
// a number — the number depends on the worktree, only the declaration is stable.
// Host is optional and defaults to the job's name.
type JobURLConfig struct {
	Port string `toml:"port"           json:"port"`
	Host string `toml:"host,omitempty" json:"host,omitempty"`
}

// Addressing is what an [[env_port]] link writes into a .env value: the job's
// port number, or the full origin it is published under. See
// docs/dev/run-addressing.md for the vocabulary this rests on.
type Addressing string

const (
	// AddressingPorts substitutes the port number, wherever it sits in the value.
	AddressingPorts Addressing = "ports"
	// AddressingNames substitutes the whole origin, for the links whose job
	// publishes a name and whose value has the shape of a URL.
	AddressingNames Addressing = "names"
)

// Concurrency is what `run up` does about the jobs another worktree already has
// running. Isolation gave each worktree its own ports and resource names, so two
// stacks no longer collide; what is left is that a machine may not hold three of
// them at once. That is a preference, not a conflict — so it is remembered here
// rather than asked at every start.
type Concurrency string

const (
	// ConcurrencyParallel leaves the other worktrees' jobs running.
	ConcurrencyParallel Concurrency = "parallel"
	// ConcurrencyExclusive stops them before starting here.
	ConcurrencyExclusive Concurrency = "exclusive"
)

// JobConfig defines a managed job from .wtm/run.toml.
type JobConfig struct {
	Name string  `toml:"name"           json:"name"`
	Kind JobKind `toml:"kind"           json:"kind"`
	Cmd  string  `toml:"cmd"            json:"cmd"`
	Stop string  `toml:"stop,omitempty" json:"stop,omitempty"`
	Cwd  string  `toml:"cwd,omitempty"  json:"cwd,omitempty"`
	// Ports maps an environment variable to the port it takes on the main
	// checkout. Every other worktree gets that base plus its own offset, so the
	// same job binds a free port in each one.
	Ports map[string]int `toml:"ports,omitempty" json:"ports,omitempty"`
	// URL publishes one of the ports above under a name. Absent means the job
	// keeps no name and stays reachable by its port, as before.
	URL *JobURLConfig `toml:"url,omitempty" json:"url,omitempty"`
	// Probe gates the port check for this job. Nil means the default, which is
	// to check: a job only opts out after its reader was asked and said so.
	Probe *bool `toml:"probe,omitempty" json:"probe,omitempty"`
	// BindsNoPort says this service listens on nothing by design — a build in
	// watch mode, a worker, or a runner whose children hold the ports. Silence
	// says the same thing as "not settled yet", and wtm reads silence as an
	// oversight; this is how a reader answers it once.
	BindsNoPort bool `toml:"binds_no_port,omitempty" json:"binds_no_port,omitempty"`
	// Runs names the declared jobs this one starts itself — `turbo run dev`
	// against a filter, a compose stack of several apps, any single process
	// that fans out. wtm learns nothing about the runner from it: the relation
	// is declared, never inferred from the command.
	Runs []string `toml:"runs,omitempty" json:"runs,omitempty"`
	// Scope makes this job one instance for the repository instead of one per
	// worktree. Tenant is the slice each worktree then gets of it; nil means
	// shared for good, one instance and one set of data.
	Scope  JobScope         `toml:"scope,omitempty"  json:"scope,omitempty"`
	Tenant *JobTenantConfig `toml:"tenant,omitempty" json:"tenant,omitempty"`
}

// JobURLChoice is one job's answer to "should this be reachable by name": the
// port to publish and whether to publish it. Publish false is an answer too — it
// withdraws a url the config already carried.
type JobURLChoice struct {
	Job     string
	Port    string
	Publish bool
}

// JobURLEntry is one published job as a surface reports it: the job's name and
// where it answers in this worktree.
type JobURLEntry struct {
	Job string `json:"job"`
	URL string `json:"url"`
}

// JobAddress is where a declared job answers in one worktree: the ports it
// binds there, and the name it is published under when it publishes one. It is
// a property of the worktree's offset, known whether or not anything is
// running.
type JobAddress struct {
	Ports []int
	URL   string
	// Held are the names this job's process answers for besides its own: one per
	// published job it runs. A runner is a single process — `turbo run dev` —
	// and the addresses a reader came for belong to the apps behind it, which
	// have no row of their own while it holds them.
	Held []JobURLEntry
}

// ProfileConfig defines a named, ordered group of jobs.
type ProfileConfig struct {
	Name    string   `toml:"name"    json:"name"`
	Jobs    []string `toml:"jobs"    json:"jobs"`
	Default bool     `toml:"default" json:"default"`
}

// RunConfig is the top-level structure of .wtm/run.toml. Each [[job]] and
// [[profile]] block declares one entry; the Go field names are kept plural
// because they hold the slices of all entries.
type RunConfig struct {
	// PortOffsetBlock spaces two worktrees' declared ports apart. Zero means the
	// default: a project only sets it to make room for base ports that would
	// otherwise land a multiple of the block apart.
	PortOffsetBlock int `toml:"port_offset_block,omitempty" json:"port_offset_block,omitempty"`
	// PortProbeTimeout is how many seconds `run up` waits for a declared port to
	// answer before reporting it silent. Zero falls back to the default; a
	// negative value disables the check.
	PortProbeTimeout int             `toml:"port_probe_timeout,omitempty" json:"port_probe_timeout,omitempty"`
	Jobs             []JobConfig     `toml:"job"                        json:"job"`
	Profiles         []ProfileConfig `toml:"profile,omitempty"          json:"profile"`
	// EnvPorts links a .env key to one of the ports declared above, so a value
	// holding a hard-coded host port follows the worktree's offset.
	EnvPorts []EnvPortLink `toml:"env_port,omitempty" json:"env_port,omitempty"`
	// Addressing is what those links write. Empty means AddressingNames: a
	// project that publishes names wants its .env values to reach them, and one
	// that publishes none is unaffected either way.
	Addressing Addressing `toml:"addressing,omitempty" json:"addressing,omitempty"`
	// Concurrency is the standing answer to "other worktrees are running jobs".
	// Empty means the question is still open: `run up` asks it once, and writes
	// the answer here when the user asks it to be remembered.
	Concurrency Concurrency `toml:"concurrency,omitempty" json:"concurrency,omitempty"`
}

// ExecSpec is a command ready for exec: the binary and the arguments it takes,
// already resolved from whatever form the config wrote it in.
type ExecSpec struct {
	Name string
	Args []string
}

// JobStatus represents the current state of a managed job.
type JobStatus string

const (
	JobStatusRunning JobStatus = "running"
	JobStatusStopped JobStatus = "stopped"
	JobStatusCrashed JobStatus = "crashed"
	// JobStatusDetached is a service whose launcher has exited, leaving the real
	// work to something wtm does not own — a compose stack, typically. It is not
	// a weaker "running": nothing was ever verified, before or after a daemon
	// restart, and there is no stream to attach to.
	JobStatusDetached JobStatus = "detached"
	// JobStatusAttached is a worktree's claim on a shared service running under
	// the main checkout's key. It owns no process: it is the pointer that keeps
	// the real job alive, which is what makes the job table the reference count.
	JobStatusAttached JobStatus = "attached"
)

// JobRoute is one name the proxy serves a started job under: the job the name
// belongs to, the host it answers on, and the port variable whose resolved
// value sits behind it.
//
// A job publishing its own url has exactly one. A runner has one per published
// job it starts: `turbo run dev` is a single process holding six apps, and
// their names are served by the only job the daemon knows about — itself.
type JobRoute struct {
	Job  string `json:"job"`
	Host string `json:"host"`
	// Port is the variable's name, not its value: the resolved port is the
	// daemon's to compute, from the same environment it gave the process.
	Port string `json:"port"`
}

// JobRecord is one entry of the daemon's durable index: which worktree started
// which job, and everything needed to stop it later from a daemon that never
// spawned it. Env, Routes and LogDir are resolved by a client — the daemon
// cannot run git — so losing them would mean losing the ability to tear the job
// down (a `docker compose down` without COMPOSE_PROJECT_NAME dismantles the
// wrong project, or nothing at all).
//
// No PID: the only entries that survive a daemon are detached ones, whose
// launcher is dead by construction and whose stop is a command, never a signal.
type JobRecord struct {
	Name      string            `json:"name"`
	WorkDir   string            `json:"work_dir"`
	Config    JobConfig         `json:"config"`
	Env       map[string]string `json:"env,omitempty"`
	Routes    []JobRoute        `json:"routes,omitempty"`
	LogDir    string            `json:"log_dir,omitempty"`
	StartedAt time.Time         `json:"started_at,omitzero"`
	// Attached says this entry is a worktree's claim on a shared service rather
	// than a process of its own. It is a fact about what the entry is, not a
	// process state: without it a claim would come back from the index as a
	// foreground service the daemon had lost, and be reported crashed.
	Attached bool `json:"attached,omitempty"`
	// SharedDir is the main checkout a shared job runs in, carried by the real
	// job and by every claim on it. Without it the two would have to be paired
	// by name, and the daemon is machine-wide: two repositories declaring a job
	// called "db" would then release each other's.
	SharedDir string `json:"shared_dir,omitempty"`
}

// TenantRef is one worktree's slice of one shared service, named by what it
// takes to recompute it: run.toml still holds the template, so an entry keeps
// only what the worktree itself contributed.
type TenantRef struct {
	Job      string `toml:"job"      json:"job"`
	Worktree string `toml:"worktree" json:"worktree"`
	Ordinal  int    `toml:"ordinal"  json:"ordinal"`
}

// SharedJobContext is what a shared job needs and only the client can resolve:
// the main checkout it runs in, and that checkout's own environment and log
// directory rather than those of the worktree asking for it.
type SharedJobContext struct {
	WorkDir string            `json:"work_dir"`
	Env     map[string]string `json:"env,omitempty"`
	LogDir  string            `json:"log_dir,omitempty"`
}

// DaemonState is the index as it sits on disk.
type DaemonState struct {
	Version int         `json:"version"`
	Jobs    []JobRecord `json:"jobs"`
}

// JobInfo is the JSON representation of a managed job, shared across the daemon
// protocol and the output/tui layers that render it.
type JobInfo struct {
	Name    string    `json:"name"`
	Kind    JobKind   `json:"kind"`
	Status  JobStatus `json:"status"`
	PID     int       `json:"pid"`
	WorkDir string    `json:"work_dir"`
	// StartedAt is when the daemon spawned the process. Zero for a job it never
	// spawned — one named by a picker, or read back after a daemon restart.
	StartedAt time.Time `json:"started_at,omitzero"`
	// URL is where the job is reachable, absent for one that publishes no name.
	URL string `json:"url,omitempty"`
	// ExitCode stays nil until the job's own process is reaped, and -1 says a
	// signal killed it. A detached launcher exiting does not end its job, so it
	// keeps a nil code for as long as the service it started is registered.
	ExitCode *int `json:"exit_code,omitempty"`
}

// JobExit is a job that was started and did not survive the sequence: what the
// daemon says of it now, and the code it left if its process was reaped.
type JobExit struct {
	Job      string    `json:"job"`
	Status   JobStatus `json:"status"`
	ExitCode *int      `json:"exit_code,omitempty"`
}

// JobActionResult is one job's outcome as a `run *` command reports it, shared
// by every surface that speaks the JobAction* vocabulary — the JSON output and
// the flow seam alike.
type JobActionResult struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	// Message carries the detail when Status is JobActionError.
	Message string `json:"message,omitempty"`
	// Output is what the job had written when it failed, raw. A caller reading
	// this document never saw the live stream, and the daemon's message alone
	// ("task migrate failed: exit status 1") does not say why it stopped.
	Output string `json:"output,omitempty"`
	// ExitCode is what the failing job exited with, absent for one that never
	// got as far as running.
	ExitCode *int `json:"exit_code,omitempty"`
	// Ports is what the probe found on each port this job declared. A reader of
	// this document never saw the live stream, and "started" alone does not say
	// whether anything is listening.
	Ports []PortProbe `json:"ports,omitempty"`
	// URL is where the job is reachable, absent for one that publishes no name.
	URL string `json:"url,omitempty"`
	// Held are the addresses this job answers for besides its own: one per
	// published job it runs. A runner is one process, and the apps behind it
	// have no entry of their own in a run that only started it.
	Held []JobURLEntry `json:"held,omitempty"`
}

// WorktreeRunResult is one worktree's half of a run over several of them. A run
// over a single worktree does not use it: the shape follows the arity, so one
// worktree still answers with the bare array of job results every run command
// emits (LUC-198).
type WorktreeRunResult struct {
	// Worktree is the branch, Path where it is — the daemon's key, and what a
	// caller needs to act on that worktree afterwards.
	Worktree string `json:"worktree"`
	Path     string `json:"path"`
	Profile  string `json:"profile,omitempty"`
	// Aborted says this worktree stopped short. The others carry on regardless,
	// so it is read per worktree and never for the run as a whole.
	Aborted bool              `json:"aborted"`
	Jobs    []JobActionResult `json:"jobs"`
}

// WorktreeJobResults is one worktree's answer to a command that acted on
// several. Like WorktreeRunResult it only exists above one worktree: a command
// acting on a single one answers with the bare array of job results it always
// has (LUC-198).
type WorktreeJobResults struct {
	Worktree string            `json:"worktree"`
	Path     string            `json:"path"`
	Jobs     []JobActionResult `json:"jobs"`
}

// LogRecord is one sanitized line of a job's output, as persisted in that job's
// log file.
type LogRecord struct {
	At   time.Time
	Text string
}

// JobLogEntry is one persisted line as `run logs --output json` reports it. At
// is absent on a line written before this format, or by a sink that could not
// stamp it: the text is still worth handing over.
type JobLogEntry struct {
	Job string `json:"job"`
	// Worktree names where the line came from, and is absent above a single
	// worktree — where the caller already knows. Without it the lines of two jobs
	// called `web` are one indistinguishable stream (LUC-216).
	Worktree string `json:"worktree,omitempty"`
	At       string `json:"at,omitempty"`
	Text     string `json:"text"`
}

// RunSurface names who shows a run's jobs: the full-screen view, a stream of
// lines on the terminal the command was launched from, or a machine-readable
// document.
type RunSurface int

const (
	RunSurfaceView RunSurface = iota
	RunSurfaceStream
	RunSurfaceMachine
)

// JobKindChoice is one job whose kind the wizard asks about. Label is how the
// job reads on screen ("apps/web / build"), Name the script it came from.
type JobKindChoice struct {
	Label string
	Cmd   string
	// Name and Workspace together identify the script: two packages of a
	// monorepo both declaring "build" are two separate answers.
	Name      string
	Workspace string
	Kind      JobKind
}

// DevOriginFix is a published Next job that would refuse requests arriving under
// its own name, and the line its config is missing.
type DevOriginFix struct {
	Job    string
	Config string
	Line   string
}

// JobCmdFix is a job whose command never mentions a port variable wtm injects
// for it. The command will bind whatever it binds today, ignoring the offset.
type JobCmdFix struct {
	Job  string
	Cmd  string
	Vars []string
}

// DaemonStatus is what `wtm run daemon status` reports: whether a daemon holds
// the socket, which build it is, and what it is holding. Counted by nature
// rather than totalled, because the two answer different questions — foreground
// services die with the daemon, detached ones outlive it.
type DaemonStatus struct {
	Running bool `json:"running"`
	// Version is this binary's, DaemonVersion the one answering. They differ
	// exactly when the daemon is serving behavior this build has moved past.
	Version       string `json:"version"`
	DaemonVersion string `json:"daemon_version,omitempty"`
	PID           int    `json:"pid,omitempty"`
	SocketPath    string `json:"socket_path"`
	StatePath     string `json:"state_path"`
	ProxyPort     int    `json:"proxy_port,omitempty"`
	Foreground    int    `json:"foreground_jobs"`
	Detached      int    `json:"detached_jobs"`
}

// JobConflict is a job that would be started twice at once: on its own, and by
// the runner that declares it in `runs`.
type JobConflict struct {
	Job    string
	Runner string
}

// JobRunnerChoice is one job and the runners that start it, empty for none.
// Options carries the answers the step can cycle through, the empty one first:
// no relation is the default, because guessing which command fans out into
// which jobs is the one thing wtm refuses to infer.
//
// Runners is a list because run.toml has always allowed one: two roots may each
// start the same app. The step sets one at a time — cycling a row replaces what
// it held — but it never drops what it did not show, so a relation written by
// hand survives a re-init that leaves its row alone.
type JobRunnerChoice struct {
	Job     string
	Label   string
	Runners []string
	Options []string
}
