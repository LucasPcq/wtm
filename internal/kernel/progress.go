package kernel

import (
	"slices"
	"sync"
)

type Of struct {
	Op      OpID   `json:"op,omitempty"`
	Subject string `json:"subject,omitempty"`
}

func (o *Of) Base() *Of { return o }

//go-sumtype:decl Progress

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

type PhaseStarted struct {
	Of
	Phase string `json:"phase"`
	Log   string `json:"log,omitempty"`
}

type PhaseFinished struct {
	Of
	Phase  string `json:"phase"`
	Failed bool   `json:"failed,omitempty"`
}

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

type Emitter interface {
	Emit(Progress)
}

type EmitFunc func(Progress)

func (f EmitFunc) Emit(progress Progress) { f(progress) }

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

func (r Reporter) Phase(name string) (finished func(failed bool)) {
	return r.phase(PhaseStarted{Phase: name})
}

func (r Reporter) phase(started PhaseStarted) (finished func(failed bool)) {
	r.send(&started)
	return func(failed bool) { r.send(&PhaseFinished{Phase: started.Phase, Failed: failed}) }
}

func (r Reporter) Output(line string) {
	r.send(&Output{Line: line})
}

func (r Reporter) Status(code Code, params Params) {
	r.send(&StatusChanged{Code: code, Params: params})
}

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
