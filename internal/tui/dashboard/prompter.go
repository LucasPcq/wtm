package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

// flowMsg wraps what a running flow sends the dashboard. Everything the flow
// goroutine produces travels this way: it never touches the model itself.
type flowMsg struct{ inner tea.Msg }

// promptMsg hands a whole session to the dashboard and blocks the flow on reply
// until the modal has been through it.
type promptMsg struct {
	title   string
	shape   modalShape
	session flow.Session
	reply   chan<- promptReply
}

type opDoneMsg struct {
	id  int
	err error
}

// opTargetMsg names the worktree a run holds, as soon as its answers say which
// one it is.
type opTargetMsg struct {
	id      int
	targets []string
}

// opStageMsg carries a run's latest Stage/HookPhase message, the way the list
// shows what a locked row is doing instead of its state pill. target names the
// worktree it came from, empty for a run that speaks of none.
type opStageMsg struct {
	id     int
	target string
	stage  string
}

type createdMsg struct{ branch string }

type cleanedMsg struct{ branch string }

// reparentedMsg says the parent metadata changed, so the rows showing "from <x>"
// have to be re-read.
type reparentedMsg struct{}

// prunedMsg says several worktrees are gone at once, so the whole list has to be
// re-read rather than one row dropped.
type prunedMsg struct{}

// syncedMsg says a cascade rewrote branches and may have moved the base, so
// every row's state against its parent and its remote is stale.
type syncedMsg struct{}

type fastForwardedMsg struct{}

// listenCmd delivers the next message a flow goroutine posted. Update re-arms it
// on every flowMsg, so there is exactly one reader on the channel at all times.
func listenCmd(msgs <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg { return flowMsg{inner: <-msgs} }
}

// prompter is the dashboard half of flow.Prompter: it turns a session into a
// modal and waits, on the flow's goroutine, for what the user answered.
type prompter struct {
	send  func(tea.Msg)
	title string
	shape modalShape
	// opID and targetKey let the answers name the worktree the run holds, without
	// the dashboard having to know what the flow asks.
	opID      int
	targetKey string
}

func (p prompter) Interactive() bool { return true }

func (p prompter) Ask(session flow.Session) (flow.Answers, error) {
	reply := make(chan promptReply, 1)
	p.send(promptMsg{title: p.title, shape: p.shape, session: session, reply: reply})
	answered := <-reply
	if answered.err == nil && p.targetKey != "" {
		// Values, never Value: a cumulative step answers with a set, and reading
		// it as a single value would silently release the lock on every worktree
		// the run is about to touch.
		p.send(opTargetMsg{id: p.opID, targets: answered.answers.Values(p.targetKey)})
	}
	return answered.answers, answered.err
}

// Confirm is a standalone decision, asked as a one-question form: a flow reaches
// it after an execution, when there is no session left to join.
func (p prompter) Confirm(params flow.ConfirmParams) (bool, error) {
	answers, err := prompter{send: p.send, title: params.Title, shape: modalForm}.Ask(confirmSession(params))
	if err != nil {
		return false, err
	}
	return answers.Value(keyConfirm) == confirmYes, nil
}

func confirmSession(params flow.ConfirmParams) flow.Session {
	return flow.Session{Steps: []flow.Step{{
		Kind:        flow.StepRecap,
		Key:         keyConfirm,
		Title:       params.Title,
		Description: flow.ConfirmDescription(params),
		Options:     confirmOptions(params),
	}}}
}

// confirmOptions names both outcomes when the caller named them: closing the
// modal is a way out, not an answer, so a two-outcome decision has to offer both.
// It leads with the outcome DefaultYes names, the same rule flowui.confirmItems
// applies, so the two surfaces never disagree on which side a caller highlighted.
func confirmOptions(params flow.ConfirmParams) []flow.Option {
	if params.YesLabel == "" {
		return []flow.Option{{Label: domain.DashboardConfirmLabel, Value: confirmYes}}
	}
	yes := flow.Option{Label: params.YesLabel, Value: confirmYes}
	no := flow.Option{Label: params.NoLabel, Value: confirmNo}
	if params.DefaultYes {
		return []flow.Option{yes, {Separator: true}, no}
	}
	return []flow.Option{no, {Separator: true}, yes}
}

const (
	keyConfirm = "dashboard.confirm"
	confirmYes = "yes"
	confirmNo  = "no"
)
