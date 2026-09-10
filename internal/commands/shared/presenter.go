package shared

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/flowui"
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
	// opened says the mid-run block already carries its frame. It is a pointer
	// because a presenter is copied by value into each command's own, and the
	// block is one across all of them.
	opened *bool
}

func NewPresenter(cmd *cobra.Command, format string) CLIPresenter {
	return CLIPresenter{Cmd: cmd, Format: format, Human: rules.IsHumanFormat(format), opened: new(bool)}
}

// phase is where everything a run says while it is still running goes: the hook
// phases and the status lines. They used to write straight to stderr, which left
// them the only human output of a migrated command outside the accent bar — and
// what a hook phase leaves behind is kept, so it belongs inside it.
//
// The block opens on its first line and is closed by whoever writes next: every
// terminal block in the tree, the error path included, opens with its own blank
// line. separate asks for the blank that sets a titled section apart from the
// lines above it; a bare status line takes none.
func (p CLIPresenter) phase(separate bool) io.Writer {
	p.openPhase(separate)
	return output.Barred(p.Cmd.ErrOrStderr())
}

// openPhase is phase for a caller that draws on the raw stream itself — the hook
// phase, whose cursor moves cannot go through the bar.
func (p CLIPresenter) openPhase(separate bool) {
	stderr := p.Cmd.ErrOrStderr()
	if p.opened != nil && *p.opened {
		if separate {
			output.Blank(output.Barred(stderr))
		}
		return
	}
	output.FrameStart(stderr)
	if p.opened != nil {
		*p.opened = true
	}
}

func (p CLIPresenter) Stage(params flow.StageParams) error {
	return components.RunLoading(components.LoadingParams{
		Message: params.Message,
		Animate: Animate(p.Cmd, p.Human),
		Work:    params.Work,
	})
}

func (p CLIPresenter) HookPhase(params flow.HookPhaseParams) error {
	if p.Human {
		p.openPhase(true)
	}
	return DrawHookPhase(DrawHookPhaseParams{
		Stderr:  p.Cmd.ErrOrStderr(),
		Human:   p.Human,
		Bar:     p.Human,
		Title:   params.Title,
		LogPath: params.LogPath,
		Run:     params.Run,
	})
}

type DrawHookPhaseParams struct {
	Stderr io.Writer
	// Human titles the phase and lets it be collapsed; a JSON run gets the raw
	// stream and no title.
	Human bool
	// Bar draws the phase inside the accent bar of an already-open block. Only a
	// surface that opened one sets it: a bar with no frame around it is half a
	// block.
	Bar     bool
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
//
// It is one function because the two callers — the migrated commands through
// CLIPresenter, extract and checkout through RunCreateHooksPhase — drifted apart
// once already, and a hook has to read the same whichever command ran it.
func DrawHookPhase(params DrawHookPhaseParams) error {
	log := output.HookLog(params.LogPath)
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

	titled := params.Stderr
	if params.Bar {
		titled = output.Barred(params.Stderr)
	}
	output.SectionTitle(titled, params.Title)
	if !output.IsTerminal(params.Stderr) {
		return params.Run(flow.HookSink{Output: stream})
	}

	view := output.NewHookView(output.HookViewParams{W: params.Stderr, Log: log, LogPath: params.LogPath, Bar: params.Bar})
	defer view.Close()
	return params.Run(flow.HookSink{Output: view, OnHook: view.OnHook})
}

func (p CLIPresenter) Notice(notice flow.Notice) {
	// Backing out changed nothing, which is the definition of the `=` register.
	// A bare sentence there reads as a result, and is how the tree ended up with
	// four wordings for one outcome.
	if notice.IsAbort() {
		output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
			output.Unchanged(w, notice.Text)
		})
		return
	}
	if notice.Kind == flow.NoticeWarning {
		output.Frame(p.Cmd.ErrOrStderr(), func(w io.Writer) {
			output.Warning(w, notice.Text)
		})
		return
	}
	output.Frame(p.Cmd.OutOrStdout(), func(w io.Writer) {
		output.Message(w, notice.Text)
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
		output.Warning(w, notice.Text)
	case flow.NoticeNote:
		output.Message(w, notice.Text)
	default:
		output.Success(w, notice.Text)
	}
}

// statusBlock renders the two registers a titled notice takes. A note is what
// the reader has nothing to do about — the bar and an indent subordinate it. The
// border is kept for what still has to be acted on, which is the only reason it
// reads as one.
func (p CLIPresenter) statusBlock(notice flow.Notice) {
	if !p.Human {
		output.Warning(p.Cmd.ErrOrStderr(), notice.Text)
		for _, line := range notice.Lines {
			output.Message(p.Cmd.ErrOrStderr(), output.Indent+line)
		}
		return
	}
	w := p.phase(true)
	if notice.Kind == flow.NoticeNote {
		output.Section(w, notice.Text, notice.Lines)
		return
	}
	output.Callout(w, notice.Text, notice.Lines)
}

// FlowContext: the flow cannot load the config itself, which reads cobra flags.
func FlowContext(config ConfigResult) flow.Context {
	return flow.Context{
		ProjectDir: config.ProjectDir,
		StateDir:   config.StateDir,
		Config:     config.Config,
	}
}

type FlowPrompterParams struct {
	// Interactive is the prompt-capability gate: a human format, on a terminal, and
	// not bypassed by --yes.
	Interactive bool
	Stderr      bool
}

func FlowPrompter(params FlowPrompterParams) flow.Prompter {
	if !params.Interactive {
		return flow.Unattended{}
	}
	return flowui.New(flowui.Params{Stderr: params.Stderr})
}
