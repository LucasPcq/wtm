package dashboard

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// menuAction is what a menu entry does.
type menuAction int

const (
	// menuNone is the zero value, so a heading or a rule never reads as the first
	// action declared here.
	menuNone menuAction = iota
	menuReparent
	menuDelete
	menuReparentBatch
	menuPrune
	menuSync
	menuSyncAll
	menuFastForward
	menuFastForwardAll
	menuRunUp
	menuRunStart
	menuRunDown
	menuRunStop
	menuRunLogs
	menuRunUpAll
	menuRunDownAll
	menuRunLogsAll
	menuDeleteBatch
)

// menuEntryKind separates what the menu offers from what it only says. A
// heading and a rule are read; neither takes the cursor, a mouse zone or a
// keypress.
type menuEntryKind int

const (
	menuEntryAction menuEntryKind = iota
	menuEntryHeading
	menuEntrySeparator
)

// menuKind is which menu is open: the one hanging off a worktree row, or the
// one hanging off the header's Actions button. They share their box, their keys
// and their mouse handling; only what they list differs.
type menuKind int

const (
	menuForWorktree menuKind = iota
	menuForGlobal
)

type menuItem struct {
	kind   menuEntryKind
	label  string
	action menuAction
	// danger marks an entry that destroys something, so it reads as one before it
	// is activated.
	danger bool
	// disabled states why the entry cannot be used right now, and shows it.
	disabled string
}

func (i menuItem) activatable() bool { return i.kind == menuEntryAction && i.disabled == "" }

// menuItems is what the open menu lists. An action that could never apply is not
// listed at all; one that cannot apply right now is listed with what is in its
// way, and is inert until that clears.
func (m Model) menuItems() []menuItem {
	if m.menuKind == menuForGlobal {
		return m.globalMenuItems()
	}
	return m.worktreeMenuItems()
}

func (m Model) worktreeMenuItems() []menuItem {
	selected, ok := m.selected()
	if !ok {
		return nil
	}

	items := m.worktreeActions(selected)
	// The Services tab does not speak of git, and it lists no worktree to
	// destroy: its blocks answer for what runs, so only the run block applies.
	// Its cursor is on a job, so the stop names that one rather than reopening a
	// picker over it.
	if m.tab == tabServices {
		items = runActions(domain.DashboardMenuRunStopThis)[1:]
	}
	caption, busy := m.busyCaption(selected.Branch)
	if !busy {
		return items
	}
	return disableActions(items, caption)
}

// disableActions leaves the headings and the rules alone: a caption under a
// title would read as a refusal to display it.
func disableActions(items []menuItem, caption string) []menuItem {
	for index := range items {
		if items[index].kind != menuEntryAction {
			continue
		}
		items[index].disabled = caption
	}
	return items
}

// worktreeActions is what a row offers. The base row hangs off nothing, so it
// has neither a parent to be moved to nor a rebase to run: all it can do is
// catch up with its own remote — the same entry every other row leads with.
// Without a run block the menu is one block, and a heading or a rule over a
// single block only adds lines: it is then v0.27.1's.
func (m Model) worktreeActions(selected domain.WorktreeStatus) []menuItem {
	run := m.runBlock(runBlockParams{
		Actions: runActions(domain.DashboardMenuRunStop),
		Stop:    menuItem{label: domain.DashboardMenuRunDown, action: menuRunDown},
		Running: m.running[selected.Path] > 0,
	})
	items := append(gitActions(selected, len(run) > 0), run...)
	if selected.IsParent {
		return items
	}
	remove := menuItem{label: domain.DashboardMenuDelete, action: menuDelete, danger: true}
	if len(run) == 0 {
		return append(items, remove)
	}
	return append(items, menuItem{kind: menuEntrySeparator}, remove)
}

type runBlockParams struct {
	// Actions is the block of a project whose run.toml declares jobs.
	Actions []menuItem
	// Stop is what an unreadable run.toml still leaves on offer while Running:
	// stopping is the one gesture that needs nothing from the file.
	Stop    menuItem
	Running bool
}

// runBlock is the run module's part of a menu: nothing for a project that
// declares no job, and one inert entry naming the cause for a run.toml that
// cannot be read.
func (m Model) runBlock(params runBlockParams) []menuItem {
	if m.runConfigErr != nil {
		items := []menuItem{
			{kind: menuEntrySeparator},
			{kind: menuEntryHeading, label: domain.DashboardMenuSectionRun},
			{label: domain.DashboardRunConfigInvalid, disabled: runConfigCause(m.runConfigErr)},
		}
		if !params.Running {
			return items
		}
		return append(items, params.Stop)
	}
	if len(m.runConfig.Jobs) == 0 {
		return nil
	}
	return params.Actions
}

// runConfigCause is the innermost error: the entry already says which file, and
// the path the loader wraps it with would fill the whole caption.
func runConfigCause(err error) string {
	for inner := errors.Unwrap(err); inner != nil; inner = errors.Unwrap(err) {
		err = inner
	}
	cause, _, _ := strings.Cut(err.Error(), "\n")
	return truncate(cause, domain.DashboardMenuCaptionMax)
}

// gitActions is what a row offers on its own history. The base row hangs off
// nothing, so it has neither a parent to be moved to nor a rebase to run: all
// it can do is catch up with its own remote.
//
// The fast-forward is never gated on the origin badges: those come from
// remote-tracking refs with no fetch, so "up to date" and "no counterpart"
// alike are what is known rather than what is true. The run fetches and
// reports the truth; disabled stays what it has always meant here, a run
// holding this worktree right now.
func gitActions(selected domain.WorktreeStatus, titled bool) []menuItem {
	var items []menuItem
	if titled {
		items = append(items, menuItem{kind: menuEntryHeading, label: domain.DashboardMenuSectionGit})
	}
	items = append(items, menuItem{label: domain.DashboardMenuFastForward, action: menuFastForward})
	if selected.IsParent {
		return items
	}
	return append(items,
		menuItem{label: domain.DashboardMenuSync, action: menuSync},
		menuItem{label: domain.DashboardMenuReparent, action: menuReparent},
	)
}

// runActions are the run module's, offered on every row including the base one:
// the main checkout runs jobs like any other worktree. The block is ruled by
// grain — what acts on the whole worktree, then what acts on one job, then the
// view over both — because the flat order alternated between the two and read
// as five unrelated verbs. stopJob is the label of the job stop, which the
// Services tab words for the job its cursor already designates.
func runActions(stopJob string) []menuItem {
	return []menuItem{
		{kind: menuEntrySeparator},
		{kind: menuEntryHeading, label: domain.DashboardMenuSectionRun},
		{label: domain.DashboardMenuRunUp, action: menuRunUp},
		{label: domain.DashboardMenuRunDown, action: menuRunDown},
		{kind: menuEntrySeparator},
		{label: domain.DashboardMenuRunStart, action: menuRunStart},
		{label: stopJob, action: menuRunStop},
		{kind: menuEntrySeparator},
		{label: domain.DashboardMenuRunLogs, action: menuRunLogs},
	}
}

// globalMenuItems act on worktrees the user picks inside the run, not on the
// selected row, so nothing here is keyed off the selection.
func (m Model) globalMenuItems() []menuItem {
	git := []menuItem{
		{label: domain.DashboardMenuFastForwardAll, action: menuFastForwardAll},
		{label: domain.DashboardMenuReparentBatch, action: menuReparentBatch},
		{label: domain.DashboardMenuSyncAll, action: menuSyncAll},
	}
	prune := menuItem{label: domain.DashboardMenuPrune, action: menuPrune, danger: true}
	deleteMany := menuItem{label: domain.DashboardMenuDeleteMany, action: menuDeleteBatch, danger: true}
	run := m.globalRunBlock()
	items := append(git, prune, deleteMany)
	if len(run) > 0 {
		items = append(append([]menuItem{{kind: menuEntryHeading, label: domain.DashboardMenuSectionGit}}, git...), run...)
		items = append(items, menuItem{kind: menuEntrySeparator}, prune, deleteMany)
	}
	caption, busy := m.busyCaption("")
	if !busy {
		return items
	}
	return disableActions(items, caption)
}

func (m Model) globalRunBlock() []menuItem {
	return m.runBlock(runBlockParams{
		Actions: []menuItem{
			{kind: menuEntrySeparator},
			{kind: menuEntryHeading, label: domain.DashboardMenuSectionRun},
			{label: domain.DashboardMenuRunUpAll, action: menuRunUpAll},
			{label: domain.DashboardMenuRunDownAll, action: menuRunDownAll},
			{kind: menuEntrySeparator},
			{label: domain.DashboardMenuRunLogsAll, action: menuRunLogsAll},
		},
		Stop:    menuItem{label: domain.DashboardMenuRunDownAll, action: menuRunDownAll},
		Running: len(m.running) > 0,
	})
}

// openMenu hangs the worktree menu off a cell. The right button is not always
// delivered — some terminals keep it for their own paste — so KeyMenu opens the
// very same menu on the selected row.
func (m Model) openMenu(anchor domain.Rect) Model {
	if _, ok := m.selected(); !ok {
		return m
	}
	return m.open(menuForWorktree, anchor)
}

// openActionsMenu hangs the global menu off the header button. It needs no
// selection: what it acts on is chosen inside the run it starts.
func (m Model) openActionsMenu(anchor domain.Rect) Model {
	return m.open(menuForGlobal, anchor)
}

func (m Model) open(kind menuKind, anchor domain.Rect) Model {
	m.menuKind, m.menuAnchor, m.menuOpen = kind, anchor, true
	m.menuCursor = firstEnabled(m.menuItems())
	return m
}

// menuAnchorPoint is where the keyboard hangs the menu from: under the selected
// row of whichever tab is showing.
func (m Model) menuAnchorPoint() domain.Rect {
	if m.tab == tabTree {
		return m.treeRowPoint()
	}
	return m.selectedRowPoint()
}

// actionsAnchorPoint hangs the global menu under the header button, where the
// mouse would have opened it.
func (m Model) actionsAnchorPoint() domain.Rect {
	if zone := m.zones.Get(zoneActions); !zone.IsZero() {
		return domain.Rect{X: zone.StartX, Y: zone.EndY}
	}
	return domain.Rect{X: 0, Y: m.layout().Tabs.Height - 1}
}

// selectedRowPoint is the last line of the selected row, so the keyboard opens
// the menu where the mouse would have — under the row, not over its own lines.
func (m Model) selectedRowPoint() domain.Rect {
	layout := m.layout()
	top := layout.List.Y + domain.DashboardChromeHeight - 1 + domain.DashboardTitleGap
	stride := domain.DashboardRowHeight + domain.DashboardRowGap
	return domain.Rect{
		X: layout.List.X + borderWidth,
		Y: top + (m.cursor-m.offset)*stride + domain.DashboardRowHeight - 1,
	}
}

func (m Model) closeMenu() Model {
	m.menuOpen, m.menuCursor = false, 0
	return m
}

// moveMenu walks over what cannot be activated: a cursor parked on an inert
// entry is a keypress that does nothing.
func (m Model) moveMenu(delta int) Model {
	items := m.menuItems()
	for index := m.menuCursor + delta; index >= 0 && index < len(items); index += delta {
		if items[index].activatable() {
			m.menuCursor = index
			return m
		}
	}
	return m
}

// firstEnabled is where the cursor lands when the menu opens.
func firstEnabled(items []menuItem) int {
	for index, item := range items {
		if item.activatable() {
			return index
		}
	}
	return 0
}

func (m Model) activateMenu(index int) (Model, tea.Cmd) {
	items := m.menuItems()
	if index < 0 || index >= len(items) {
		return m, nil
	}
	item := items[index]
	if !item.activatable() {
		return m, nil
	}
	m = m.closeMenu()

	selected, ok := m.selected()
	if !ok && m.menuKind == menuForWorktree {
		return m, nil
	}
	switch item.action {
	case menuReparentBatch:
		return m.startBatchReparent()
	case menuPrune:
		return m.startPrune()
	case menuDeleteBatch:
		return m.startBatchClean()
	case menuSyncAll:
		return m.startSyncAll()
	case menuFastForwardAll:
		return m.startFastForwardAll()
	case menuRunUpAll:
		return m.startRunUpAll()
	case menuRunDownAll:
		return m.startRunDownAll()
	case menuRunLogsAll:
		return m.startRunLogsAll()
	case menuReparent:
		return m.startReparent(selected.Branch)
	case menuDelete:
		return m.startClean(selected.Branch)
	case menuSync:
		return m.startSync(selected.Branch)
	case menuFastForward:
		return m.startFastForward(selected.Branch)
	case menuRunUp:
		return m.startRunUp(selected)
	case menuRunStart:
		return m.startRunJob(selected)
	case menuRunStop:
		return m.stopRunJob(selected)
	case menuRunDown:
		return m.startRunDown(selected)
	case menuRunLogs:
		return m.openLogsTab()
	}
	return m, nil
}

// menuBox draws the context menu as a floating box: it is anchored on the cell it
// was opened from, and overlay() pastes it over the frame. It names the worktree
// it acts on, then rules that off from the actions themselves.
func (m Model) menuBox() (string, domain.Rect) {
	title, ok := m.menuTitle()
	if !ok {
		return "", domain.Rect{}
	}

	items := m.menuItems()
	inner := menuInnerWidth(menuInnerWidthParams{Items: items, Title: title, Screen: m.width})
	if inner <= 0 {
		return "", domain.Rect{}
	}

	lines := []string{
		styles.DashboardMenuTitle.Render(truncate(title, inner)),
		styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, inner)),
	}
	if len(items) == 0 {
		lines = append(lines, styles.DashboardEmpty.Render(truncate(domain.DashboardMenuEmpty, inner)))
	}
	for index, item := range items {
		rendered := m.menuItemLines(menuItemParams{Item: item, Inner: inner, Focused: index == m.menuCursor})
		// Only what can be activated is marked: a zone over an inert entry is a
		// click that looks like it should do something.
		if item.activatable() {
			rendered[0] = m.zones.Mark(menuZone(index), rendered[0])
		}
		lines = append(lines, rendered...)
	}

	box := styles.DashboardMenu.Render(strings.Join(lines, "\n"))
	rect := rules.ComputeMenuRect(rules.MenuRectParams{
		AnchorX:      m.menuAnchor.X,
		AnchorY:      m.menuAnchor.Y,
		Width:        lipgloss.Width(box),
		Height:       lipgloss.Height(box),
		ScreenWidth:  m.width,
		ScreenHeight: m.height,
	})
	return box, rect
}

// menuTitle names what the menu acts on: one worktree, or the dashboard itself.
func (m Model) menuTitle() (string, bool) {
	if m.menuKind == menuForGlobal {
		return domain.DashboardActionsTitle, true
	}
	selected, ok := m.selected()
	if !ok {
		return "", false
	}
	return selected.Branch, true
}

type menuItemParams struct {
	Item    menuItem
	Inner   int
	Focused bool
}

// menuItemLines draws one entry: its label, and under it what stands in its way
// when something does. Every entry spans the whole box, so what separates two
// actions is the edge of the block the pointer is over rather than empty space —
// the same accent bar and tint the worktree list uses for the row it is on.
func (m Model) menuItemLines(params menuItemParams) []string {
	if params.Item.kind == menuEntrySeparator {
		return []string{styles.DashboardRule.Render(strings.Repeat(domain.DashboardRuleGlyph, params.Inner))}
	}
	inner := max(params.Inner-rowBarWidth, 0)
	if params.Item.kind == menuEntryHeading {
		return []string{rowIndent + styles.DashboardSectionTitle.Render(pad(truncate(params.Item.label, inner), inner))}
	}
	label := truncate(params.Item.label, inner)
	if params.Item.disabled != "" {
		// The caption hangs under its own entry: at the same indent it would read as
		// a fourth peer in a menu of two.
		caption := max(inner-rowBarWidth, 0)
		return []string{
			rowIndent + styles.DashboardDisabled.Render(pad(label, inner)),
			rowIndent + rowIndent + styles.DashboardRowMeta.Render(truncate(params.Item.disabled, caption)),
		}
	}
	if params.Focused {
		return []string{
			styles.DashboardRowBar.Render(rowBar+" ") + styles.DashboardRowSelected.Width(inner).Render(label),
		}
	}
	return []string{rowIndent + menuItemStyle(params.Item).Render(pad(label, inner))}
}

func menuItemStyle(item menuItem) lipgloss.Style {
	if item.danger {
		return styles.DashboardDanger
	}
	return styles.DashboardRow
}

type menuInnerWidthParams struct {
	Items  []menuItem
	Title  string
	Screen int
}

// menuInnerWidth sizes the menu on its longest line, gutter included, then keeps
// it inside the screen: the box is pasted whole, so it must never need trimming.
func menuInnerWidth(params menuInnerWidthParams) int {
	inner := max(lipgloss.Width(params.Title), lipgloss.Width(domain.DashboardMenuEmpty))
	for _, item := range params.Items {
		// Every entry carries the focus gutter, whether or not it is the focused one.
		inner = max(inner, lipgloss.Width(item.label)+rowBarWidth)
		if item.kind != menuEntryAction {
			continue
		}
		if item.disabled != "" {
			inner = max(inner, lipgloss.Width(item.disabled)+2*rowBarWidth)
		}
	}
	return min(inner, params.Screen-domain.DashboardMenuChrome)
}
