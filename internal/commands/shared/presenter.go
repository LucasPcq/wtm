package shared

import (
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
}

func NewPresenter(cmd *cobra.Command, format string) CLIPresenter {
	return CLIPresenter{Cmd: cmd, Format: format, Human: rules.IsHumanFormat(format)}
}

func (p CLIPresenter) Stage(params flow.StageParams) error {
	return components.RunLoading(components.LoadingParams{
		Message: params.Message,
		Animate: p.Human,
		Work:    params.Work,
	})
}

// HookPhase draws the phase only where it can be undrawn. A terminal gets a
// bounded tail replaced by one result line per hook; anything else — a pipe, a
// CI log, a JSON run — gets the stream whole, which is what a reader who cannot
// watch it live came for.
func (p CLIPresenter) HookPhase(params flow.HookPhaseParams) error {
	stderr := p.Cmd.ErrOrStderr()
	if !p.Human {
		return params.Run(flow.HookSink{Output: stderr})
	}

	output.HooksSection(stderr, params.Title)
	if !output.IsTerminal(stderr) {
		return params.Run(flow.HookSink{Output: stderr})
	}

	view := output.NewHookView(output.HookViewParams{W: stderr, LogPath: params.LogPath})
	defer view.Close()
	return params.Run(flow.HookSink{Output: view, OnHook: view.OnHook})
}

func (p CLIPresenter) Notice(notice flow.Notice) {
	if notice.Kind == flow.NoticeWarning {
		output.Frame(p.Cmd.ErrOrStderr(), func() {
			output.Warning(p.Cmd.ErrOrStderr(), notice.Text)
		})
		return
	}
	output.Frame(p.Cmd.OutOrStdout(), func() {
		output.Message(p.Cmd.OutOrStdout(), notice.Text)
	})
}

func (p CLIPresenter) Status(notice flow.Notice) {
	if len(notice.Lines) > 0 {
		output.Callout(p.Cmd.ErrOrStderr(), notice.Text, notice.Lines)
		return
	}
	switch notice.Kind {
	case flow.NoticeWarning:
		output.Warning(p.Cmd.ErrOrStderr(), notice.Text)
	default:
		output.Success(p.Cmd.ErrOrStderr(), notice.Text)
	}
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
