package domain

import (
	"encoding/json"
	"errors"
	"fmt"

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
)

// Phases returns every Phase, in the order a subject's work runs them.
func Phases() []Phase {
	return []Phase{PhaseFastForward, PhaseWorktree, PhaseEnv, PhaseMeta, PhaseOrdinal, PhasePorts, PhaseOnCreate, PhaseMove, PhaseOnClean, PhaseRemove}
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
	Command string `json:"command"`
	Surface string `json:"surface"`
}

// OperationFinished reports that an operation ended: OK, or the code of its failure.
type OperationFinished struct {
	Command string `json:"command"`
	Surface string `json:"surface"`
	OK      bool   `json:"ok"`
	Code    string `json:"code,omitempty"`
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
