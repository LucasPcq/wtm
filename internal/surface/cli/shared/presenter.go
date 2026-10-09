package shared

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/events"
	"github.com/LucasPcq/wtm/internal/surface/cli/render"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
	"github.com/LucasPcq/wtm/internal/surface/tui/flowui"
)

// CLIPresenter is the CLI half of flow.Presenter: the flow decides what happens,
// this decides how it reads. It lives here rather than beside one command's
// runners because every migrated command needs exactly this one.
type CLIPresenter struct {
	Cmd    *cobra.Command
	Format string
	// human means the output is meant for a person: progress is animated and the
	// hook phase gets its title.
	Human bool
}

func NewPresenter(cmd *cobra.Command, format string) CLIPresenter {
	return CLIPresenter{Cmd: cmd, Format: format, Human: rules.IsHumanFormat(format)}
}

// OpenBlock puts the caller inside the block a run keeps while it is still
// running — its status lines and its hook phases — and returns the writer that
// block draws on. separate asks for the blank that sets a titled section apart
// from the lines above it; a bare status line takes none.
//
// The surface, not the caller, says whether a block is already open: the frames
// beside this one are written by code that never sees this presenter.
func OpenBlock(w io.Writer, separate bool) io.Writer {
	if !render.BlockOpen(w) {
		render.FrameStart(w)
		return render.Barred(w)
	}
	barred := render.Barred(w)
	if separate {
		render.Blank(barred)
	}
	return barred
}

func (p CLIPresenter) phase(separate bool) io.Writer {
	return OpenBlock(p.Cmd.ErrOrStderr(), separate)
}

func (p CLIPresenter) Stage(ctx context.Context, params flow.StageParams) error {
	return components.RunLoading(ctx, components.LoadingParams{
		Message: params.Message,
		Animate: Animate(p.Cmd, p.Human),
		Work:    func() error { return params.Work(ctx) },
	})
}

func (p CLIPresenter) HookPhase(params flow.HookPhaseParams) error {
	return DrawHookPhase(DrawHookPhaseParams{
		Stderr:  p.Cmd.ErrOrStderr(),
		Human:   p.Human,
		Title:   params.Title,
		LogPath: params.LogPath,
		Run:     params.Run,
	})
}

type DrawHookPhaseParams struct {
	Stderr io.Writer
	// Human titles the phase and lets it be collapsed; a JSON run gets the raw
	// stream and no title.
	Human   bool
	Title   string
	LogPath string
	Run     func(flow.HookSink) error
}

// DrawHookPhase draws the phase only where it can be undrawn. A terminal gets a
// bounded tail replaced by one result line per hook; anything else — a pipe, a
// CI log, a JSON run, a quiet one — gets the stream whole, which is what a
// reader who cannot watch it live came for. Whichever it is, the sink is the
// writer the command was given and the log is written: a hook must never find
// its own way to the terminal, and its record must not depend on who was
// watching.
func DrawHookPhase(params DrawHookPhaseParams) error {
	log := render.HookLog(params.LogPath)
	if log != nil {
		defer func() { _ = log.Close() }()
	}

	stream := params.Stderr
	if log != nil {
		stream = io.MultiWriter(params.Stderr, log)
	}

	if !params.Human {
		return params.Run(flow.HookSink{Output: stream})
	}

	// The phase joins the run's block rather than opening one beside it.
	render.SectionTitle(OpenBlock(params.Stderr, true), params.Title)
	if !render.IsTerminal(params.Stderr) {
		return params.Run(flow.HookSink{Output: stream})
	}

	view := render.NewHookView(render.HookViewParams{W: params.Stderr, Log: log, LogPath: params.LogPath, Bar: true})
	defer view.Close()
	return params.Run(flow.HookSink{Output: view, OnHook: view.OnHook})
}

func (p CLIPresenter) Notice(notice flow.Notice) {
	// Backing out changed nothing, which is the definition of the `=` register.
	// A bare sentence there reads as a result, and is how the tree ended up with
	// four wordings for one outcome.
	if notice.IsAbort() {
		render.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			render.Unchanged(w, notice.Text)
		})
		MarkCancelled(p.Cmd)
		return
	}
	if notice.Kind == flow.NoticeWarning {
		render.Frame(p.Cmd.ErrOrStderr(), func(w io.Writer) {
			render.Warning(w, notice.Text)
		})
		return
	}
	render.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		render.Message(w, notice.Text)
	})
}

// Status is one line inside an ongoing phase, on stderr. It is a diagnostic and
// not the answer, so it is emitted whatever the format — stdout is the machine
// contract, stderr never was — but a JSON run gets it as plain lines: a bordered,
// coloured box in a CI log is a picture nobody asked for.
func (p CLIPresenter) Status(notice flow.Notice) {
	if len(notice.Lines) > 0 {
		p.statusBlock(notice)
		return
	}
	if !p.Human {
		p.statusLine(p.Cmd.ErrOrStderr(), notice)
		return
	}
	p.statusLine(p.phase(false), notice)
}

func (p CLIPresenter) statusLine(w io.Writer, notice flow.Notice) {
	switch notice.Kind {
	case flow.NoticeWarning:
		render.Warning(w, notice.Text)
	case flow.NoticeNote:
		render.Unchanged(w, notice.Text)
	default:
		render.Success(w, notice.Text)
	}
}

func (p CLIPresenter) statusBlock(notice flow.Notice) {
	if !p.Human {
		render.Warning(p.Cmd.ErrOrStderr(), notice.Text)
		for _, line := range notice.Lines {
			render.Message(p.Cmd.ErrOrStderr(), render.Indent+line)
		}
		return
	}
	w := p.phase(true)
	if notice.Kind == flow.NoticeNote {
		render.Section(w, notice.Text, notice.Lines)
		return
	}
	render.Callout(w, notice.Text, notice.Lines)
}

// FlowContext: the flow cannot load the config itself, which reads cobra flags.
// Every command publishes what its flow changes, whoever ran it.
func FlowContext(config ConfigResult) flow.Context {
	return flow.Context{
		ProjectDir: config.ProjectDir,
		StateDir:   config.StateDir,
		Config:     config.Config,
		Publisher:  events.NewPublisher(events.PublisherParams{ProjectDir: config.ProjectDir, CorrelationID: os.Getenv(domain.EnvCorrelationID)}),
	}
}

type FlowPrompterParams struct {
	// Interactive is the prompt-capability gate: a human format, on a terminal, and
	// not bypassed by --yes.
	Interactive bool
	Stderr      bool
}

func FlowPrompter(ctx context.Context, params FlowPrompterParams) flow.Prompter {
	if !params.Interactive {
		return flow.Unattended{}
	}
	return InteractivePrompter(ctx, params)
}

// StdinIsTerminal is the terminal gate of a command whose wizard renders on
// stderr, so stdout may be consumed. A var so a test can stand in for the terminal.
var StdinIsTerminal = func() bool { return term.IsTerminal(int(os.Stdin.Fd())) }

// InteractivePrompter is the wizard a fully interactive run asks through. A
// var so a test can stand in for the terminal.
var InteractivePrompter = func(ctx context.Context, params FlowPrompterParams) flow.Prompter {
	return flowui.New(ctx, flowui.Params{Stderr: params.Stderr})
}
