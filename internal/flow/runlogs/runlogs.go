// Package runlogs is the seam between a surface showing a worktree's jobs and
// the daemon holding them: what a job looks like, how its live output is
// subscribed to, and how a profile's start sequence is reported.
package runlogs

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
)

type JobView struct {
	Name string
	// WorkDir is the worktree the job belongs to, as git spells it — the
	// daemon's half of its key, and what a merged board routes on. Two worktrees
	// running the same profile hold two jobs named `web`, and the name alone no
	// longer identifies either.
	WorkDir string
	// Worktree is that worktree's branch, which is what a reader recognises.
	Worktree  string
	Kind      domain.JobKind
	Status    domain.JobStatus
	StartedAt time.Time
	ExitCode  *int
	// Attachable is false for a job with no live output to bind to: a detached
	// launcher, whose stream ended with the launcher itself, and any job that is
	// no longer running. History is what is left to show for those. A task is
	// attachable while it runs — the daemon streams it like any other job, and
	// its pane is the only place its colours and redraws survive.
	Attachable bool
	// Address is where the job answers in this worktree — its ports, and the
	// name it is published under. It is a property of the worktree's offset, so
	// it is known whether or not the job runs, and it is carried here so every
	// surface reads one source: `run up`, `run logs` and the dashboard used to
	// compute it three times, and only the first of them showed it.
	Address domain.JobAddress
}

type Size struct {
	Cols int
	Rows int
}

type Stream interface {
	// Chunks carries the job's bytes exactly as the job wrote them, escape
	// sequences included — an emulator needs them untouched. A chunk is shared
	// with the job's other subscribers: read it, never write into it. The channel
	// closes when the output ends or the stream is closed.
	Chunks() <-chan []byte
	// Write feeds the job's stdin. Nothing arbitrates the PTY between
	// subscribers: two of them writing at once interleave their bytes.
	Write(p []byte) error
	// Resize sizes the job's PTY, the only way its child ever reflows. Every
	// subscriber sizes for itself, so the last one to call wins for all of them.
	Resize(Size) error
	Close() error
}

type AttachParams struct {
	Job string
	// WorkDir names the worktree the job belongs to. A single-worktree board
	// ignores it; a merged one routes on it.
	WorkDir string
	// Size sizes the job's PTY as the subscription opens. Zero leaves it as it is.
	Size Size
}

type HistoryParams struct {
	Job string
	// WorkDir names the worktree, as in AttachParams.
	WorkDir string
	Lines   int
}

type Board interface {
	Jobs() []JobView
	// Refresh re-reads the daemon's view of the jobs.
	Refresh() error
	// Attach subscribes to a job's live output, and refuses one that has none.
	Attach(AttachParams) (Stream, error)
	// History reads back the lines a job persisted, whether or not it still runs.
	History(HistoryParams) ([]string, error)
}

type StartRequest struct {
	Job     domain.JobConfig
	WorkDir string
	LogDir  string
	Env     map[string]string
	// Routes are the names the proxy is to serve once this job runs — its own,
	// and one per published job it runs itself. Empty when nothing is published
	// or the proxy is off.
	Routes []domain.JobRoute
	// Shared is where a shared job runs — the main checkout, with its own
	// environment and log directory. Nil for a per-worktree job.
	Shared *domain.SharedJobContext
	// OnOutput receives what the job writes while it starts — everything for a
	// task or a detached launcher, nothing for a job the daemon backgrounds.
	OnOutput func([]byte)
}

// StartResult is the daemon's answer. Refused is the daemon declining to start
// the job, as opposed to the error Start returns when it could not be reached.
type StartResult struct {
	Refused bool
	Message string
	// ExitCode is what a task exited with, nil for a job whose lifetime does not
	// end with this answer.
	ExitCode *int
	// Ports are the ports the job bound, base plus this worktree's offset. Empty
	// for a job that declares none.
	Ports map[string]int
	// ProxyPort is what the daemon's proxy is really serving on, zero when it is
	// off or could not bind.
	ProxyPort int
	// PublicPort is what a named URL announces, which the redirection may have
	// stripped of its port.
	PublicPort int
}

type AttachRequest struct {
	Name    string
	WorkDir string
	Size    Size
}

type TailRequest struct {
	LogDir string
	Job    string
	Lines  int
}

// Service is the daemon as this package uses it: the seam NewService implements
// over internal/service/process, and a test replaces.
type Service interface {
	// Start returns as soon as the run stops watching: cancelling ends the
	// conversation about the job, never the job.
	Start(context.Context, StartRequest) (StartResult, error)
	List(workDir string) ([]domain.JobInfo, error)
	Attach(AttachRequest) (Stream, error)
	Tail(TailRequest) ([]string, error)
}

type Phase int

const (
	// PhaseStarting through PhaseDone are one job's own steps; PhaseAborted and
	// PhaseReady conclude the sequence and carry its Outcome.
	PhaseStarting Phase = iota
	PhaseOutput
	PhaseStarted
	PhaseDone
	PhaseFailed
	PhaseAborted
	PhaseProbed
	// PhaseCrashed reports a job the daemon accepted that was gone by the end of
	// the sequence.
	PhaseCrashed
	PhaseNotice
	PhaseReady
)

// Prober answers which of the given ports are listening. It is the seam over
// service/portprobe: the flow states what it wants checked, the surface owns
// the budget and the dialing.
type Prober interface {
	// Listening dials until settled says the answer is complete or the surface's
	// own budget runs out.
	Listening(ctx context.Context, ports []int, settled func(map[int]bool) bool) map[int]bool
}

// Event is one step of a profile's start sequence. Rendering it is the surface's
// business: nothing here formats, colours or frames anything.
type Event struct {
	Phase Phase
	Job   string
	// WorkDir and Worktree name where this step happened. A run over several
	// worktrees merges their sequences into one Sink, and a surface that cannot
	// tell two `web` apart cannot report either.
	WorkDir  string
	Worktree string
	// Kind is the job's, carried on PhaseOutput so a surface knows what it is
	// reading: a task's bytes come off a pipe, with none of the line discipline
	// a PTY applies on the way out.
	Kind domain.JobKind
	// Step places the job in the sequence, from 1 to Steps.
	Step  int
	Steps int
	// Chunk is a PhaseOutput's raw bytes, under the read-only contract Stream
	// states.
	Chunk []byte
	// Reason is what the daemon answered when a job could not be started, and
	// the state it was found in on PhaseCrashed.
	Reason string
	// ExitCode is what a PhaseCrashed job left behind, nil when its process was
	// never reaped.
	ExitCode *int
	// AlreadyRunning marks a start refused because the job was already up, which
	// is benign: it counts as running and the sequence carries on.
	AlreadyRunning bool
	// Outcome is the partial state PhaseAborted reports and the final state
	// PhaseReady reports; zero on every other phase.
	Outcome Outcome
	// Ports are what a PhaseStarted or PhaseDone job bound, for a surface that
	// tells the user where to reach it.
	Ports map[string]int
	// URL is where a PhaseStarted or PhaseDone job is reachable, empty for one
	// that publishes no name.
	URL string
	// Held are the addresses a PhaseStarted or PhaseDone job answers for besides
	// its own — the apps a runner started, which have no line of their own.
	Held []domain.JobURLEntry
	// Probes is what PhaseProbed observed on one job's declared ports.
	Probes []domain.PortProbe
	// DevOrigins are the config lines a PhaseStarted job needs before it will
	// answer under the name the proxy serves it under.
	DevOrigins []domain.DevOriginFix
	// Notice is what PhaseNotice has to say: a fact about the run that belongs
	// to no single job. Empty on every other phase.
	Notice string
}

// NextConfigLookup reads a job's next.config.*. It is a seam only so a test can
// answer without a disk; a nil one is filled in by the flow itself, because
// which file to read is the flow's decision and not a surface's.
type NextConfigLookup func(job domain.JobConfig) (path, source string)

// Sink is emitted to on the goroutine that called Run.
type Sink interface {
	Emit(Event)
}

type noSink struct{}

func (noSink) Emit(Event) {}

// StartFunc is a start sequence as a surface drives it: it draws first, then
// calls this when it is ready to report. Cancelling the context ends the
// reporting, never the jobs — a view the reader walked away from stops being
// written to while the daemon keeps running what it started.
//
// It answers with one Outcome per worktree it covered. A run over a single
// worktree is an Outcomes of one rather than a shape of its own: the surfaces
// read the arity, so there is nothing to branch on here.
type StartFunc func(context.Context, Sink) (Outcomes, error)
