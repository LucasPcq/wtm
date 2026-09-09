package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// Step defines one step in a wizard.
// Model must be a SelectListModel, TextInputModel, ConfirmModel, MultiSelectModel,
// ReorderListModel, HookListModel, EnvResolveModel, PortListModel, RouteListModel,
// ProfileListModel, KindListModel, ScopeListModel, or CmdListModel. Adding one means teaching every switch below about it —
// TestWizardRendersEveryStepModel and its neighbours are what enforce that.
type Step struct {
	Name  string
	Model any
	// Build, when set, rebuilds Model from the already-completed prior steps each
	// time the wizard enters this step. Use it for steps whose contents depend on
	// an earlier selection.
	Build   func(prev []Step) any
	Summary func(model any) string
	// AutoSkip, when set and returning true on entry, skips this step
	// automatically and advances. Use it for steps that are irrelevant given an
	// earlier answer (e.g. a sub-step whose section gate was set to "skip").
	// A skipped step is hidden from the breadcrumb; it is reported via Skipped(i)
	// and, when SkipReason yields a non-empty string, listed in the summaries.
	AutoSkip func(w WizardModel) bool
	// SkipReason, when set, is read right after AutoSkip returns true and returns
	// the human reason the step was skipped. A non-empty reason is shown in the
	// completed-step summaries as "⊘ <Name> — <reason>"; an empty reason keeps the
	// skipped step hidden (compat with init section gates).
	SkipReason func() string
	// Recap marks the final synthesis step: its description is rendered with a
	// distinct "Review & confirm" header (see styles.RenderRecap) so it reads as
	// the action point rather than another prompt.
	Recap bool
	// Callout renders this step's description as an emphasized intro callout
	// (bold title + accent bar) instead of plain muted text. Use it for section
	// gates that explain what a section does.
	Callout bool
	// CalloutNote is an optional secondary line shown in the callout (e.g.
	// "Detected: …"). Only used when Callout is true.
	CalloutNote string
	// CanRefresh marks a SelectList step the user can refresh in place with the
	// refresh key (e.g. re-fetch branches). It gates the key interception and adds
	// an "r refresh" hint to the help bar for this step only.
	CanRefresh bool
	// OnEnter, when set, returns a command fired each time the wizard advances into
	// this step (not on back navigation, where the step keeps its prior state). Use
	// it to kick off a per-step async load — pair it with a message handler that
	// updates the step model and toggles the loading spinner. prev is the completed
	// prior steps, so the command can depend on an earlier answer.
	OnEnter func(prev []Step) tea.Cmd
}

// WizardMsgHandler intercepts a message before it reaches the current step.
// It receives the wizard (so it can mutate step models, e.g. refresh badges
// when async data arrives) and returns a command plus whether it consumed the
// message. A consumed message is not forwarded to the step.
type WizardMsgHandler func(w *WizardModel, msg tea.Msg) (tea.Cmd, bool)

// WizardModel manages a multi-step form with breadcrumb and back navigation.
type WizardModel struct {
	steps         []Step
	skipped       []bool
	skippedReason []string
	current       int
	width         int
	height        int
	done          bool
	aborted       bool
	initCmd       tea.Cmd
	onMsg         WizardMsgHandler
	// Async status banner, rendered under the breadcrumb. While loading, an
	// animated spinner + loadingText is shown inside the box; once a handler
	// calls SetLoading(false), the banner (if set) is shown instead.
	spinner     spinner.Model
	loading     bool
	loadingText string
	banner      WizardBanner
}

// WizardBanner is an optional titled notice shown in the wizard's status box
// (e.g. a "GitHub not connected" hint surfaced after an async fetch).
type WizardBanner struct {
	Title string
	Lines []string
}

// NewWizard creates a wizard with the given steps.
func NewWizard(steps []Step) WizardModel {
	return WizardModel{
		steps:         steps,
		skipped:       make([]bool, len(steps)),
		skippedReason: make([]string, len(steps)),
		width:         80,
	}
}

// WizardParams holds inputs for a wizard that runs a background command and/or
// intercepts messages (used for async streaming such as lazily-loaded PRs).
type WizardParams struct {
	Steps   []Step
	InitCmd tea.Cmd
	OnMsg   WizardMsgHandler
	// Loading, when true, renders an animated spinner + LoadingText under the
	// breadcrumb until a message handler calls SetLoading(false).
	Loading     bool
	LoadingText string
}

// NewWizardWithParams creates a wizard that fires InitCmd on start and routes
// every message through OnMsg before the current step.
func NewWizardWithParams(params WizardParams) WizardModel {
	m := NewWizard(params.Steps)
	m.initCmd = params.InitCmd
	m.onMsg = params.OnMsg
	m.loading = params.Loading
	m.loadingText = params.LoadingText
	m.spinner = newMutedSpinner()
	return m
}

// SetLoading toggles the async loading state (stops the spinner when false).
func (m *WizardModel) SetLoading(loading bool) { m.loading = loading }

// StartLoading enters the loading state with the given text and returns the
// spinner tick command so the spinner animates even when the wizard did not start
// loading at Init (e.g. an in-flight refresh triggered by a key press).
func (m *WizardModel) StartLoading(text string) tea.Cmd {
	m.loading = true
	m.loadingText = text
	return m.spinner.Tick
}

// Loading reports whether the wizard is currently showing the loading spinner.
func (m WizardModel) Loading() bool { return m.loading }

// CurrentStepModel returns the current step's child model (nil if out of range),
// letting a message handler inspect it (e.g. to skip refresh while filtering).
func (m WizardModel) CurrentStepModel() any {
	if m.current < 0 || m.current >= len(m.steps) {
		return nil
	}
	return m.steps[m.current].Model
}

// CurrentStepCanRefresh reports whether the current step opted into in-place
// refresh via Step.CanRefresh.
func (m WizardModel) CurrentStepCanRefresh() bool {
	if m.current < 0 || m.current >= len(m.steps) {
		return false
	}
	return m.steps[m.current].CanRefresh
}

// RebuildCurrentStep re-runs the current step's Build hook (if any) against the
// completed prior steps, re-deriving its model. Used to refresh a step in place
// after its data source changed (e.g. branch candidates were re-fetched). Unlike
// buildStep it also rebuilds the first step, which advance never reaches.
func (m *WizardModel) RebuildCurrentStep() {
	if m.current < 0 || m.current >= len(m.steps) {
		return
	}
	step := &m.steps[m.current]
	if step.Build == nil {
		return
	}
	step.Model = step.Build(m.steps[:m.current])
	m.propagateSize(m.current)
}

// SetBanner sets the status banner shown under the breadcrumb once loading has
// finished (e.g. a "GitHub CLI not connected" hint). An empty Title hides it.
func (m *WizardModel) SetBanner(banner WizardBanner) { m.banner = banner }

// UpdateStepModel replaces the model of the step at stepIdx by applying fn.
// Used by message handlers to mutate a step (e.g. refresh a SelectList's badges)
// in response to async messages.
func (m *WizardModel) UpdateStepModel(stepIdx int, fn func(model any) any) {
	if stepIdx < 0 || stepIdx >= len(m.steps) {
		return
	}
	m.steps[stepIdx].Model = fn(m.steps[stepIdx].Model)
}

// Done returns true when all steps have been completed.
func (m WizardModel) Done() bool { return m.done }

// Aborted returns true when the user aborted at the first step.
func (m WizardModel) Aborted() bool { return m.aborted }

// Steps returns the wizard steps for value extraction.
func (m WizardModel) Steps() []Step { return m.steps }

// Skipped reports whether the step at index i was skipped by the user.
func (m WizardModel) Skipped(i int) bool {
	if i < 0 || i >= len(m.skipped) {
		return false
	}
	return m.skipped[i]
}

// Init initializes the current step's model (the first step unless the wizard
// was created with NewWizardAtStep).
func (m WizardModel) Init() tea.Cmd {
	if len(m.steps) == 0 {
		return tea.Quit
	}
	// Run the initial step's Build so a step whose content derives from presets (a
	// recap that is itself the first step, e.g. `create <branch> --from --env-from`)
	// is populated. advance() builds every later step on entry; without this the
	// very first step would keep its placeholder. Prior-step models are already set
	// (construction or NewWizardAtStep), so Build sees completed inputs.
	if build := m.steps[m.current].Build; build != nil {
		m.steps[m.current].Model = build(m.steps[:m.current])
	}
	m.propagateSize(m.current)
	cmds := []tea.Cmd{m.initStep(m.current)}
	if m.initCmd != nil {
		cmds = append(cmds, m.initCmd)
	}
	if m.loading {
		cmds = append(cmds, m.spinner.Tick)
	}
	return tea.Batch(cmds...)
}

// Update delegates to the current step and manages transitions.
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.onMsg != nil {
		if cmd, handled := m.onMsg(&m, msg); handled {
			return m, cmd
		}
	}

	if _, ok := msg.(spinner.TickMsg); ok {
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	if wsMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = wsMsg.Width
		m.height = wsMsg.Height
		m.propagateSize(m.current)
		return m, nil
	}

	if m.current >= len(m.steps) {
		return m, nil
	}

	step := &m.steps[m.current]

	advanced, back, cmd := m.updateStep(step, msg)
	if advanced {
		return m.advance()
	}
	if back {
		return m.goBack()
	}

	return m, cmd
}

func (m WizardModel) updateStep(step *Step, msg tea.Msg) (advanced bool, back bool, cmd tea.Cmd) {
	switch child := step.Model.(type) {
	case SelectListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Chosen(), updated.Aborted(), c
	case TextInputModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case ConfirmModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case MultiSelectModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case ReorderListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case HookListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case EnvResolveModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case PortListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case RouteListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case RunnerListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case ProfileListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case KindListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case ScopeListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	case CmdListModel:
		updated, c := child.Update(msg)
		step.Model = updated
		return updated.Done(), updated.Aborted(), c
	}
	return false, false, nil
}

// View renders the breadcrumb, completed step summaries, and the current step.
// It is assembled as head + list + tail so the same fragments can be measured by
// chromeHeight to size the list — keeping the whole render within the terminal
// height (and thus the breadcrumb on screen). See LUC-85.
func (m WizardModel) View() string {
	if len(m.steps) == 0 || m.done || m.aborted {
		return ""
	}
	return m.renderHead() + m.viewStep(m.current) + m.renderTail()
}

// renderHead renders everything above the current step's list: the leading
// blank, breadcrumb, status banner, the (bounded) completed-step summaries, and
// the step description.
func (m WizardModel) renderHead() string {
	var b strings.Builder
	b.WriteString(m.renderTop())
	if summaries := m.renderSummaries(m.maxSummaryLines()); summaries != "" {
		b.WriteString(summaries)
		b.WriteString("\n")
	}
	if m.current > 0 {
		b.WriteString("\n")
	}
	if desc := m.renderDescription(); desc != "" {
		b.WriteString(desc)
		b.WriteString("\n\n")
	}
	return b.String()
}

// renderTail renders everything below the current step's list (the help bar).
func (m WizardModel) renderTail() string {
	return "\n\n" + m.renderHelpBar() + "\n"
}

// renderTop renders the leading blank, breadcrumb, and optional status banner.
// The wizard owns its own top padding (one blank line between the prompt and the
// breadcrumb), matching standaloneModel and the framed non-TUI command output.
func (m WizardModel) renderTop() string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(m.renderBreadcrumb())
	b.WriteString("\n\n")
	if status := m.renderStatusBanner(); status != "" {
		b.WriteString(status)
		b.WriteString("\n\n")
	}
	return b.String()
}

// renderSummaries renders the "✓ <step>: <summary>" lines for completed visible
// steps. When they would exceed maxLines, only the most recent are shown and a
// muted "… (N earlier steps)" line stands in for the rest, so the block stays
// bounded and never pushes the breadcrumb off-screen. maxLines <= 0 means
// unbounded.
func (m WizardModel) renderSummaries(maxLines int) string {
	var lines []string
	for i := 0; i < m.current; i++ {
		if m.skipped[i] {
			if reason := m.skippedReason[i]; reason != "" {
				line := fmt.Sprintf("  ⊘ %s — %s", m.steps[i].Name, reason)
				lines = append(lines, styles.SummaryLineSkipped.Render(line))
			}
			continue
		}
		summary := ""
		if m.steps[i].Summary != nil {
			summary = m.steps[i].Summary(m.steps[i].Model)
		}
		line := fmt.Sprintf("  ✓ %s: %s", m.steps[i].Name, summary)
		lines = append(lines, styles.SummaryLine.Render(line))
	}
	if len(lines) == 0 {
		return ""
	}
	if maxLines > 0 && len(lines) > maxLines {
		lines = collapseSummaries(lines, maxLines)
	}
	return strings.Join(lines, "\n")
}

// collapseSummaries keeps the most recent lines and prefixes a muted collapse
// line standing in for the hidden older ones, fitting the result into maxLines.
func collapseSummaries(lines []string, maxLines int) []string {
	keep := maxLines - 1
	if keep < 0 {
		keep = 0
	}
	hidden := len(lines) - keep
	collapse := styles.Muted.Render(fmt.Sprintf("  ✓ … (%d earlier steps)", hidden))
	return append([]string{collapse}, lines[len(lines)-keep:]...)
}

// renderDescription renders the current step's description (callout or muted),
// without the trailing blank lines the head adds around it.
func (m WizardModel) renderDescription() string {
	step := m.steps[m.current]
	desc := m.stepDescription(step)
	if desc == "" {
		return ""
	}
	if step.Recap {
		return styles.RenderRecap(styles.IntroParams{
			Width: m.width,
			Title: domain.WizardRecapTitle,
			Body:  desc,
		})
	}
	if step.Callout {
		return styles.RenderIntro(styles.IntroParams{
			Width: m.width,
			Body:  desc,
			Note:  step.CalloutNote,
		})
	}
	return styles.Muted.Render(indentLines(desc))
}

// chromeHeight is the number of newlines around the current step's list (head +
// tail). The list renders R rows as R-1 newlines with no trailing newline, so
// the full render occupies chromeHeight + R rows; propagateSize sizes the list to
// R = height - chromeHeight, keeping the whole render — breadcrumb included — on
// screen.
func (m WizardModel) chromeHeight() int {
	return strings.Count(m.renderHead(), "\n") + strings.Count(m.renderTail(), "\n")
}

// baseChromeHeight is chromeHeight excluding the completed-step summaries: the
// fixed overhead the summaries must leave room for. Used to budget how many
// summary lines can be shown above the list.
func (m WizardModel) baseChromeHeight() int {
	var b strings.Builder
	b.WriteString(m.renderTop())
	if m.current > 0 {
		b.WriteString("\n")
	}
	if desc := m.renderDescription(); desc != "" {
		b.WriteString(desc)
		b.WriteString("\n\n")
	}
	return strings.Count(b.String(), "\n") + strings.Count(m.renderTail(), "\n")
}

// maxSummaryLines is how many summary lines fit above the list while still
// reserving MinWizardListHeight rows for the list itself. Each summary line adds
// exactly one newline to the head, so the budget is height - base - minList.
func (m WizardModel) maxSummaryLines() int {
	avail := m.height - m.baseChromeHeight() - domain.MinWizardListHeight
	if avail < 1 {
		return 1
	}
	return avail
}

// renderStatusBanner renders the async status box: an animated spinner while
// loading, otherwise the banner (if set). Both share the same bordered box so
// the loading state visually becomes the resulting notice.
func (m WizardModel) renderStatusBanner() string {
	if m.loading {
		return renderLoadingBox(m.spinner.View(), m.loadingText)
	}
	if m.banner.Title != "" {
		rows := append([]string{styles.CalloutTitle.Render(m.banner.Title)}, m.banner.Lines...)
		return styles.StatusBox.Render(strings.Join(rows, "\n"))
	}
	return ""
}

func (m WizardModel) renderBreadcrumb() string {
	counter := styles.Breadcrumb.Render(fmt.Sprintf("  Step %d/%d", m.visiblePosition(), m.visibleCount()))
	sep := styles.Breadcrumb.Render(" • ")
	name := styles.BreadcrumbActive.Render(m.steps[m.current].Name)
	return counter + sep + name
}

// visibleCount is the breadcrumb denominator: the fixed total number of steps.
// It stays constant across the whole flow so the counter reads honestly — an
// auto-skipped step makes the position jump (e.g. 3/5 → 5/5) rather than shrinking
// the total. The final recap step is unconditional, so the last step is reliably
// n/n.
func (m WizardModel) visibleCount() int {
	return len(m.steps)
}

// visiblePosition is the 1-based index of the current step. Because skipped steps
// are hopped over in advance(), this jumps past them, matching the fixed
// denominator from visibleCount.
func (m WizardModel) visiblePosition() int {
	return m.current + 1
}

// stepHelp is what a step says about its own keys. The bar itself is composed
// in one place: a step that spelled its whole line out is how the wizard ended
// up naming the same gesture "confirm", "select" and "continue" on three
// consecutive screens.
type stepHelp interface {
	// helpActions are the row-level gestures this step adds, between navigation
	// and confirmation, in the order they are worth learning.
	helpActions() []string
	// helpModal is the whole bar to show instead while the step is in a
	// sub-mode — editing a value, naming a profile, typing a filter — where
	// none of the navigation keys apply.
	helpModal() string
}

func (m WizardModel) renderHelpBar() string {
	return styles.HelpBar.Render(m.helpLine())
}

func (m WizardModel) helpLine() string {
	step, ok := m.steps[m.current].Model.(stepHelp)
	if !ok {
		return m.composedHelp(nil)
	}
	if modal := step.helpModal(); modal != "" {
		return modal
	}
	return m.composedHelp(step.helpActions())
}

// rowless is a step with nothing to move between: it takes typing, not a cursor.
// The assertion below names its one implementation — the step models are held
// as `any`, so without it nothing ties the method to the interface, for a
// reader or for a reachability analysis.
type rowless interface{ helpRowless() bool }

var _ rowless = TextInputModel{}

// doneRower is a step whose last row confirms it. The word for enter follows:
// on such a step enter acts on the row under the cursor, everywhere else it
// ends the step.
type doneRower interface{ doneRow() int }

func (m WizardModel) confirmHelp() string {
	if _, ok := m.steps[m.current].Model.(doneRower); ok {
		return domain.HelpSelect
	}
	return domain.HelpConfirm
}

// composedHelp lays every bar out the same way: navigate, then what this step
// adds, then confirm, then the way out — whose word depends on whether there is
// a step to go back to.
func (m WizardModel) composedHelp(actions []string) string {
	parts := make([]string, 0, len(actions)+3)
	if _, rowless := m.steps[m.current].Model.(rowless); !rowless {
		parts = append(parts, domain.HelpNavigate)
	}
	parts = append(parts, actions...)
	if m.steps[m.current].CanRefresh {
		parts = append(parts, domain.HelpRefresh)
	}
	parts = append(parts, m.confirmHelp())
	if m.visiblePosition() > 1 {
		parts = append(parts, domain.HelpBack)
	} else {
		parts = append(parts, domain.HelpCancel)
	}
	return domain.HelpBarIndent + strings.Join(parts, domain.HelpBarSep)
}

func (m *WizardModel) propagateSize(stepIdx int) {
	if stepIdx >= len(m.steps) {
		return
	}
	h := max(1, m.height-m.chromeHeight())
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case TextInputModel:
		child.width = m.width
		child.input.Width = max(10, m.width-4)
		m.steps[stepIdx].Model = child
	case ConfirmModel:
		child.width = m.width
		m.steps[stepIdx].Model = child
	case MultiSelectModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case ReorderListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case HookListModel:
		child.width = m.width
		child.height = h
		child.cmdInput.Width = max(hookInputMinWidth, m.width-hookInputWidthInset)
		child.cwdInput.Width = max(hookInputMinWidth, m.width-hookInputWidthInset)
		m.steps[stepIdx].Model = child
	case EnvResolveModel:
		child.width = m.width
		child.height = h
		child.input.Width = max(10, m.width-8)
		m.steps[stepIdx].Model = child
	case PortListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case RouteListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case RunnerListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case ProfileListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case KindListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case ScopeListModel:
		child.width = m.width
		child.height = h
		m.steps[stepIdx].Model = child
	case CmdListModel:
		child.width = m.width
		child.height = h
		child.input.Width = max(domain.CmdListMinWidth, m.width-domain.CmdListWidthInset)
		m.steps[stepIdx].Model = child
	}
}

func (m WizardModel) advance() (tea.Model, tea.Cmd) {
	m.current++
	for m.current < len(m.steps) {
		m.buildStep(m.current)
		if m.steps[m.current].AutoSkip != nil && m.steps[m.current].AutoSkip(m) {
			m.skipped[m.current] = true
			if sr := m.steps[m.current].SkipReason; sr != nil {
				m.skippedReason[m.current] = sr()
			}
			m.current++
			continue
		}
		break
	}
	if m.current >= len(m.steps) {
		m.done = true
		return m, tea.Quit
	}
	m.propagateSize(m.current)
	cmds := []tea.Cmd{m.initStep(m.current)}
	if onEnter := m.steps[m.current].OnEnter; onEnter != nil {
		cmds = append(cmds, onEnter(m.steps[:m.current]))
	}
	return m, tea.Batch(cmds...)
}

// buildStep rebuilds a step's model from prior steps when a Build hook is set.
func (m *WizardModel) buildStep(stepIdx int) {
	if stepIdx <= 0 || stepIdx >= len(m.steps) {
		return
	}
	step := &m.steps[stepIdx]
	if step.Build == nil {
		return
	}
	step.Model = step.Build(m.steps[:stepIdx])
}

func (m WizardModel) goBack() (tea.Model, tea.Cmd) {
	// Land on the nearest previous visible step, hopping over auto-skipped ones.
	prev := m.current - 1
	for prev >= 0 && m.skipped[prev] {
		prev--
	}
	if prev < 0 {
		m.aborted = true
		return m, tea.Quit
	}
	m.resetStep(m.current)
	// Clear skip flags for the steps we hop back over so re-advancing
	// re-evaluates AutoSkip against the (possibly changed) gate answer.
	for i := prev; i < m.current; i++ {
		m.skipped[i] = false
		m.skippedReason[i] = ""
	}
	m.current = prev
	m.resetStep(m.current)
	m.propagateSize(m.current)
	return m, m.initStep(m.current)
}

func (m WizardModel) initStep(stepIdx int) tea.Cmd {
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		return child.Init()
	case TextInputModel:
		return child.Init()
	case ConfirmModel:
		return child.Init()
	case MultiSelectModel:
		return child.Init()
	case ReorderListModel:
		return child.Init()
	case HookListModel:
		return child.Init()
	case EnvResolveModel:
		return child.Init()
	case PortListModel:
		return child.Init()
	case RouteListModel:
		return child.Init()
	case RunnerListModel:
		return child.Init()
	case ProfileListModel:
		return child.Init()
	case KindListModel:
		return child.Init()
	case ScopeListModel:
		return child.Init()
	case CmdListModel:
		return child.Init()
	}
	return nil
}

func (m WizardModel) viewStep(stepIdx int) string {
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		return child.View()
	case TextInputModel:
		return child.View()
	case ConfirmModel:
		return child.View()
	case MultiSelectModel:
		return child.View()
	case ReorderListModel:
		return child.View()
	case HookListModel:
		return child.View()
	case EnvResolveModel:
		return child.View()
	case PortListModel:
		return child.View()
	case RouteListModel:
		return child.View()
	case RunnerListModel:
		return child.View()
	case ProfileListModel:
		return child.View()
	case KindListModel:
		return child.View()
	case ScopeListModel:
		return child.View()
	case CmdListModel:
		return child.View()
	}
	return ""
}

func (m *WizardModel) resetStep(stepIdx int) {
	if stepIdx >= len(m.steps) {
		return
	}
	switch child := m.steps[stepIdx].Model.(type) {
	case SelectListModel:
		child.chosen = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case TextInputModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case ConfirmModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case MultiSelectModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case ReorderListModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case HookListModel:
		child.done = false
		child.aborted = false
		child.editing = false
		m.steps[stepIdx].Model = child
	case EnvResolveModel:
		child.done = false
		child.aborted = false
		child.editing = false
		m.steps[stepIdx].Model = child
	case PortListModel:
		child.done = false
		child.aborted = false
		child.editing = false
		m.steps[stepIdx].Model = child
	case RouteListModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case RunnerListModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case ProfileListModel:
		child.done = false
		child.aborted = false
		child.naming = false
		m.steps[stepIdx].Model = child
	case KindListModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case ScopeListModel:
		child.done = false
		child.aborted = false
		m.steps[stepIdx].Model = child
	case CmdListModel:
		child.done = false
		child.aborted = false
		child.editing = false
		m.steps[stepIdx].Model = child
	}
}

// indentLines prefixes every line with the standard indent so multi-line
// descriptions stay aligned (lipgloss only pads the first line otherwise).
func indentLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = styles.Indent + line
	}
	return strings.Join(lines, "\n")
}

func (m WizardModel) stepDescription(step Step) string {
	switch child := step.Model.(type) {
	case SelectListModel:
		return child.desc
	case TextInputModel:
		return child.desc
	case ConfirmModel:
		return child.desc
	case MultiSelectModel:
		return child.desc
	case ReorderListModel:
		return child.desc
	case HookListModel:
		return child.desc
	case EnvResolveModel:
		return child.desc
	case PortListModel:
		return child.desc
	case RouteListModel:
		return child.desc
	case RunnerListModel:
		return child.desc
	case ProfileListModel:
		return child.desc
	case KindListModel:
		return child.desc
	case ScopeListModel:
		return child.desc
	case CmdListModel:
		return child.desc
	}
	return ""
}
