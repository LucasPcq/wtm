package dashboard

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	downflow "github.com/LucasPcq/wtm/internal/flow/run/down"
	logsflow "github.com/LucasPcq/wtm/internal/flow/run/logs"
	startflow "github.com/LucasPcq/wtm/internal/flow/run/start"
	stopflow "github.com/LucasPcq/wtm/internal/flow/run/stop"
	upflow "github.com/LucasPcq/wtm/internal/flow/run/up"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/integration"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
)

// runRequest is what every run action needs and the dashboard has to fetch:
// run.toml, which the flows take as a business input rather than reading
// themselves.
func (m Model) runRequest(selected domain.WorktreeStatus) (domain.RunConfig, bool) {
	if selected.Path == "" {
		return domain.RunConfig{}, false
	}
	return m.loadRunConfig()
}

// loadRunConfig is the same, for a run the user aims at worktrees it has yet to
// pick: there is no row to read, only a project that has a run module or has not.
func (m Model) loadRunConfig() (domain.RunConfig, bool) {
	cfg, err := runconfig.Load(m.params.StateDir)
	if err != nil {
		return domain.RunConfig{}, false
	}
	if len(cfg.Jobs) == 0 {
		return domain.RunConfig{}, false
	}
	return cfg, true
}

// runWorktree is what a run started from a row is told to act on. A row already
// designates a worktree, so it goes in where the positional would: target
// presets it, and the worktree step is read back in the recap instead of being
// put to someone who has just answered it by clicking.
//
// The branch, never the path: the positional is resolved by name
// (worktree.Resolve matches branches, exactly then by substring), so a path
// matches nothing and refuses the run. A detached worktree carries no branch
// and names nothing — the flow then falls back to Cwd, which is this same row.
func runWorktree(selected domain.WorktreeStatus) string { return selected.Branch }

// runningWorktrees are the worktrees the daemon holds something up in, as git
// spells them. They are what a stop and a view arrive with ticked: those two
// gestures are about what is standing, where a start is about where you are.
func (m Model) runningWorktrees() []string { return rules.RunningWorktreeDirs(m.board) }

type runUpParams struct {
	Title string
	// Global marks the gesture made on no row, which is what makes the flow ask
	// which worktrees it acts on.
	Global bool
	// Row is the worktree the gesture was made on, when it was made on one:
	// nothing may start jobs in a worktree another run is holding.
	Row domain.WorktreeStatus
	// Worktrees fixes the selection; Precheck only says what arrives ticked.
	Worktrees []string
	Precheck  []string
}

// startRunUp brings a worktree's default profile up, detached: the surface is
// given back and the progress goes to the output panel and to the held row's
// stage. Starting three worktrees in a row is the case this serves, and each
// one used to cost an open and an exit of the run view. Watching is what the
// LOGS tab is for.
func (m Model) startRunUp(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	return m.runUp(runUpParams{
		Title:     domain.DashboardMenuRunUp,
		Row:       selected,
		Worktrees: []string{runWorktree(selected)},
	})
}

// startRunUpAll fixes no worktree, so the flow asks which ones — the picker the
// modal already renders for `sync`, with the "N running" badge per worktree that
// the list does not show. It arrives with the worktree the shell is in ticked,
// which the step does on its own: a start is about where you are.
func (m Model) startRunUpAll() (Model, tea.Cmd) {
	return m.runUp(runUpParams{Title: domain.DashboardRunUpAllTitle, Global: true})
}

func (m Model) runUp(params runUpParams) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(params.Row.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runConfigFor(runConfigParams{Row: params.Row, Global: params.Global})
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := upflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: params.Row.Branch})
	send := m.sender()

	flowParams := upflow.Params{
		Context: m.flowContext(),
		Request: upflow.Request{
			Worktrees: params.Worktrees,
			Precheck:  params.Precheck,
			Cwd:       m.cwdFor(params.Row),
			Config:    cfg,
		},
		Prompter: prompter{
			send:      send,
			title:     params.Title,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: runPresenter{presenter: presenter{send: send, id: id}, Watcher: detachedWatcher{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := upflow.Run(flowParams)
		return opDoneMsg{id: id, err: err}
	})
}

// startRunJob starts one job the user names, detached like startRunUp: a job is
// a request of its own, and `target.JobStep` has no safe default — which is why
// its picker opens here rather than a job being guessed.
func (m Model) startRunJob(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := startflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: selected.Branch})
	send := m.sender()

	params := startflow.Params{
		Context: m.flowContext(),
		Request: startflow.Request{Worktree: runWorktree(selected), Cwd: selected.Path, Config: cfg},
		Prompter: prompter{
			send:      send,
			title:     domain.DashboardMenuRunStart,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: runPresenter{presenter: presenter{send: send, id: id}, Watcher: detachedWatcher{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := startflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// stopRunJob stops one job. Nothing is attached to, so no view opens: what
// became of the job is reported in the output panel. The job is the one the
// surface designates — the Services tab has a cursor on one — and only a surface
// designating none opens the picker.
func (m Model) stopRunJob(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(selected.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runRequest(selected)
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := stopflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: selected.Branch})
	send := m.sender()
	title := domain.DashboardMenuRunStop
	job := m.designatedJob()
	if job != "" {
		title = domain.DashboardMenuRunStopThis
	}

	params := stopflow.Params{
		Context: m.flowContext(),
		Request: stopflow.Request{
			Worktrees: []string{runWorktree(selected)},
			Cwd:       selected.Path,
			Job:       job,
			Config:    cfg,
		},
		Prompter: prompter{
			send:      send,
			title:     title,
			shape:     modalStepper,
			opID:      id,
			targetKey: declared.TargetKey,
		},
		Presenter: stopPresenter{presenter: presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := stopflow.Run(params)
		return opDoneMsg{id: id, err: err}
	})
}

// designatedJob is the job the surface already points at, empty where it points
// at none. Only the Services tab does: its cursor walks job rows, and asking
// again which job is the question a designated subject exists to not ask.
func (m Model) designatedJob() string {
	if m.tab != tabServices {
		return ""
	}
	row, ok := m.selectedService()
	if !ok {
		return ""
	}
	return row.Job.Key
}

type runDownParams struct {
	Title string
	// Row is the worktree the gesture was made on, when it was made on one.
	Row domain.WorktreeStatus
	// Worktrees fixes the selection; Precheck only says what arrives ticked.
	Worktrees []string
	Precheck  []string
	// Ask installs the modal, for a run that has no row to act on. From a row
	// there is nothing to ask: stopping everything there is the safe default.
	Ask bool
}

// startRunDown stops what the worktree has running. It asks nothing — stopping
// everything here is the command's safe default — so the modal never opens.
func (m Model) startRunDown(selected domain.WorktreeStatus) (Model, tea.Cmd) {
	return m.runDown(runDownParams{
		Row:       selected,
		Worktrees: []string{runWorktree(selected)},
	})
}

// startRunDownAll designates no worktree, so it has to ask which ones — the one
// thing the row gesture never does. It arrives with the worktrees that have
// something up ticked: a stop is about what is standing.
func (m Model) startRunDownAll() (Model, tea.Cmd) {
	return m.runDown(runDownParams{
		Title:    domain.DashboardRunDownAllTitle,
		Precheck: m.runningWorktrees(),
		Ask:      true,
	})
}

func (m Model) runDown(params runDownParams) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(params.Row.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runConfigFor(runConfigParams{Row: params.Row, Global: params.Ask})
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	declared := downflow.Operation()
	m, id := m.beginOp(beginParams{Operation: declared, Target: params.Row.Branch})
	send := m.sender()

	flowParams := downflow.Params{
		Context: m.flowContext(),
		Request: downflow.Request{
			Worktrees: params.Worktrees,
			Precheck:  params.Precheck,
			Cwd:       m.cwdFor(params.Row),
			Config:    cfg,
		},
		Prompter: m.runPrompter(runPrompterParams{
			Ask:       params.Ask,
			Title:     params.Title,
			OpID:      id,
			TargetKey: declared.TargetKey,
			Send:      send,
		}),
		Presenter: downPresenter{presenter: presenter{send: send, id: id}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := downflow.Run(flowParams)
		return opDoneMsg{id: id, err: err}
	})
}

type runPrompterParams struct {
	Ask       bool
	Title     string
	OpID      int
	TargetKey string
	Send      func(tea.Msg)
}

// runPrompter picks which of the two seams a run installs: the modal for a run
// whose subject is still to be chosen, and the unattended one for a run a row
// has already answered for.
func (m Model) runPrompter(params runPrompterParams) flow.Prompter {
	if !params.Ask {
		return flow.Unattended{}
	}
	return prompter{
		send:      params.Send,
		title:     params.Title,
		shape:     modalStepper,
		opID:      params.OpID,
		targetKey: params.TargetKey,
	}
}

type runConfigParams struct {
	Row domain.WorktreeStatus
	// Global marks a gesture made on no row: there is no worktree to read, only
	// a project that has a run module or has not. A row gesture keeps refusing a
	// row that designates no worktree.
	Global bool
}

// runConfigFor reads run.toml for a gesture made on a row, or made on none.
func (m Model) runConfigFor(params runConfigParams) (domain.RunConfig, bool) {
	if params.Global {
		return m.loadRunConfig()
	}
	return m.runRequest(params.Row)
}

// cwdFor is the worktree step's safe default: the row the gesture was made on,
// else the directory the shell was in when it launched `wtm ui` — which is what
// the picker marks as current.
func (m Model) cwdFor(row domain.WorktreeStatus) string {
	if row.Path != "" {
		return row.Path
	}
	return m.params.Cwd
}

// watchLogs hands the terminal to the run view, on the job the logs view is
// showing. It is the one run action that takes the terminal on purpose: the
// full session is what it is for.
func (m Model) watchLogs() (Model, tea.Cmd) {
	selected := m.statusFor(m.logsBranch)
	request := m.watchLogsRequest()
	return m.runLogs(runLogsParams{Row: selected, Request: request})
}

// startRunLogsAll opens the same view over worktrees the user picks, which is
// what R10 built the two-level list for: one group header per worktree, its jobs
// indented under it. It arrives with the worktrees that have something up ticked.
func (m Model) startRunLogsAll() (Model, tea.Cmd) {
	return m.runLogs(runLogsParams{
		Title:   domain.DashboardMenuRunLogsAll,
		Request: logsflow.Request{Precheck: m.runningWorktrees(), Cwd: m.params.Cwd},
		Ask:     true,
	})
}

type runLogsParams struct {
	Title string
	// Row is the worktree the gesture was made on, when it was made on one.
	Row     domain.WorktreeStatus
	Request logsflow.Request
	// Ask installs the modal, for a view whose worktrees are still to be picked.
	Ask bool
}

func (m Model) runLogs(params runLogsParams) (Model, tea.Cmd) {
	if reason, refused := m.busyReason(params.Row.Branch); refused {
		return m.refuse(reason), nil
	}
	cfg, ok := m.runConfigFor(runConfigParams{Row: params.Row, Global: params.Ask})
	if !ok {
		return m.refuse(domain.DashboardRunNotConfigured), nil
	}

	request := params.Request
	request.Config = cfg

	declared := flow.Operation{Kind: domain.OpKindRunLogs, Mode: flow.ModeBlocking}
	m, id := m.beginOp(beginParams{Operation: declared})
	send := m.sender()

	flowParams := logsflow.Params{
		Context: m.flowContext(),
		Request: request,
		Prompter: m.runPrompter(runPrompterParams{
			Ask:   params.Ask,
			Title: params.Title,
			OpID:  id,
			Send:  send,
		}),
		Presenter: logsPresenter{presenter: presenter{send: send, id: id}, watcher: watcher{send: send}},
	}

	return m, tea.Batch(m.spinner.Tick, func() tea.Msg {
		_, err := logsflow.Run(flowParams)
		return opDoneMsg{id: id, err: err}
	})
}

// clickRunRow answers a click on a RUN row: the job's address when it publishes
// one. A job that publishes none leads to its logs instead.
func (m Model) clickRunRow(msg tea.MouseMsg) (tea.Model, tea.Cmd, bool) {
	for _, job := range m.runConfig.Jobs {
		address := m.addressFor(job.Name)
		// A runner's children are rows of their own, each carrying its own url —
		// which is the whole point of unfolding them. Answered before the runner
		// so a click inside the fold does not toggle it shut.
		for _, entry := range address.Held {
			if m.inZone(runURLZone(rules.HeldRowKey(job.Name, entry.Job)), msg) {
				model, cmd := m.openJobURL(entry.URL)
				return model, cmd, true
			}
		}
		// The address cell first: clicking what you read opens what you read,
		// and the rest of the row leads to the job's logs.
		if m.inZone(runURLZone(job.Name), msg) {
			model, cmd := m.openJobURL(address.URL)
			return model, cmd, true
		}
		if m.inZone(runFoldZone(job.Name), msg) {
			return m.toggleRunFold(job.Name), nil, true
		}
		if !m.inZone(runRowZone(job.Name), msg) {
			continue
		}
		model, cmd := m.openLogsTabOn(job.Name)
		return model, cmd, true
	}
	return m, nil, false
}

// toggleRunFold opens or closes a runner's addresses. Keyed by job name across
// every worktree: unfolding `dev` in one and finding it folded in the next is
// the opposite of what comparing two worktrees needs.
func (m Model) toggleRunFold(job string) Model {
	expanded := make(map[string]bool, len(m.runExpanded)+1)
	for name, open := range m.runExpanded {
		expanded[name] = open
	}
	expanded[job] = !expanded[job]
	m.runExpanded = expanded
	return m
}

// addressFor is where the selected worktree's job answers.
func (m Model) addressFor(job string) domain.JobAddress {
	return m.addresses[m.selectedBranch()][job]
}

// Off the UI goroutine: the opener hands the url to the desktop, and calling it
// inside Update would freeze the program.
func (m Model) openJobURL(url string) (Model, tea.Cmd) {
	opener := m.urlOpener()
	if opener == nil || url == "" {
		return m, nil
	}
	return m, func() tea.Msg { return openURLMsg{err: opener(url)} }
}

func (m Model) urlOpener() func(string) error {
	if m.params.URLOpener != nil {
		return m.params.URLOpener
	}
	return integration.OpenURL
}

// openSelectedAddress opens the address of the job the surface designates: the
// one on the logs view's selection line, or the one under the Services cursor.
// The DETAIL panel designates none — it has no cursor — so it stays silent, the
// same way KeyOpenPR is silent where no PR is designated.
func (m Model) openSelectedAddress() (Model, tea.Cmd) {
	if m.logsOpen() {
		return m.openJobURL(m.logsAddress().URL)
	}
	if m.tab != tabServices {
		return m, nil
	}
	row, ok := m.selectedService()
	if !ok {
		return m, nil
	}
	return m.openJobURL(m.addresses[row.Branch][row.Job.Key].URL)
}
