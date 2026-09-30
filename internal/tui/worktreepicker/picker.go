// Package worktreepicker renders a rich worktree selector with the same
// styling (breadcrumb, badges, spacing) as the interactive `list` wizard.
package worktreepicker

import (
	"fmt"
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/worktreerefresh"
)

// LoadingPRsText is the status-line label shown while PRs stream in.
const LoadingPRsText = "Loading pull requests…"

// GHBanner returns the status banner shown in the picker once the PR fetch
// completes, when the GitHub CLI is unavailable. An empty Title means gh is OK
// (no banner).
func GHBanner(conn domain.GHConnection) components.WizardBanner {
	switch conn {
	case domain.GHConnectionNotInstalled:
		return components.WizardBanner{
			Title: "GitHub CLI not found",
			Lines: []string{"Install it to see PRs linked to your worktrees:", "https://cli.github.com"},
		}
	case domain.GHConnectionNotAuthenticated:
		return components.WizardBanner{
			Title: "GitHub not connected",
			Lines: []string{"Connect to see PRs linked to your worktrees:", "run `gh auth login`"},
		}
	default:
		return components.WizardBanner{}
	}
}

// PRLoaderFunc fetches open PRs and the GitHub CLI connection status. Provided
// by the command layer so the TUI never imports services/commands directly.
type PRLoaderFunc func() ([]domain.PRInfo, domain.GHConnection)

// PRsLoadedMsg is emitted once the asynchronous PR fetch completes.
type PRsLoadedMsg struct {
	PRs  []domain.PRInfo
	Conn domain.GHConnection
}

// PRLoadCmd wraps a PRLoaderFunc into a bubbletea command that runs off the
// main loop and emits a PRsLoadedMsg when done. Returns nil when loader is nil.
func PRLoadCmd(loader PRLoaderFunc) tea.Cmd {
	if loader == nil {
		return nil
	}
	return func() tea.Msg {
		prs, conn := loader()
		return PRsLoadedMsg{PRs: prs, Conn: conn}
	}
}

// RunParams holds the inputs required to render the picker.
type RunParams struct {
	Statuses     []domain.WorktreeStatus
	ActiveBranch string
	PRs          []domain.PRInfo
	Services     []domain.JobInfo
	Title        string
	// PRLoader, when set, defers the PR fetch: the picker renders instantly with
	// a loading badge per row and refreshes badges when the fetch completes.
	// When set, PRs is ignored (it is loaded lazily instead).
	PRLoader PRLoaderFunc
	// ProjectDir/StateDir/Config back the "r" refresh key: pressing r re-fetches
	// origin and re-lists the worktrees so the divergence badges update in place.
	ProjectDir string
	StateDir   string
	Config     domain.Config
}

// Run shows the picker and returns the selected worktree.
// Returns domain.ErrUserAborted when the user presses Esc.
func Run(params RunParams) (domain.WorktreeStatus, error) {
	if params.Title == "" {
		params.Title = "Select a worktree"
	}

	// This picker is commonly invoked from `wtm resolve` through a shell
	// wrapper that captures stdout. Force lipgloss to detect color support
	// against stderr (the TTY) so selected rows keep their accent styling.
	styles.UseRendererOn(os.Stderr)

	loading := params.PRLoader != nil

	// holder backs the rows and is swapped in place when the user presses "r"
	// (re-fetch origin + re-list); lastPRs is the most recent async PR set. Both
	// the initial build and every in-place rebuild read through them so a refresh
	// keeps the streamed PR badges.
	holder := &params.Statuses
	lastPRs := params.PRs
	buildItems := func() []components.SelectItem {
		items := make([]components.SelectItem, 0, len(*holder))
		for i, s := range *holder {
			status := BuildStatus(s)
			items = append(items, components.SelectItem{
				Label: s.Branch,
				Value: strconv.Itoa(i),
				Badges: BuildTags(BuildTagsParams{
					Status:       s,
					PRs:          lastPRs,
					Services:     params.Services,
					ActiveBranch: params.ActiveBranch,
				}),
				Status: &status,
			})
		}
		return items
	}

	listParams := domain.ListParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Config: params.Config}

	wiz := components.NewWizardWithParams(components.WizardParams{
		Steps: []components.Step{
			{
				Name:  params.Title,
				Model: components.NewSelectList(components.NewSelectListParams{Title: params.Title, Items: buildItems()}),
				Build: func([]components.Step) any {
					return components.NewSelectList(components.NewSelectListParams{Title: params.Title, Items: buildItems()})
				},
				CanRefresh: true,
			},
		},
		InitCmd:     PRLoadCmd(params.PRLoader),
		Loading:     loading,
		LoadingText: LoadingPRsText,
		OnMsg: func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
			if cmd, handled := worktreerefresh.Handle(worktreerefresh.HandleParams{
				Wizard:     w,
				Msg:        msg,
				ListParams: listParams,
				Holder:     holder,
			}); handled {
				return cmd, true
			}
			loaded, ok := msg.(PRsLoadedMsg)
			if !ok {
				return nil, false
			}
			lastPRs = loaded.PRs
			badges := BadgesByValue(BadgesByValueParams{
				Statuses:     *holder,
				PRs:          loaded.PRs,
				Services:     params.Services,
				ActiveBranch: params.ActiveBranch,
			})
			w.UpdateStepModel(0, applyBadges(badges))
			w.SetLoading(false)
			w.SetBanner(GHBanner(loaded.Conn))
			return nil, true
		},
	})

	finalModel, err := tea.NewProgram(wiz, tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("worktree picker: %w", err)
	}

	final, ok := finalModel.(components.WizardModel)
	if !ok || final.Aborted() {
		return domain.WorktreeStatus{}, domain.ErrUserAborted
	}

	sl, ok := final.Steps()[0].Model.(components.SelectListModel)
	if !ok {
		return domain.WorktreeStatus{}, fmt.Errorf("unexpected model type")
	}

	idx, err := strconv.Atoi(sl.Value())
	if err != nil {
		return domain.WorktreeStatus{}, fmt.Errorf("parse worktree index: %w", err)
	}
	if idx < 0 || idx >= len(*holder) {
		return domain.WorktreeStatus{}, fmt.Errorf("worktree index out of range")
	}

	return (*holder)[idx], nil
}

// BuildTagsParams holds the inputs for BuildTags.
type BuildTagsParams struct {
	Status   domain.WorktreeStatus
	PRs      []domain.PRInfo
	Services []domain.JobInfo
	// ActiveBranch is the worktree the shell is currently inside. Empty means
	// no worktree in this set is marked — a picker invoked outside any worktree.
	ActiveBranch string
}

// BuildTags returns the left-column tags for a worktree row (parent, PR,
// services) as colored text, used by both `list` and the switch picker. While
// PRs are still loading, PRs is empty and no PR tag is shown — the picker
// surfaces a loading status line instead. The dirty/clean state is a separate
// right-aligned status pill (see BuildStatus).
func BuildTags(params BuildTagsParams) []components.Badge {
	s := params.Status
	var tags []components.Badge
	if s.IsParent {
		tags = append(tags, components.Badge{Text: "parent", Variant: components.BadgeAccent})
	}
	for _, pr := range params.PRs {
		if pr.Branch == s.Branch {
			tags = append(tags, components.Badge{Text: fmt.Sprintf("PR #%d", pr.Number), Variant: components.BadgeSuccess})
			break
		}
	}
	for _, svc := range params.Services {
		if svc.WorkDir == s.Path && rules.IsJobUp(svc.Status) {
			tags = append(tags, components.Badge{Text: "services", Variant: components.BadgeSuccess})
			break
		}
	}
	if s.CommitsAhead > 0 {
		tags = append(tags, components.Badge{
			Text:    fmt.Sprintf("%s %s%d", domain.BadgeTextBase, domain.BadgeGlyphAhead, s.CommitsAhead),
			Variant: components.BadgeNeutral,
		})
	}
	if badge, ok := components.DivergenceBadge(components.DivergenceBadgeParams{
		State:  s.OriginState,
		Ahead:  s.OriginAhead,
		Behind: s.OriginBehind,
		Label:  domain.BadgeTextOrigin,
	}); ok {
		tags = append(tags, badge)
	}
	if params.ActiveBranch != "" && s.Branch == params.ActiveBranch {
		tags = append(tags, components.Badge{Text: domain.WorktreeActiveTag, Variant: components.BadgeSuccess})
	}
	return tags
}

// BuildStatus returns the right-aligned status pill for a worktree row: a filled
// chip with a leading glyph so the state scans at a glance. A paused rebase takes
// precedence over the generic dirty flag (a mid-rebase tree is always dirty, but
// "rebasing" is the actionable signal). It is computed once at construction (the
// working tree state does not change while the picker is open) and is not
// refreshed when PRs stream in.
func BuildStatus(s domain.WorktreeStatus) components.Badge {
	if s.RebaseInProgress {
		return components.Badge{Text: "rebasing", Variant: components.BadgeWarning, Glyph: domain.BadgeGlyphDirty}
	}
	if s.IsDirty {
		return components.Badge{Text: "dirty", Variant: components.BadgeWarning, Glyph: domain.BadgeGlyphDirty}
	}
	return components.Badge{Text: "clean", Variant: components.BadgeSuccess, Glyph: domain.BadgeGlyphClean}
}

// BadgesByValueParams holds the inputs for BadgesByValue.
type BadgesByValueParams struct {
	Statuses     []domain.WorktreeStatus
	PRs          []domain.PRInfo
	Services     []domain.JobInfo
	ActiveBranch string
}

// BadgesByValue builds the left-column tag sets for a slice of worktree statuses
// keyed by the SelectItem.Value (the status index as a string), so a message
// handler can refresh the tag column by value once PRs arrive. The status pill
// is set once at construction and preserved across the refresh.
func BadgesByValue(params BadgesByValueParams) map[string][]components.Badge {
	out := make(map[string][]components.Badge, len(params.Statuses))
	for i, s := range params.Statuses {
		out[strconv.Itoa(i)] = BuildTags(BuildTagsParams{
			Status:       s,
			PRs:          params.PRs,
			Services:     params.Services,
			ActiveBranch: params.ActiveBranch,
		})
	}
	return out
}

// applyBadges returns a step-model mutator that refreshes a SelectList's badges.
func applyBadges(byValue map[string][]components.Badge) func(any) any {
	return func(model any) any {
		sl, ok := model.(components.SelectListModel)
		if !ok {
			return model
		}
		sl.SetBadges(byValue)
		return sl
	}
}
