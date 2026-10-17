package domain

import (
	"encoding/json"
	"fmt"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

//go-sumtype:decl Event

// Event is one step of an operation as it happens, reported to the progress view and to the processes listening on the events socket.
type Event interface{ event() }

// UnitStarted reports that the work on one subject (a branch) begins.
type UnitStarted struct {
	Subject string `json:"subject"`
}

// PhaseStarted reports that a phase of a subject's work begins; Log is the file its output goes to, if any.
type PhaseStarted struct {
	Subject string `json:"subject"`
	Phase   string `json:"phase"`
	Log     string `json:"log,omitempty"`
}

// PhaseFinished reports that a phase of a subject's work ended, and whether it failed.
type PhaseFinished struct {
	Subject string `json:"subject"`
	Phase   string `json:"phase"`
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

// WorktreeNumbered reports that a worktree received its number (its ports and env values).
type WorktreeNumbered struct {
	Worktree worktree.Identity `json:"worktree"`
}

// WorktreeProvisioned reports that a worktree's provisioning ended: OK, or the hook that failed and its exit code.
type WorktreeProvisioned struct {
	Worktree worktree.Identity `json:"worktree"`
	OK       bool              `json:"ok"`
	Hook     string            `json:"hook,omitempty"`
	ExitCode *int              `json:"exit_code,omitempty"`
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

func (UnitStarted) event()         {}
func (PhaseStarted) event()        {}
func (PhaseFinished) event()       {}
func (Output) event()              {}
func (UnitFinished) event()        {}
func (WorktreeCreated) event()     {}
func (WorktreeNumbered) event()    {}
func (WorktreeProvisioned) event() {}
func (OperationStarted) event()    {}
func (OperationFinished) event()   {}

// Report receives the events of an operation as they happen.
type Report func(Event)

// The type of each event on the wire, its "type": the events socket's name where it publishes the event, except WorktreeNumbered, which the socket's v1 calls worktree.updated.
const (
	EventUnitStarted         = "unit.started"
	EventPhaseStarted        = "phase.started"
	EventPhaseFinished       = "phase.finished"
	EventOutput              = "phase.output"
	EventUnitFinished        = "unit.finished"
	EventWorktreeCreated     = "worktree.created"
	EventWorktreeNumbered    = "worktree.numbered"
	EventWorktreeProvisioned = "worktree.provisioned"
	EventOperationStarted    = "operation.started"
	EventOperationFinished   = "operation.finished"
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
	case WorktreeNumbered:
		return EventWorktreeNumbered
	case WorktreeProvisioned:
		return EventWorktreeProvisioned
	case OperationStarted:
		return EventOperationStarted
	case OperationFinished:
		return EventOperationFinished
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
	EventWorktreeNumbered:    decodeEvent[WorktreeNumbered],
	EventWorktreeProvisioned: decodeEvent[WorktreeProvisioned],
	EventOperationStarted:    decodeEvent[OperationStarted],
	EventOperationFinished:   decodeEvent[OperationFinished],
}

// UnmarshalEvent returns the event data encodes, as [MarshalEvent] writes it; it refuses a type it does not know.
func UnmarshalEvent(data []byte) (Event, error) {
	var head struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return nil, err
	}

	decode, known := eventDecoders[head.Type]
	if !known {
		return nil, fmt.Errorf("unknown event type %q", head.Type)
	}
	return decode(data)
}

func decodeEvent[T Event](data []byte) (Event, error) {
	var event T
	err := json.Unmarshal(data, &event)
	return event, err
}
