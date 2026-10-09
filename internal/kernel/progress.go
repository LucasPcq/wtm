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
// output of a hook or a process, never a message of the engine. Build it
// through a Reporter.
type Progress struct {
	Op      OpID         `json:"op,omitempty"`
	Subject string       `json:"subject,omitempty"`
	Kind    ProgressKind `json:"kind"`
	Code    Code         `json:"code,omitempty"`
	Params  Params       `json:"params,omitempty"`
	Line    string       `json:"line,omitempty"`
}

// Emitter is a carrier, like io.Writer.
type Emitter interface {
	Emit(Progress)
}

type EmitFunc func(Progress)

func (f EmitFunc) Emit(progress Progress) { f(progress) }

// Reporter emits the progress of one subject; a nil Emitter drops it.
type Reporter struct {
	emit    Emitter
	subject string
}

func Report(emit Emitter, subject string) Reporter {
	return Reporter{emit: emit, subject: subject}
}

func (r Reporter) UnitStarted() {
	r.send(Progress{Kind: ProgressUnitStarted})
}

func (r Reporter) UnitFinished(status Status) {
	r.send(Progress{Kind: ProgressUnitFinished, Params: Params{ParamStatus: string(status)}})
}

// Phase announces a phase and returns what announces its end.
func (r Reporter) Phase(name string) (finished func()) {
	params := Params{ParamPhase: name}
	r.send(Progress{Kind: ProgressPhaseStarted, Params: params})
	return func() { r.send(Progress{Kind: ProgressPhaseFinished, Params: params}) }
}

// Output passes on one raw line of a hook or a process.
func (r Reporter) Output(line string) {
	r.send(Progress{Kind: ProgressOutput, Line: line})
}

func (r Reporter) Status(code Code, params Params) {
	r.send(Progress{Kind: ProgressStatus, Code: code, Params: params})
}

func (r Reporter) send(progress Progress) {
	if r.emit == nil {
		return
	}
	progress.Subject = r.subject
	r.emit.Emit(progress)
}
