package dashboard

import (
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/styles"
)

type detailMsg struct {
	branch string
	detail domain.WorktreeDetail
}

type detailTickMsg struct{ branch string }

func (m Model) selectedBranch() string {
	status, ok := m.selected()
	if !ok {
		return ""
	}
	return status.Branch
}

// detailSince stays zero until the load actually starts, so the debounce wait is
// never counted against the grace delay.
func (m Model) triggerDetailReload(before string) (Model, tea.Cmd) {
	branch := m.selectedBranch()
	if branch == before {
		return m, nil
	}
	// The logs panel speaks of one worktree's job: moving off it closes it.
	m = m.closePanelLogs()
	if branch == "" {
		return m, nil
	}
	m.detailLoading, m.detailSince = branch, time.Time{}
	return m, m.scheduleDetail(branch)
}

func (m Model) scheduleDetail(branch string) tea.Cmd {
	return tea.Tick(domain.DashboardDetailDebounce, func(time.Time) tea.Msg {
		return detailTickMsg{branch: branch}
	})
}

// Dropping a tick whose branch is no longer selected is what makes this a
// debounce rather than a delay: walking the list fires one load, not one per row.
func (m Model) fireDetailTick(msg detailTickMsg) (Model, tea.Cmd) {
	if msg.branch != m.selectedBranch() {
		return m, nil
	}
	m.detailSince = time.Now()
	return m, tea.Batch(m.loadDetailCmd(msg.branch), m.spinner.Tick)
}

// An explicit refresh and a post-operation invalidation skip the debounce, and
// leave the cache in place: old data stays on screen rather than blanking while
// this runs.
func (m Model) reloadDetailCmd() (Model, tea.Cmd) {
	branch := m.selectedBranch()
	if branch == "" {
		return m, nil
	}
	m.detailLoading, m.detailSince = branch, time.Now()
	return m, tea.Batch(m.loadDetailCmd(branch), m.spinner.Tick)
}

// A branch that is not on screen is left alone: its stale entry costs nothing
// until it is selected again, which reloads it anyway.
func (m Model) invalidateDetail(branch string) (Model, tea.Cmd) {
	if branch == "" || branch != m.selectedBranch() {
		return m, nil
	}
	return m.reloadDetailCmd()
}

// A superseded load landing late must not clear the newer one's marker.
func (m Model) applyDetail(msg detailMsg) Model {
	m.details[msg.branch] = msg.detail
	if m.detailLoading == msg.branch {
		m.detailLoading = ""
	}
	return m
}

// loadDetailCmd runs worktree.Detail off the UI goroutine: it shells out to
// git, and calling it inside Update would freeze the whole program.
func (m Model) loadDetailCmd(branch string) tea.Cmd {
	params := m.detailParams(branch)
	return func() tea.Msg {
		return detailMsg{branch: branch, detail: worktree.Detail(params)}
	}
}

func (m Model) detailParams(branch string) worktree.DetailParams {
	parent := m.parents[branch]
	return worktree.DetailParams{
		ProjectDir: m.params.ProjectDir,
		StateDir:   m.params.StateDir,
		Config:     m.params.Config,
		Status:     m.statusFor(branch),
		Parent:     parent,
		ParentPath: m.statusFor(parent).Path,
		Children:   m.childrenOf(branch),
		PRs:        m.prs,
		Commits:    domain.DashboardDetailCommits,
	}
}

// The empty-branch guard is load-bearing: a detached worktree can carry an empty
// Branch, and a parentless branch would otherwise match it and borrow its Path as
// ParentPath — silently wrong .env drift counters.
func (m Model) statusFor(branch string) domain.WorktreeStatus {
	if branch == "" {
		return domain.WorktreeStatus{}
	}
	for _, status := range m.statuses {
		if status.Branch == branch {
			return status
		}
	}
	return domain.WorktreeStatus{}
}

// Sorted because a map range would permute this line between two loads of the
// same branch, and the panel must not move under the eye.
func (m Model) childrenOf(branch string) []string {
	children := make([]string, 0, len(m.parents))
	for child, parent := range m.parents {
		if parent == branch {
			children = append(children, child)
		}
	}
	sort.Strings(children)
	return children
}

// Below the grace delay nothing is said: showing then hiding a marker in the
// time a local git call takes is flash dressed up as feedback. A zero
// detailSince means still debouncing, which is not yet worth mentioning either.
func (m Model) detailIsStale() bool {
	return m.detailLoading != "" &&
		m.detailLoading == m.selectedBranch() &&
		!m.detailSince.IsZero() &&
		time.Since(m.detailSince) > domain.DashboardSpinnerGrace
}

// The fresh state never signals; only stale-data-on-screen does.
func (m Model) detailFreshnessMarker() string {
	if !m.detailIsStale() {
		return ""
	}
	return m.spinner.View() + " " + styles.Muted.Render(domain.DashboardRefreshing)
}
