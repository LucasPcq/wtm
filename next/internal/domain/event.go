package domain

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

//go-sumtype:decl Event

// Event is what happens to a repository's worktrees as it happens: a step of an operation, reported to the progress view and to the processes listening on the event bus, or a line of the event stream itself (a snapshot, ready, a repository joining or leaving it).
type Event interface{ event() }

// UnitStarted reports that the work on one subject (a branch) begins.
type UnitStarted struct {
	Subject string `json:"subject"`
}

// Phase is one phase of a subject's work, as PhaseStarted and PhaseFinished name it: a closed set, published as an enum so that a surface names each one.
type Phase string

// The phases of a subject's work.
const (
	PhaseFastForward Phase = "fast_forward" // its source branch fast-forwarded to origin, before the worktree is added
	PhaseWorktree    Phase = "worktree"     // git adds the worktree
	PhaseEnv         Phase = "env"          // its .env files copied
	PhaseMeta        Phase = "meta"         // its meta record written
	PhaseOrdinal     Phase = "ordinal"      // its ordinal, the number its ports are offset by
	PhasePorts       Phase = "ports"        // its ports settled in its .env files
	PhaseOnCreate    Phase = "on_create"    // the project's on_create hooks
	PhaseMove        Phase = "move"         // extract moves its changes
	PhaseOnClean     Phase = "on_clean"     // the project's on_clean hooks, before a worktree is removed
	PhaseRemove      Phase = "remove"       // git removes the worktree, its branch and its meta record go
	PhaseExec        Phase = "exec"         // a command exec runs in the worktree
)

// Phases returns every Phase, in the order a subject's work runs them.
func Phases() []Phase {
	return []Phase{PhaseFastForward, PhaseWorktree, PhaseEnv, PhaseMeta, PhaseOrdinal, PhasePorts, PhaseOnCreate, PhaseMove, PhaseOnClean, PhaseRemove, PhaseExec}
}

// PhaseStarted reports that a phase of a subject's work begins; Log is the file its output goes to, if any.
type PhaseStarted struct {
	Subject string `json:"subject"`
	Phase   Phase  `json:"phase"`
	Log     string `json:"log,omitempty"`
}

// PhaseFinished reports that a phase of a subject's work ended, and whether it failed.
type PhaseFinished struct {
	Subject string `json:"subject"`
	Phase   Phase  `json:"phase"`
	Failed  bool   `json:"failed"`
}

// Output is one line printed by a phase of a subject's work (a hook's output).
type Output struct {
	Subject string `json:"subject"`
	Line    string `json:"line"`
}

// UnitFinished reports that the work on one subject ended, with its status.
type UnitFinished struct {
	Subject string `json:"subject"`
	Status  Status `json:"status"`
}

// WorktreeCreated reports that a worktree was added.
type WorktreeCreated struct {
	Worktree worktree.Identity `json:"worktree"`
}

// WorktreeUpdated reports that fields of a worktree's identity changed, Changed naming them: a worktree numbered changes its ordinal.
type WorktreeUpdated struct {
	Worktree worktree.Identity        `json:"worktree"`
	Changed  []worktree.IdentityField `json:"changed"`
}

// WorktreeChanged reports that what a worktree holds changed with no operation of wtm, as the event stream polls it (a commit or a checkout from a terminal, a job that started or stopped): Changed names what. A worktree the stream had not seen is one, every field changed.
type WorktreeChanged struct {
	Worktree worktree.Identity      `json:"worktree"`
	Changed  []worktree.PolledField `json:"changed"`
}

// WorktreeProvisioned reports that a worktree's provisioning ended: OK, or the hook that failed and its exit code.
type WorktreeProvisioned struct {
	Worktree worktree.Identity `json:"worktree"`
	OK       bool              `json:"ok"`
	Hook     string            `json:"hook,omitempty"`
	ExitCode *int              `json:"exit_code,omitempty"`
}

// WorktreeRemoved reports that a worktree was removed, with its branch and its meta record: Worktree is its identity as it stood before.
type WorktreeRemoved struct {
	Worktree worktree.Identity `json:"worktree"`
}

// OperationStarted reports that an operation began, from which command and surface.
type OperationStarted struct {
	Command string  `json:"command"`
	Surface Surface `json:"surface"`
}

// OperationFinished reports that an operation ended: OK, or the code of its failure.
type OperationFinished struct {
	Command string  `json:"command"`
	Surface Surface `json:"surface"`
	OK      bool    `json:"ok"`
	Code    string  `json:"code,omitempty"`
}

// Snapshot is every worktree of a repository on a branch, as the stream opens on it: a reader resets what it knows of the repository from it.
type Snapshot struct {
	Worktrees []worktree.Identity `json:"worktrees"`
}

// MarshalJSON writes the snapshot with its worktrees always an array, empty included: a reader resets its state from it.
func (s Snapshot) MarshalJSON() ([]byte, error) {
	type plain Snapshot
	if s.Worktrees == nil {
		s.Worktrees = []worktree.Identity{}
	}

	return json.Marshal(plain(s))
}

// Ready reports that the stream sent its snapshots: every event after it is a change.
type Ready struct{}

// RepoAdded reports that a repository joined those wtm knows.
type RepoAdded struct{}

// RepoRemoved reports that a repository left those wtm knows: it is gone, or no longer initialized.
type RepoRemoved struct{}

func (UnitStarted) event()         {}
func (PhaseStarted) event()        {}
func (PhaseFinished) event()       {}
func (Output) event()              {}
func (UnitFinished) event()        {}
func (WorktreeCreated) event()     {}
func (WorktreeUpdated) event()     {}
func (WorktreeChanged) event()     {}
func (WorktreeProvisioned) event() {}
func (WorktreeRemoved) event()     {}
func (OperationStarted) event()    {}
func (OperationFinished) event()   {}
func (Snapshot) event()            {}
func (Ready) event()               {}
func (RepoAdded) event()           {}
func (RepoRemoved) event()         {}
func (JobStarted) event()          {}
func (JobStopped) event()          {}
func (JobCrashed) event()          {}
func (JobExited) event()           {}

// JobChange is what a job event says: the worktree the job runs in (its branch and path: the run daemon reads no more of it), the job, and, for one that ended, its exit code and the last lines it printed.
type JobChange struct {
	Worktree  JobWorktree `json:"worktree"`
	Job       EventJob    `json:"job"`
	ExitCode  *int        `json:"exit_code,omitempty"`
	LastLines []string    `json:"last_lines,omitempty"`
}

// JobWorktree is the worktree a job event is about, as its client named it to the run daemon.
type JobWorktree struct {
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// EventJob is the job a job event is about: its name, its kind, its URL, and whether it is detached (a stack its launcher left running).
type EventJob struct {
	Name     string          `json:"name"`
	Kind     project.JobKind `json:"kind"`
	URL      string          `json:"url,omitempty"`
	Detached bool            `json:"detached,omitempty"`
}

// JobStarted reports that the run daemon started a job: a foreground service is up, a detached launcher left its stack running, a task began.
type JobStarted struct{ JobChange }

// JobStopped reports that a job stopped because a client asked.
type JobStopped struct{ JobChange }

// JobCrashed reports that a service ended on its own, or a task failed, with its exit code (-1 for a signal) and its last lines.
type JobCrashed struct{ JobChange }

// JobExited reports that a task ran to its end with exit code 0.
type JobExited struct{ JobChange }

// Report receives the events of an operation as they happen.
type Report func(Event)

// The type of each event on the wire, its "type".
const (
	EventUnitStarted         = "unit.started"
	EventPhaseStarted        = "phase.started"
	EventPhaseFinished       = "phase.finished"
	EventOutput              = "phase.output"
	EventUnitFinished        = "unit.finished"
	EventWorktreeCreated     = "worktree.created"
	EventWorktreeUpdated     = "worktree.updated"
	EventWorktreeChanged     = "worktree.changed"
	EventWorktreeProvisioned = "worktree.provisioned"
	EventWorktreeRemoved     = "worktree.removed"
	EventOperationStarted    = "operation.started"
	EventOperationFinished   = "operation.finished"
	EventSnapshot            = "snapshot"
	EventReady               = "ready"
	EventRepoAdded           = "repo.added"
	EventRepoRemoved         = "repo.removed"
	EventJobStarted          = "job.started"
	EventJobStopped          = "job.stopped"
	EventJobCrashed          = "job.crashed"
	EventJobExited           = "job.exited"
)

// EventType returns the type of event on the wire.
func EventType(event Event) string {
	switch event.(type) {
	case UnitStarted:
		return EventUnitStarted
	case PhaseStarted:
		return EventPhaseStarted
	case PhaseFinished:
		return EventPhaseFinished
	case Output:
		return EventOutput
	case UnitFinished:
		return EventUnitFinished
	case WorktreeCreated:
		return EventWorktreeCreated
	case WorktreeUpdated:
		return EventWorktreeUpdated
	case WorktreeChanged:
		return EventWorktreeChanged
	case WorktreeProvisioned:
		return EventWorktreeProvisioned
	case WorktreeRemoved:
		return EventWorktreeRemoved
	case OperationStarted:
		return EventOperationStarted
	case OperationFinished:
		return EventOperationFinished
	case Snapshot:
		return EventSnapshot
	case Ready:
		return EventReady
	case RepoAdded:
		return EventRepoAdded
	case RepoRemoved:
		return EventRepoRemoved
	case JobStarted:
		return EventJobStarted
	case JobStopped:
		return EventJobStopped
	case JobCrashed:
		return EventJobCrashed
	case JobExited:
		return EventJobExited
	}
	return ""
}

// MarshalEvent returns event as a JSON object whose "type" names it, followed by its fields.
func MarshalEvent(event Event) ([]byte, error) {
	return discriminated("type", EventType(event), event)
}

var eventDecoders = map[string]func([]byte) (Event, error){
	EventUnitStarted:         decodeEvent[UnitStarted],
	EventPhaseStarted:        decodeEvent[PhaseStarted],
	EventPhaseFinished:       decodeEvent[PhaseFinished],
	EventOutput:              decodeEvent[Output],
	EventUnitFinished:        decodeEvent[UnitFinished],
	EventWorktreeCreated:     decodeEvent[WorktreeCreated],
	EventWorktreeUpdated:     decodeEvent[WorktreeUpdated],
	EventWorktreeChanged:     decodeEvent[WorktreeChanged],
	EventWorktreeProvisioned: decodeEvent[WorktreeProvisioned],
	EventWorktreeRemoved:     decodeEvent[WorktreeRemoved],
	EventOperationStarted:    decodeEvent[OperationStarted],
	EventOperationFinished:   decodeEvent[OperationFinished],
	EventSnapshot:            decodeEvent[Snapshot],
	EventReady:               decodeEvent[Ready],
	EventRepoAdded:           decodeEvent[RepoAdded],
	EventRepoRemoved:         decodeEvent[RepoRemoved],
	EventJobStarted:          decodeEvent[JobStarted],
	EventJobStopped:          decodeEvent[JobStopped],
	EventJobCrashed:          decodeEvent[JobCrashed],
	EventJobExited:           decodeEvent[JobExited],
}

// ErrUnknownEventType is the error of an event whose type this wtm does not know: a newer wtm's, which a reader ignores.
var ErrUnknownEventType = errors.New("unknown event type")

// UnmarshalEvent returns the event data encodes, as [MarshalEvent] writes it; it refuses a type it does not know with an error wrapping [ErrUnknownEventType].
func UnmarshalEvent(data []byte) (Event, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}

	decode, known := eventDecoders[head.Type]
	if !known {
		return nil, fmt.Errorf("%w %q", ErrUnknownEventType, head.Type)
	}
	return decode(data)
}

func decodeEvent[T Event](data []byte) (Event, error) {
	var event T
	err := json.Unmarshal(data, &event)
	return event, err
}
