package dashboard

import (
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	cleanflow "github.com/LucasPcq/wtm/internal/flow/clean"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	ffflow "github.com/LucasPcq/wtm/internal/flow/fastforward"
	pruneflow "github.com/LucasPcq/wtm/internal/flow/prune"
	reparentflow "github.com/LucasPcq/wtm/internal/flow/reparent"
	syncflow "github.com/LucasPcq/wtm/internal/flow/sync"
	"github.com/LucasPcq/wtm/internal/rules"
)

func (m Model) flowContext() flow.Context {
	return flow.Context{
		ProjectDir: m.params.ProjectDir,
		StateDir:   m.params.StateDir,
		Config:     m.params.Config,
	}
}

// sender hands a flow goroutine the one way it may reach the model: a message on
// the dashboard's channel, delivered by listenCmd on the UI goroutine.
func (m Model) sender() func(tea.Msg) {
	msgs := m.msgs
	return func(msg tea.Msg) { msgs <- msg }
}

func (m Model) startCreate() (Model, tea.Cmd) {
	if reason, refused := m.busyReason(""); refused {
		return m.refuse(reason), nil
	}
	// The branch is only known once the wizard is answered; the prompter names it
	// then, and the run holds it from there on.
	declared := createflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared})
	send := m.sender()

	params := createflow.Params{
		Context: m.flowContext(),
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardCreateTitle,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: createPresenter{presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := createflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startClean hands the removal to the same flow the CLI runs. The dashboard
// never presets Force: lifting a refusal is an answer the user gives in the
// modal, one refusal at a time.
func (m Model) startClean(branch string) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(branch); refused {
		return m.refuse(reason), nil
	}
	declared := cleanflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: branch})
	send := m.sender()

	params := cleanflow.Params{
		Context: m.flowContext(),
		Request: cleanflow.Request{
			Branch:     branch,
			BaseBranch: m.baseBranch(),
		},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardDeleteTitle,
			shape:     modalForm,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: cleanPresenter{presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := cleanflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startReparent changes the parent of the one worktree the menu was opened from.
// The flow's own worktree step is preset by the request, so the modal only asks
// what is left: the new parent, then the recap.
func (m Model) startReparent(branch string) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(branch); refused {
		return m.refuse(reason), nil
	}
	m, id := m.beginOp(beginParams{Operation: reparentflow.Operation(), Target: branch})
	send := m.sender()

	params := reparentflow.Params{
		Context: m.flowContext(),
		Request: reparentflow.Request{Branches: []string{branch}},
		Prompter: prompter{
			send:  send,
			title: domain.DashboardReparentTitle,
			shape: modalStepper,
			opID:  id,
		},
		Presenter: reparentPresenter{presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := reparentflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startBatchReparent runs the same flow with nothing preset, so it asks which
// worktrees to move before it asks where to. It blocks the surface for its whole
// run, which is why it needs no per-worktree lock: nothing else can start.
func (m Model) startBatchReparent() (Model, tea.Cmd) {
	if reason, refused := m.busyReason(""); refused {
		return m.refuse(reason), nil
	}
	m, id := m.beginOp(beginParams{Operation: reparentflow.Operation()})
	send := m.sender()

	params := reparentflow.Params{
		Context: m.flowContext(),
		Prompter: prompter{
			send:  send,
			title: domain.DashboardReparentBatchTitle,
			shape: modalStepper,
			opID:  id,
		},
		Presenter: reparentPresenter{presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := reparentflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startPrune removes every finished worktree in one run. Like the batch
// reparent it holds the whole surface, so it needs no per-worktree lock. It runs
// the broad default — merged, closed and gone — because there is no flag at the
// click: the picker tags every candidate with what made it prunable, which is
// where the user narrows. --dry-run has no equivalent either; the recap already
// lists what goes, and closing the modal removes nothing.
func (m Model) startPrune() (Model, tea.Cmd) {
	if reason, refused := m.busyReason(""); refused {
		return m.refuse(reason), nil
	}
	m, id := m.beginOp(beginParams{Operation: pruneflow.Operation()})
	send := m.sender()

	params := pruneflow.Params{
		Context: m.flowContext(),
		Request: pruneflow.Request{
			Merged:     true,
			Closed:     true,
			Gone:       true,
			BaseBranch: m.baseBranch(),
		},
		Prompter: prompter{
			send:  send,
			title: domain.DashboardPruneTitle,
			shape: modalStepper,
			opID:  id,
		},
		Presenter: prunePresenter{presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := pruneflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// startSync rebases the row and the chain it hangs off: replaying a worktree onto
// a parent nobody refreshed is the stale-parent problem the run otherwise has to
// ask about, so the gesture offers the whole ancestry, base included. Descendants
// are left out — they are their own gesture, made from their own row. Only the
// pre-check changes: the selection stays the user's, and nothing is deduced from
// it inside the run.
func (m Model) startSync(branch string) (Model, tea.Cmd) {
	return m.runSync(runSyncParams{
		Title:    domain.DashboardSyncRowTitle,
		Row:      branch,
		Precheck: m.ancestryOf(branch),
	})
}

// startSyncAll offers every worktree, base included, and leaves the ones a cascade
// would skip unchecked: they stay listed, with the tag saying why, one keystroke
// from being included.
func (m Model) startSyncAll() (Model, tea.Cmd) {
	return m.runSync(runSyncParams{
		Title:    domain.DashboardSyncTitle,
		Precheck: m.syncReadyBranches(),
	})
}

// startFastForward advances the row's own branch to its origin counterpart, and
// nothing else: the parent it hangs off and the rebase onto it are their own
// gestures. The branch is preset, so the modal only asks the recap. The base row
// reaches the same run — it hangs off nothing, so catching up with its remote is
// all it can do.
func (m Model) startFastForward(branch string) (Model, tea.Cmd) {
	return m.runFastForward(runFastForwardParams{
		Title:    domain.DashboardFastForwardTitle,
		Row:      branch,
		Branches: []string{branch},
		Shape:    modalForm,
	})
}

// startFastForwardAll presets nothing, so the run asks which worktrees first. It
// arrives with the ones the badges call behind already checked; the rest stay
// listed, tagged with why, one keystroke from being included. Those tags are
// cached remote-tracking refs, so the recap — which fetches — is what settles it.
func (m Model) startFastForwardAll() (Model, tea.Cmd) {
	return m.runFastForward(runFastForwardParams{
		Title:    domain.DashboardFastForwardAllTitle,
		Precheck: rules.FastForwardReadyBranches(m.statuses),
		Shape:    modalStepper,
	})
}

type runFastForwardParams struct {
	Title string
	// Row is the worktree the gesture was made on, when it was made on one:
	// nothing may advance a branch another run is holding.
	Row string
	// Branches fixes the selection; Precheck only says what arrives checked.
	Branches []string
	Precheck []string
	Shape    modalShape
}

func (m Model) runFastForward(params runFastForwardParams) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(params.Row); refused {
		return m.refuse(reason), nil
	}
	m, id := m.beginOp(beginParams{Operation: ffflow.Operation(), Target: params.Row})
	send := m.sender()

	flowParams := ffflow.Params{
		Context: m.flowContext(),
		Request: ffflow.Request{
			Branches: params.Branches,
			Precheck: params.Precheck,
		},
		Prompter: prompter{
			send:  send,
			title: params.Title,
			shape: params.Shape,
			opID:  id,
		},
		Presenter: ffPresenter{presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := ffflow.Run(flowParams)
		return opDoneMsg{id: id, err: err}
	})
}

type runSyncParams struct {
	Title string
	// Row is the worktree the gesture was made on, when it was made on one:
	// nothing may rebase a worktree another run is holding.
	Row string
	// Branches fixes the selection; Precheck only says what arrives checked.
	Branches []string
	Precheck []string
}

func (m Model) runSync(params runSyncParams) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(params.Row); refused {
		return m.refuse(reason), nil
	}
	m, id := m.beginOp(beginParams{Operation: syncflow.Operation()})
	send := m.sender()

	flowParams := syncflow.Params{
		Context: m.flowContext(),
		Request: syncflow.Request{
			Branches:   params.Branches,
			Precheck:   params.Precheck,
			BaseBranch: m.baseBranch(),
		},
		Prompter: prompter{
			send:  send,
			title: params.Title,
			shape: modalStepper,
			opID:  id,
		},
		Presenter: syncPresenter{presenter{send: send}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := syncflow.Run(flowParams)
		return opDoneMsg{id: id, err: err}
	})
}

func (m Model) baseBranch() string { return m.params.Config.Project.Worktrees.BaseBranch }

func (m Model) ancestryOf(branch string) []string {
	return rules.SyncAncestry(rules.SyncAncestryParams{Nodes: m.worktreeNodes(), Leaf: branch})
}

func (m Model) syncReadyBranches() []string { return rules.SyncReadyBranches(m.statuses) }

// worktreeNodes is the forest the sync rules read, built from the two things the
// model already holds: a re-read from disk on a keystroke would buy nothing.
func (m Model) worktreeNodes() []domain.WorktreeNode {
	return rules.WorktreeNodes(rules.WorktreeNodesParams{Statuses: m.statuses, Parents: m.parents})
}

// branchFor names a worktree the list would recognise. A run names the worktree
// an event came from by its branch, and falls back to its path for one git
// cannot name; empty stays empty — that is a run speaking of no worktree at all.
func (m Model) branchFor(target string) string {
	if target == "" {
		return ""
	}
	return rules.BranchForPath(rules.BranchForPathParams{Path: target, Statuses: m.statuses})
}

// busyReason states why nothing may act on a worktree right now: a run already
// holds it, or one holds the whole dashboard. This is where the mode a flow
// declares is enforced — once, rather than at every action site.
func (m Model) busyReason(target string) (string, bool) {
	if op, ok := m.ops.holding(target); ok {
		return fmt.Sprintf(domain.DashboardBusyFmt, target, op.kind), true
	}
	if op, ok := m.ops.blocking(); ok {
		return fmt.Sprintf(domain.DashboardBlockedByFmt, op.kind), true
	}
	return "", false
}

// busyCaption is what a menu entry says under itself while something holds its
// worktree: the same fact as busyReason, worded to sit under a label rather than
// to stand as a line in the output panel.
func (m Model) busyCaption(target string) (string, bool) {
	if op, ok := m.ops.holding(target); ok {
		return fmt.Sprintf(domain.DashboardBusyCaptionFmt, op.kind), true
	}
	if op, ok := m.ops.blocking(); ok {
		return fmt.Sprintf(domain.DashboardBusyCaptionFmt, op.kind), true
	}
	return "", false
}

// refuse states the refusal where the user is already looking for the run that
// caused it.
func (m Model) refuse(text string) Model {
	m.outputExpanded = true
	return m.appendOutput(OutputLineMsg{Text: text}).reflow()
}

type beginParams struct {
	Operation flow.Operation
	// Target is the worktree the run holds, when the surface already knows it.
	Target string
}

func targetsOf(target string) []string {
	if target == "" {
		return nil
	}
	return []string{target}
}

// firstTarget names the worktree an operation is reported against. A run over
// several is still one line of output, and the first is the one the surface
// selected.
func (o operation) firstTarget() string {
	if len(o.targets) == 0 {
		return ""
	}
	return o.targets[0]
}

// beginOp records the run and opens the output panel: a run whose output is
// folded away is one the user cannot follow.
func (m Model) beginOp(params beginParams) (Model, int) {
	declared := params.Operation
	ops, id := m.ops.begin(operation{kind: declared.Kind, mode: declared.Mode, targets: targetsOf(params.Target)})
	m.ops = ops
	m.outputExpanded = true
	return m.reflow(), id
}

// finishOp also invalidates the finished operation's target: its detail, if
// currently on screen, just went stale under it and is reloaded — the cache is
// refreshed, never emptied.
func (m Model) finishOp(msg opDoneMsg) (Model, tea.Cmd) {
	op, _ := m.ops.byID(msg.id)
	m.ops = m.ops.end(msg.id)
	m, detailCmd := m.invalidateDetail(op.firstTarget())
	// A run that just started or stopped jobs changes what the badges and the
	// RUN section say, and waiting for the next poll to notice is what made a
	// finished run look like nothing had happened.
	detailCmd = tea.Batch(detailCmd, m.loadJobsCmd(true))
	// ErrAborted is a run that already reported its own failure — a cascade whose
	// steps each said what became of them. A second, redundant line under them
	// would name nothing the panel does not already hold.
	if msg.err == nil || errors.Is(msg.err, domain.ErrUserAborted) || errors.Is(msg.err, domain.ErrAborted) {
		return m, detailCmd
	}

	m = m.appendOutput(OutputLineMsg{
		Text: fmt.Sprintf(domain.DashboardFailedFmt, domain.DashboardOperationLabel, msg.err),
	})

	// The privileged removal prompts for a password on the terminal this surface
	// is holding, so it is never offered here — the way to it is named instead.
	if errors.Is(msg.err, domain.ErrWorktreeRemoveFailed) {
		return m.appendOutput(OutputLineMsg{Text: fmt.Sprintf(domain.DashboardPrivilegedHintFmt, op.firstTarget())}), detailCmd
	}
	return m, detailCmd
}

// applyFlow handles what a running flow posted. Nothing here mutates anything the
// flow goroutine can see: it only reads the message it was handed.
func (m Model) applyFlow(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case promptMsg:
		return m.openModal(msg)
	case handoffMsg:
		return m, handoffCmd(msg, m.sender())
	case OutputLineMsg:
		return m.appendOutput(msg), nil
	case opTargetMsg:
		// The run answers with paths — the daemon's half of a job's key — and the
		// list, the refusals and the detail all speak branches. The translation
		// happens here, where the paths enter, rather than at every reader.
		m.ops = m.ops.retarget(msg.id, rules.BranchesForPaths(rules.BranchesForPathsParams{
			Paths:    msg.targets,
			Statuses: m.statuses,
		}))
		return m, nil
	case opStageMsg:
		m.ops = m.ops.stage(stageParams{ID: msg.id, Target: m.branchFor(msg.target), Stage: msg.stage})
		return m, nil
	case createdMsg:
		m.selectBranch = msg.branch
		return m, m.reload()
	case cleanedMsg:
		return m, m.reload()
	case reparentedMsg:
		return m, m.reload()
	case prunedMsg:
		return m, m.reload()
	case syncedMsg:
		return m, m.reload()
	case fastForwardedMsg:
		return m, m.reload()
	}
	return m, nil
}

// openModal refuses a second question rather than stacking it: two modals would
// leave the user answering one flow while another waits behind it, unseen.
func (m Model) openModal(msg promptMsg) (Model, tea.Cmd) {
	if m.modal.open {
		return m, replyCmd(msg.reply, promptReply{err: domain.ErrUserAborted})
	}
	modal, cmd := newModal(modalParams{
		Title:   msg.title,
		Shape:   msg.shape,
		Session: msg.session,
		Reply:   msg.reply,
		Width:   m.width,
		Height:  m.height,
	})
	m.modal = modal
	return m, cmd
}

// reload re-reads what a finished run changed: the worktrees, and the forest
// when it has ever been built — a run creates, removes or reparents a node.
func (m Model) reload() tea.Cmd {
	return tea.Batch(m.loadWorktreesCmd(false), m.treeCmd())
}

// appendOutput splits an incoming entry on its newlines so every stored line
// is exactly one rendered row: outputBody's window (offset + OutputLines)
// counts slice entries, and an entry carrying embedded newlines — a hook or
// git failure message stored verbatim — would otherwise occupy several rows
// under the guise of one, growing the panel past its budget. A bare \r is
// dropped rather than kept: left in place it would move the terminal cursor
// to column 0 mid-row and corrupt whatever is drawn after it.
func (m Model) appendOutput(msg OutputLineMsg) Model {
	m.outputLines = append(append([]string(nil), m.outputLines...), splitOutputLines(msg.Text)...)
	m.outputOffset = max(len(m.outputLines)-m.layout().OutputLines, 0)
	return m
}

func splitOutputLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "")
	return strings.Split(text, "\n")
}
