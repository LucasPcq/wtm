package dashboard

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/integration"
	"github.com/LucasPcq/wtm/internal/tui/runview"
)

// handoffMsg asks the dashboard to give the terminal to the run view, and blocks
// the flow that asked until it is given back. It travels the same way a prompt
// does: a flow goroutine may only reach the model through a message.
type handoffMsg struct {
	params seam.SequenceParams
	reply  chan<- handoffReply
}

type handoffReply struct {
	outcomes runlogs.Outcomes
	err      error
}

// handoffDoneMsg carries what the run view concluded back onto the UI goroutine,
// once bubbletea has restored the terminal.
type handoffDoneMsg struct {
	reply    chan<- handoffReply
	outcomes runlogs.Outcomes
	recap    string
	err      error
}

// handoff is the tea.ExecCommand that runs the view. bubbletea releases the
// terminal, hands it the three streams, calls Run, then restores it — which is
// exactly the window a second full-screen program needs. Running the view in
// this process rather than re-executing the binary is what keeps its result
// typed: a child process could only have returned an exit code.
type handoff struct {
	params seam.SequenceParams
	// detached is where the run reports if the reader closes the view before it
	// ends. It posts to the model, which is safe from here: tea.Exec has the
	// event loop blocked, so the messages queue and are handled once the
	// terminal comes back.
	detached detachedRun

	in  io.Reader
	out io.Writer

	result runview.Result
}

func (h *handoff) SetStdin(r io.Reader)  { h.in = r }
func (h *handoff) SetStdout(w io.Writer) { h.out = w }
func (h *handoff) SetStderr(io.Writer)   {}

func (h *handoff) Run() error {
	result, err := runview.Run(runview.Params{
		Board:     h.params.Board,
		Job:       h.params.Job,
		Profile:   h.params.Profile,
		Worktrees: h.params.Worktrees,
		Warnings:  h.params.Warnings,
		Start:     h.params.Start,
		Open:      integration.OpenURL,
		Detach:    runview.Detach{Sink: h.detached},
		In:        h.in,
		Out:       h.out,
	})
	if err != nil {
		return err
	}
	h.result = result
	return nil
}

// handoffCmd gives the terminal away and takes it back. The mouse has to be
// asked for again: RestoreTerminal puts back the alternate screen, the bracketed
// paste and the focus reporting, but never the mouse tracking it turned off —
// and every one of this dashboard's click targets depends on it.
func handoffCmd(msg handoffMsg, send func(tea.Msg)) tea.Cmd {
	cmd := &handoff{params: msg.params, detached: detachedRun{send: send}}
	return tea.Exec(cmd, func(err error) tea.Msg {
		return handoffDoneMsg{
			reply:    msg.reply,
			outcomes: cmd.result.Outcomes,
			recap:    cmd.result.Recap,
			err:      err,
		}
	})
}

// watcher is the dashboard's half of seam.Watcher: it asks for the terminal and
// waits, on the flow's goroutine, for what the view concluded.
type watcher struct {
	send func(tea.Msg)
}

func (w watcher) Sequence(params seam.SequenceParams) (runlogs.Outcomes, error) {
	reply := make(chan handoffReply, 1)
	w.send(handoffMsg{params: params, reply: reply})
	answered := <-reply
	return answered.outcomes, answered.err
}

// finishHandoff unblocks the flow that gave the terminal away, and asks for the
// mouse back: bubbletea restores the alternate screen and the bracketed paste
// on its own, never the mouse tracking, and this surface is clicked on.
func (m Model) finishHandoff(msg handoffDoneMsg) (Model, tea.Cmd) {
	if msg.recap != "" {
		m = m.appendOutput(OutputLineMsg{Text: msg.recap})
	}
	msg.reply <- handoffReply{outcomes: msg.outcomes, err: msg.err}
	return m, tea.EnableMouseCellMotion
}
