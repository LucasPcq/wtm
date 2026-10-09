package kernel

type OpID string

type ProgressKind string

const (
	ProgressUnitStarted   ProgressKind = "unit.started"
	ProgressUnitFinished  ProgressKind = "unit.finished"
	ProgressPhaseStarted  ProgressKind = "phase.started"
	ProgressPhaseFinished ProgressKind = "phase.finished"
	ProgressOutput        ProgressKind = "output"
	ProgressStatus        ProgressKind = "status"
)

// Progress is what a command says while it applies, as data. Line is raw
// output of a hook or a process, never a message of the engine.
type Progress struct {
	Op      OpID              `json:"op,omitempty"`
	Subject string            `json:"subject,omitempty"`
	Kind    ProgressKind      `json:"kind"`
	Code    Code              `json:"code,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
	Line    string            `json:"line,omitempty"`
}

// Emitter is a carrier, like io.Writer.
type Emitter interface {
	Emit(Progress)
}

type EmitFunc func(Progress)

func (f EmitFunc) Emit(progress Progress) { f(progress) }

type discard struct{}

func (discard) Emit(Progress) {}
