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

// JobNamespaceConfig is the worktree's slice of a shared service. It names one
// namespace and never a list: four keycloak realms are one namespace, whose internal
// shape belongs to the create script rather than to wtm.
type JobNamespaceConfig struct {
	Name   string            `toml:"name"             json:"name"`
	Create string            `toml:"create,omitempty" json:"create,omitempty"`
	Remove string            `toml:"remove,omitempty" json:"remove,omitempty"`
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

// Isolation is how a worktree stands against the checkout its .env was copied
// from. It is one decision read by both halves of the run module — what is
// written into the .env and what the daemon hands a job — because a worktree
// whose file says one thing and whose processes are told another starts wired
// to a neighbour without a word.
type Isolation string

const (
	// IsolationIsolated gives the worktree its own ports, compose project and
	// service slices, in its .env and at run time alike.
	IsolationIsolated Isolation = "isolated"
	// IsolationVerbatim keeps the .env exactly as it was copied: wtm writes
	// nothing into it, and runs the worktree on the ports and data it names —
	// its source's, so the two cannot run at the same time.
	IsolationVerbatim Isolation = "verbatim"
)

// IsolationAdoption is what `wtm env` did about a worktree created before the
// isolation choice existed: such a worktree runs on its source's ports and
// compose project until it adopts one.
type IsolationAdoption string

const (
	IsolationAdopted    IsolationAdoption = "adopted"
	IsolationNotAdopted IsolationAdoption = "not_adopted"
)

// IsolationAdoptionPlan says whether a worktree still has to adopt its
// isolation, and what adopting it changes.
type IsolationAdoptionPlan struct {
	Pending bool
	// ComposeProject is the project an isolated worktree runs under, empty when
	// run.toml starts no compose stack.
	ComposeProject string
	// CurrentComposeProject is the one it runs under today, whose volumes it
	// would stop using.
	CurrentComposeProject string
}

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
	// Touches names the declared services whose data this job changes — a
	// migration, a reset, a seed. wtm cannot read that from a command, and it
	// is what lets `run up` stop before a job rewrites data the worktree does
	// not own: its source's, for a verbatim worktree.
	Touches []string `toml:"touches,omitempty" json:"touches,omitempty"`
	// A nil Namespace on a shared job means shared for good: one instance, one set
	// of data.
	Scope     JobScope            `toml:"scope,omitempty"     json:"scope,omitempty"`
	Namespace *JobNamespaceConfig `toml:"namespace,omitempty" json:"namespace,omitempty"`
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

// ReachEntry is where one started job is reached: the URLs it answers on — its
// own, or one per app a runner holds — else the ports it binds, and the
// namespace a shared job carved out for this worktree. Every run surface
// renders the same list, at its own density.
type ReachEntry struct {
	Job       string
	URLs      []JobURLEntry
	Ports     []NamedPort
	Namespace string
	// SharedIn is the worktree a shared service runs in when this worktree only
	// holds it — main, by construction. Empty for a job that runs here.
	SharedIn string
}

// ReachSection is one titled part of the reach block: a worktree's own jobs,
// or the shared services the worktrees hold. Worktree is empty on the second.
type ReachSection struct {
	Title    string
	Worktree string
	Shared   bool
	Lines    []string
}

// NamedPort is a port as a reader reaches it: its declared name, empty when
// only the number is known.
type NamedPort struct {
	Name string
	Port int
}

// JobAddress is where a declared job answers in one worktree: the ports it
// binds there, and the name it is published under when it publishes one. It is
// a property of the worktree's offset, known whether or not anything is
// running.
type JobAddress struct {
	Ports []int
	// Named is Ports with the name each one is declared under, for a surface
	// that lists several: six numbers alone say nothing of which is redis.
	Named []NamedPort
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
	// EnvValues are the .env keys wtm writes in full, from a template. They say
	// what this worktree holds of a shared service, which no port can express.
	EnvValues []EnvValueLink `toml:"env,omitempty" json:"env,omitempty"`
	// Addressing is what those links write. Empty means AddressingNames: a
	// project that publishes names wants its .env values to reach them, and one
	// that publishes none is unaffected either way.
	Addressing Addressing `toml:"addressing,omitempty" json:"addressing,omitempty"`
	// Concurrency is the standing answer to "other worktrees are running jobs".
	// Empty means the question is still open: `run up` asks it once, and writes
	// the answer here when the user asks it to be remembered.
	Concurrency Concurrency `toml:"concurrency,omitempty" json:"concurrency,omitempty"`
	// Isolation is what a new worktree gets when nobody is asked. Empty means
	// IsolationIsolated.
	Isolation Isolation `toml:"isolation,omitempty" json:"isolation,omitempty"`
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
	// JobStatusReaped is a foreground service that outlived the daemon which
	// owned it and was killed by the next one. Distinct from Crashed because the
	// two say opposite things about who acted: crashed is a process that died on
	// its own, reaped is one wtm found still running days later and took down.
	JobStatusReaped JobStatus = "reaped"
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
	// PID and PGID are the fingerprint that lets the next daemon tell a
	// foreground service it lost from one it left running. PGID is what gets
	// signalled: the leader is a `sh -c` that often dies before its children, so
	// asking whether the PID is alive answers no about a group that still holds
	// a port. Zero on a record written before the fingerprint existed, which
	// reads as "not reapable" rather than as a group to guess at.
	PID  int `json:"pid,omitempty"`
	PGID int `json:"pgid,omitempty"`
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
	// MainHolds is the main checkout's own hold on a shared service, carried by
	// the real job: main posts no claim, so this is the only record of it.
	MainHolds bool `json:"main_holds,omitempty"`
}

// NamespaceRef is one worktree's slice of one shared service, named by what it
// takes to recompute it: run.toml still holds the template, so an entry keeps
// only what the worktree itself contributed.
type NamespaceRef struct {
	Job      string `toml:"job"      json:"job"`
	Worktree string `toml:"worktree" json:"worktree"`
	Ordinal  int    `toml:"ordinal"  json:"ordinal"`
}

// NamespaceHolding is what one worktree about to be removed carved out of the
// shared services, read while it still exists: its remove commands run in its
// directory, with its environment. Config holds only the jobs that have
// something to give back.
type NamespaceHolding struct {
	Branch  string
	WorkDir string
	Env     map[string]string
	Config  RunConfig
}

// HeldNamespace is one line of what a removal gives back: the namespace, the
// shared service holding it, and whether that service is up to take it.
type HeldNamespace struct {
	Name string
	Job  string
	Up   bool
}

// NamespaceField is one editable line of the namespace step: which job it
// belongs to, which of the three fields it is, and what has been typed. Vars are
// the variables that field's command may read — the worktree's own plus the
// ports THIS job declares, under the names it declares them by, so nothing has
// to be guessed at.
type NamespaceField struct {
	Job   string
	Field NamespaceFieldKind
	Value string
	Vars  []NamespaceVarGroup
}

// NamespaceVarGroup is one labelled row of the variables a command may read.
// They are grouped by where they come from — the worktree, then this job's own
// ports — because a single run-on line stops being readable as soon as a job
// declares more than one port.
type NamespaceVarGroup struct {
	Label string
	Vars  []string
}

// NamespaceFieldKind is which of a namespace's three inputs a line carries.
type NamespaceFieldKind string

const (
	NamespaceFieldName   NamespaceFieldKind = "name"
	NamespaceFieldCreate NamespaceFieldKind = "create"
	NamespaceFieldRemove NamespaceFieldKind = "remove"
)

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
	// Released marks a shared job this stop let go of without stopping it: the
	// service is still up for another worktree.
	Released bool `json:"released,omitempty"`
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
	// Namespace is what a shared job's start carved out of it for this worktree
	// — the database or realm its create command made sure exists.
	Namespace string `json:"namespace,omitempty"`
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
	// IndexFrozen says a newer wtm owns the index, so this build records nothing
	// it starts. It is read from the file rather than asked of the daemon: the
	// two versions are all it takes to know, and the warning has to work when no
	// daemon is up.
	IndexFrozen bool `json:"index_frozen,omitempty"`
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

// JobTouchChoice is one row of the init step asking which services a task
// changes the data of. Options holds "" first, for none.
type JobTouchChoice struct {
	Job     string
	Label   string
	Touches []string
	Options []string
}

// DataOwner is whose data a job would change, when it is not the worktree's.
type DataOwner string

const (
	// DataOwnerSource is a verbatim worktree's source: its .env names the
	// source's databases and realms.
	DataOwnerSource DataOwner = "source"
	// DataOwnerEveryone is a shared service carving no namespace: one set of
	// data for every worktree.
	DataOwnerEveryone DataOwner = "everyone"
)

// DataRisk is a job this run would start that changes data the worktree does
// not own.
type DataRisk struct {
	Job     string
	Service string
	Owner   DataOwner
	WorkDir string
	// Via is the runner that starts Job as one of its `runs`, empty when the run
	// starts Job itself.
	Via string
}

// PortClaim is one port a job binds in one worktree.
type PortClaim struct {
	Port    int
	Job     string
	WorkDir string
}

// PortClash is a port a job about to start would bind while another job
// already holds it — in another worktree, or in the same run.
type PortClash struct {
	Port   int
	Want   PortClaim
	HeldBy PortClaim
}
