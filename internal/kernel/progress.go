package kernel

import (
	"slices"
	"sync"
)

type OpID string

// Of is what every progress carries, whatever its kind.
type Of struct {
	Op      OpID   `json:"op,omitempty"`
	Subject string `json:"subject,omitempty"`
}

func (o *Of) Base() *Of { return o }

//go-sumtype:decl Progress

// Progress is what a command says while it applies, as data: the kind is the
// type. Build it through a Reporter.
type Progress interface {
	Base() *Of
	progress()
}

type UnitStarted struct {
	Of
}

type UnitFinished struct {
	Of
	Status Status `json:"status"`
}

// PhaseStarted names the log the phase's output is kept in, when it has one.
type PhaseStarted struct {
	Of
	Phase string `json:"phase"`
	Log   string `json:"log,omitempty"`
}

type PhaseFinished struct {
	Of
	Phase string `json:"phase"`
}

// Output is one raw line of a hook or a process, never a message of the engine.
type Output struct {
	Of
	Line string `json:"line"`
}

type StatusChanged struct {
	Of
	Code   Code   `json:"code"`
	Params Params `json:"params,omitempty"`
}

func (*UnitStarted) progress()   {}
func (*UnitFinished) progress()  {}
func (*PhaseStarted) progress()  {}
func (*PhaseFinished) progress() {}
func (*Output) progress()        {}
func (*StatusChanged) progress() {}

// Emitter is a carrier, like io.Writer.
type Emitter interface {
	Emit(Progress)
}

type EmitFunc func(Progress)

func (f EmitFunc) Emit(progress Progress) { f(progress) }

// Reporter emits the progress of one subject and keeps the warnings about it;
// a nil Emitter drops the progress.
type Reporter struct {
	emit     Emitter
	subject  string
	warnings *warnings
}

type warnings struct {
	mu   sync.Mutex
	kept []Warning
}

func Report(emit Emitter, subject string) Reporter {
	return Reporter{emit: emit, subject: subject, warnings: &warnings{}}
}

func (r Reporter) UnitStarted() {
	r.send(&UnitStarted{})
}

func (r Reporter) UnitFinished(status Status) {
	r.send(&UnitFinished{Status: status})
}

// Phase announces a phase and returns what announces its end.
func (r Reporter) Phase(name string) (finished func()) {
	return r.phase(PhaseStarted{Phase: name})
}

func (r Reporter) phase(started PhaseStarted) (finished func()) {
	r.send(&started)
	return func() { r.send(&PhaseFinished{Phase: started.Phase}) }
}

func (r Reporter) Output(line string) {
	r.send(&Output{Line: line})
}

func (r Reporter) Status(code Code, params Params) {
	r.send(&StatusChanged{Code: code, Params: params})
}

// Warn keeps a warning about the subject: Each hands it to the subject's item.
func (r Reporter) Warn(warning Warning) {
	r.warnings.mu.Lock()
	defer r.warnings.mu.Unlock()
	r.warnings.kept = append(r.warnings.kept, warning)
}

func (r Reporter) Warnings() []Warning {
	r.warnings.mu.Lock()
	defer r.warnings.mu.Unlock()
	return slices.Clone(r.warnings.kept)
}

func (r Reporter) send(progress Progress) {
	if r.emit == nil {
		return
	}
	progress.Base().Subject = r.subject
	r.emit.Emit(progress)
}
