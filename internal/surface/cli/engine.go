package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/registry"
	"github.com/LucasPcq/wtm/internal/service/events"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

type Engine struct {
	registry   func(*dispatch.Dispatcher) dispatch.Registry
	dispatcher func(context.Context) (*dispatch.Dispatcher, error)
	form       Form
	confirm    Confirm
	bindings   map[string]*binding
	catalog    text.Catalog
}

type EngineParams struct {
	Registry   func(*dispatch.Dispatcher) dispatch.Registry
	Dispatcher func(context.Context) (*dispatch.Dispatcher, error)
	Form       Form
	Confirm    Confirm
}

func NewEngine(params EngineParams) *Engine {
	engine := &Engine{registry: params.Registry, dispatcher: params.Dispatcher, form: params.Form, confirm: params.Confirm, bindings: map[string]*binding{}}
	engine.catalog = engine.schema().Catalog()
	return engine
}

type Form func(ctx context.Context, params FormParams) (dispatch.Reply, error)

type FormParams struct {
	Entry    dispatch.Fillable
	Request  json.RawMessage
	Options  dispatch.Options
	Names    render.Names
	Detail   render.Detail
	Progress kernel.Emitter
	Catalog  text.Catalog
}

type Confirm func(params ConfirmParams) (bool, error)

type ConfirmParams struct {
	Command string
	Note    string
}

var production = NewEngine(EngineParams{
	Registry:   registry.Production,
	Dispatcher: projectDispatcher,
	Form:       interactiveForm,
	Confirm:    confirmFollowUp,
})

func projectDispatcher(ctx context.Context) (*dispatch.Dispatcher, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, err := shared.ProjectRoot(ctx, cwd)
	if err != nil {
		return nil, err
	}
	stateDir, err := shared.StateDir(ctx, root)
	if err != nil {
		return nil, err
	}
	return dispatch.New(dispatch.Params{
		Dir:       root,
		StateDir:  stateDir,
		Publisher: events.NewPublisher(events.PublisherParams{ProjectDir: root, CorrelationID: os.Getenv(domain.EnvCorrelationID)}),
	}), nil
}

func (e *Engine) schema() dispatch.Registry {
	return e.registry(dispatch.New(dispatch.Params{}))
}
