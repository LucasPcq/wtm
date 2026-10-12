package cli

import (
	"context"
	"encoding/json"
	"os"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/service/events"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/cli/shared"
)

// Engine is what the generated commands run on: the registry, and the
// dispatcher it is reached through, built only once a command runs — the tree
// is built outside any repository, for --help and the docs.
type Engine struct {
	registry   func(*dispatch.Dispatcher) dispatch.Registry
	dispatcher func(context.Context) (*dispatch.Dispatcher, error)
	form       Form
	confirm    Confirm
	bindings   map[string]*binding
}

type EngineParams struct {
	Registry   func(*dispatch.Dispatcher) dispatch.Registry
	Dispatcher func(context.Context) (*dispatch.Dispatcher, error)
	Form       Form
	Confirm    Confirm
}

func NewEngine(params EngineParams) *Engine {
	return &Engine{registry: params.Registry, dispatcher: params.Dispatcher, form: params.Form, confirm: params.Confirm, bindings: map[string]*binding{}}
}

// Form runs a command interactively: it fills the request, shows the plan and
// asks before applying it. Without one, an interactive run takes the
// unattended route.
type Form func(ctx context.Context, params FormParams) (dispatch.Reply, error)

type FormParams struct {
	Entry    dispatch.Fillable
	Request  json.RawMessage
	Options  dispatch.Options
	Names    render.Names
	Detail   render.Detail
	Progress kernel.Emitter
}

// Confirm asks whether to run a follow-up once an interactive run concluded.
type Confirm func(params ConfirmParams) (bool, error)

type ConfirmParams struct {
	Command string
	Note    string
}

var production = NewEngine(EngineParams{
	Registry:   (*dispatch.Dispatcher).Registry,
	Dispatcher: projectDispatcher,
	Form:       interactiveForm,
	Confirm:    confirmFollowUp,
})

// projectDispatcher runs on the repository the command is run from: its main
// worktree, its state directory, its event bus.
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

// schema reads the registry with no dispatcher behind it: enough for names and
// fields, never called.
func (e *Engine) schema() dispatch.Registry {
	return e.registry(dispatch.New(dispatch.Params{}))
}
