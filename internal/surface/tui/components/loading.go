package components

import (
	"context"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// renderLoadingBox renders an animated spinner glyph + muted message inside the
// shared bordered status box. Used by both the standalone loader and the wizard's
// async status banner so the loading look stays identical.
func renderLoadingBox(spinnerView, message string) string {
	return styles.StatusBox.Render(spinnerView + " " + styles.Muted.Render(message))
}

// newMutedSpinner returns the project's standard loading spinner: a MiniDot
// braille animation in the muted style.
func newMutedSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.MiniDot
	sp.Style = styles.Muted
	return sp
}

// MutedSpinner is the project's standard spinner, for surfaces that animate
// their own wait rather than going through RunLoading.
func MutedSpinner() spinner.Model { return newMutedSpinner() }

type LoadingParams struct {
	Message string
	// Work is the blocking operation to run while the loader animates. Capture any
	// results via closure.
	Work func() error
	// Animate gates the visual loader: when false (e.g. JSON output), Work runs
	// directly with no box. The box is also skipped when stderr is not a terminal.
	Animate bool
}

type loadingDoneMsg struct{ err error }

// loadingModel shows a bordered, animated loading box while Work runs in the
// background, then quits. Its View returns "" once done so the box is cleared
// from the terminal, leaving the command's framed result as the only output.
type loadingModel struct {
	ctx        context.Context
	spinner    spinner.Model
	message    string
	work       tea.Cmd
	err        error
	done       bool
	cancelling bool
	abandoned  bool
	// shielded work cannot be left behind: a second Ctrl-C waits for it too.
	shielded bool
}

func (m loadingModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.work, AwaitInterrupt(m.ctx))
}

func (m loadingModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case loadingDoneMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case InterruptedMsg:
		m.cancelling = true
		return m, nil
	case tea.KeyMsg:
		if msg.String() != domain.KeyInterrupt {
			return m, nil
		}
		return m.interrupt()
	}
	return m, nil
}

// interrupt cancels the work and keeps waiting for it to unwind: quitting at
// once would leave a git or docker child running behind a prompt that came back.
func (m loadingModel) interrupt() (tea.Model, tea.Cmd) {
	if m.cancelling && m.shielded {
		return m, nil
	}
	if m.cancelling {
		m.abandoned = true
		m.done = true
		return m, tea.Quit
	}
	m.cancelling = true
	Interrupt(m.ctx)
	return m, nil
}

func (m loadingModel) View() string {
	if m.done {
		return ""
	}
	message := m.message
	if m.cancelling {
		message = domain.CancellingMessage
	}
	return "\n" + renderLoadingBox(m.spinner.View(), message) + "\n"
}

// RunLoading runs work while showing an animated bordered loading box on stderr,
// then clears the box and returns work's error. When params.Animate is false or
// stderr is not a terminal, work runs directly with no box, so piped and JSON
// output stay clean.
//
// The terminal is raw while the box is up, so Ctrl-C arrives as a key: the
// first one cancels ctx, the second stops waiting for the work to notice —
// unless the work is shielded, which is always waited for.
func RunLoading(ctx context.Context, params LoadingParams) error {
	if !params.Animate || !term.IsTerminal(int(os.Stderr.Fd())) {
		return params.Work()
	}

	m, stop := newLoadingModel(ctx, params)
	defer stop()

	// Bubbletea's own handler would end the program on the very SIGINT the
	// first Ctrl-C raises, before the work has unwound.
	final, err := tea.NewProgram(m, tea.WithOutput(os.Stderr), tea.WithoutSignalHandler()).Run()
	if err != nil {
		return ProgramError(err)
	}
	return loadingOutcome(final)
}

// newLoadingModel watches the context a shielded one was made from: that is
// where the interrupt it goes on through shows.
func newLoadingModel(ctx context.Context, params LoadingParams) (loadingModel, context.CancelFunc) {
	outer, shielded := ctx.Value(domain.ShieldedFrom{}).(context.Context)
	if !shielded {
		outer = ctx
	}
	watch, stop := context.WithCancel(outer)
	return loadingModel{
		ctx:      watch,
		shielded: shielded,
		spinner:  newMutedSpinner(),
		message:  params.Message,
		work: func() tea.Msg {
			return loadingDoneMsg{err: params.Work()}
		},
	}, stop
}

func loadingOutcome(final tea.Model) error {
	lm, ok := final.(loadingModel)
	if !ok {
		return nil
	}
	if lm.abandoned {
		return domain.ErrCancelled
	}
	return lm.err
}
