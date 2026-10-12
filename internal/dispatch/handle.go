package dispatch

import (
	"fmt"
	"reflect"

	"github.com/LucasPcq/wtm/internal/kernel"
)

const planShapeFmt = "%s: the detail of a plan is a struct, not %s"

type Handle[Req, F, P, D any] struct {
	dispatcher *Dispatcher
	command    kernel.Command[Req, F, P, D]
}

type RegisterParams[Req, F, P, D any] struct {
	Dispatcher *Dispatcher
	Command    kernel.Command[Req, F, P, D]
}

// Register panics on a plan whose detail is not a struct: every surface shows
// a plan as its fields, so a plan that has none could not be shown.
func Register[Req, F, P, D any](params RegisterParams[Req, F, P, D]) *Handle[Req, F, P, D] {
	if detail := reflect.TypeFor[P](); detail.Kind() != reflect.Struct {
		panic(fmt.Sprintf(planShapeFmt, params.Command.Name, detail))
	}
	return &Handle[Req, F, P, D]{dispatcher: params.Dispatcher, command: params.Command}
}

func (h *Handle[Req, F, P, D]) Name() string { return h.command.Name }

func (h *Handle[Req, F, P, D]) Schema() []kernel.FieldSpec {
	specs := make([]kernel.FieldSpec, 0, len(h.command.Fields))
	for _, field := range h.command.Fields {
		specs = append(specs, field.Spec)
	}
	return specs
}
