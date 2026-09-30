package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

// envInputCharLimit caps the inline edit field.
const envInputCharLimit = 512

// optCode is the decision behind a resolve row's currently selected action.
type optCode int

const (
	optKeep      optCode = iota // conflict/orphan: leave as-is
	optOverwrite                // conflict: take the source value
	optAdd                      // resolved add: include it (default)
	optAccept                   // missing: use the template placeholder as-is
	optSkip                     // missing: leave out; add: don't add
	optRemove                   // orphan: prune the key
)

// envOption is one selectable (non-edit) action on a row.
type envOption struct {
	label string
	code  optCode
}

// envRow is one line of the resolve screen: a file header (Header) or a key entry.
// For an entry, Options are the base actions cycled with ←→; Edit (conflict /
// missing / add) overrides them with a typed value.
type envRow struct {
	header bool
	target string
	source string

	key         string
	status      domain.EnvKeyStatus
	current     string
	resolved    string
	sourceLabel string
	placeholder string
	isAdd       bool // resolved addition sourced from parent/main

	options []envOption
	sel     int
	canEdit bool
	useEdit bool
	edited  string
}

// EnvFileDecision is the collected resolution for one file, returned by the model.
type EnvFileDecision struct {
	Target       string
	Decisions    map[string]domain.EnvConflictDecision
	FilledValues map[string]string
	PruneKeys    []string
	SkipKeys     []string
}

// EnvResolveModel is the single-screen interactive resolver for `wtm env`: it lists
// every drifting key grouped by file (conflicts, missing, orphans, and additions)
// and collects a per-key decision. It is a wizard step model in the same shape as
// HookListModel (value receiver, fields sized by the wizard).
type EnvResolveModel struct {
	rows    []envRow
	cursor  int
	width   int
	height  int
	title   string
	desc    string
	done    bool
	aborted bool

	editing bool
	input   textinput.Model
}

// NewEnvResolveParams holds inputs for NewEnvResolve.
type NewEnvResolveParams struct {
	Title       string
	Description string
	Files       []domain.EnvFileResult
}

// NewEnvResolve builds the model from computed per-file drift.
func NewEnvResolve(params NewEnvResolveParams) EnvResolveModel {
	m := EnvResolveModel{
		title: params.Title,
		desc:  params.Description,
		width: 80,
		rows:  buildEnvRows(params.Files),
	}
	m.cursor = m.firstNavigable()
	return m
}

// buildEnvRows flattens the files into header + entry rows, dropping in-sync keys.
func buildEnvRows(files []domain.EnvFileResult) []envRow {
	var rows []envRow
	for _, f := range files {
		var entries []envRow
		for _, e := range f.Diff.Entries {
			row, keep := entryRow(f, e)
			if keep {
				entries = append(entries, row)
			}
		}
		if len(entries) == 0 {
			continue
		}
		rows = append(rows, envRow{
			header: true,
			target: f.Target,
			source: fmt.Sprintf("strategy: %s  ·  source: %s", f.Strategy, f.Source),
		})
		rows = append(rows, entries...)
	}
	return rows
}

// entryRow maps one diff entry to a row, or keep=false when the key is in sync.
func entryRow(f domain.EnvFileResult, e domain.EnvKeyDiff) (envRow, bool) {
	row := envRow{
		key:         e.Key,
		status:      e.Status,
		current:     e.CurrentValue,
		resolved:    e.ResolvedValue,
		sourceLabel: e.Source,
		placeholder: e.Placeholder,
	}
	switch e.Status {
	case domain.EnvKeyConflict:
		row.options = []envOption{{"keep", optKeep}, {"use " + sourceName(e.Source, f.ParentBranch), optOverwrite}}
		row.canEdit = true
		return row, true
	case domain.EnvKeyMissing:
		row.options = []envOption{{"accept", optAccept}, {"skip", optSkip}}
		row.canEdit = true
		return row, true
	case domain.EnvKeyOrphan:
		row.options = []envOption{{"keep", optKeep}, {"remove", optRemove}}
		return row, true
	case domain.EnvKeyResolved:
		if e.CurrentValue == "" && e.ResolvedValue != "" {
			row.isAdd = true
			row.options = []envOption{{"add", optAdd}, {"skip", optSkip}}
			row.canEdit = true
			return row, true
		}
	}
	return envRow{}, false
}

// sourceName turns a per-key cascade level ("parent" / "main") into a human name:
// the actual parent branch when the value came from the parent worktree, else the
// level itself. So a conflict row reads "use feature", not "use parent".
func sourceName(level, parentBranch string) string {
	if level == domain.EnvSourceParent && parentBranch != "" {
		return parentBranch
	}
	if level == "" {
		return domain.EnvSourceMain
	}
	return level
}

// Empty reports whether there is nothing to decide (no entry rows) — the worktree
// is already in sync. The wizard auto-skips this step.
func (m EnvResolveModel) Empty() bool {
	for i := range m.rows {
		if m.navigable(i) {
			return false
		}
	}
	return true
}

// RecapLines renders the pending decisions grouped by file, styled to separate the
// key (bold), the action (semantic color), and the value (quoted).
func (m EnvResolveModel) RecapLines() []string {
	var lines []string
	for _, r := range m.rows {
		if r.header {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, styles.Bold.Render(r.target)+":")
			continue
		}
		lines = append(lines, "  "+recapLineFor(r))
	}
	return lines
}

// recapLineFor renders one row's decision: bold key, colored action verb, quoted
// value. Every line shows the value involved, consistently across actions.
func recapLineFor(r envRow) string {
	key := styles.Bold.Render(r.key)
	if r.useEdit {
		return recapLine(key, styles.Primary.Render("set"), plainVal(r.edited))
	}
	code := r.options[r.sel].code
	st := actionStyle(code)
	switch code {
	case optOverwrite:
		return recapLine(key, st.Render("overwrite →"), plainVal(r.resolved))
	case optAdd:
		return recapLine(key, st.Render("add"), plainVal(r.resolved))
	case optAccept:
		return recapLine(key, st.Render("set"), plainVal(r.placeholder))
	case optSkip:
		val := r.placeholder
		if r.isAdd {
			val = r.resolved
		}
		return recapLine(key, st.Render("skip"), st.Render(fmt.Sprintf("(%s not added)", plainVal(val))))
	case optRemove:
		return recapLine(key, st.Render("remove"), plainVal(r.current))
	default: // optKeep (conflict or orphan)
		return recapLine(key, st.Render("keep"), plainVal(r.current))
	}
}

// actionStyle maps an action to its semantic color, shared by the live list, the
// recap, and the glossary so a color means the same thing everywhere: accent =
// writes a value, muted = leaves as-is, danger = removes.
func actionStyle(code optCode) lipgloss.Style {
	switch code {
	case optRemove:
		return styles.DangerText
	case optKeep, optSkip:
		return styles.Muted
	default: // optOverwrite, optAdd, optAccept
		return styles.Primary
	}
}

// actionStyleFor is actionStyle for a row's current selection (an active edit writes
// a value → accent).
func actionStyleFor(r envRow) lipgloss.Style {
	if r.useEdit {
		return styles.Primary
	}
	return actionStyle(r.options[r.sel].code)
}

// recapLine assembles "key  action value".
func recapLine(key, action, value string) string {
	return fmt.Sprintf("%s  %s %s", key, action, value)
}

// EnvResolveGlossary is the short legend shown as a callout above the resolve list:
// what each case keyword means, colored to match the list's status column.
func EnvResolveGlossary() string {
	warn, mut := styles.Warning, styles.Muted
	caseCol := func(s lipgloss.Style, w string) string { return s.Render(fmt.Sprintf("%-9s", w)) }
	return strings.Join([]string{
		caseCol(warn, "conflict") + mut.Render("your value differs from the source"),
		caseCol(warn, "missing") + mut.Render("expected, but has no value yet"),
		caseCol(mut, "orphan") + mut.Render("present here, but in no source"),
		caseCol(mut, "add") + mut.Render("the source has a value you don't"),
	}, "\n")
}

func plainVal(v string) string {
	if v == "" {
		return "(empty)"
	}
	return fmt.Sprintf("%q", v)
}

// Done reports the user pressed Enter to proceed.
func (m EnvResolveModel) Done() bool { return m.done }

// Aborted reports the user pressed Esc to go back.
func (m EnvResolveModel) Aborted() bool { return m.aborted }

// Init satisfies tea.Model.
func (m EnvResolveModel) Init() tea.Cmd { return nil }

// Decisions returns the collected per-file resolutions.
func (m EnvResolveModel) Decisions() []EnvFileDecision {
	byTarget := map[string]*EnvFileDecision{}
	var order []string
	current := ""
	for _, r := range m.rows {
		if r.header {
			current = r.target
			if _, ok := byTarget[current]; !ok {
				byTarget[current] = &EnvFileDecision{
					Target:       current,
					Decisions:    map[string]domain.EnvConflictDecision{},
					FilledValues: map[string]string{},
				}
				order = append(order, current)
			}
			continue
		}
		applyRowDecision(byTarget[current], r)
	}

	out := make([]EnvFileDecision, 0, len(order))
	for _, t := range order {
		out = append(out, *byTarget[t])
	}
	return out
}

// applyRowDecision folds one row's chosen action into its file decision.
func applyRowDecision(d *EnvFileDecision, r envRow) {
	if r.useEdit {
		d.FilledValues[r.key] = r.edited
		return
	}
	switch r.options[r.sel].code {
	case optOverwrite:
		d.Decisions[r.key] = domain.EnvDecisionOverwrite
	case optKeep:
		if r.status == domain.EnvKeyConflict {
			d.Decisions[r.key] = domain.EnvDecisionKeep
		}
	case optAccept:
		d.FilledValues[r.key] = r.placeholder
	case optRemove:
		d.PruneKeys = append(d.PruneKeys, r.key)
	case optSkip:
		if r.isAdd {
			d.SkipKeys = append(d.SkipKeys, r.key)
		}
	case optAdd:
		// included automatically by ApplyEnvDiff
	}
}

// Update handles key events for browse and inline-edit modes.
func (m EnvResolveModel) Update(msg tea.Msg) (EnvResolveModel, tea.Cmd) {
	if m.editing {
		return m.updateEdit(msg)
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.cursor = m.prevNavigable(m.cursor)
	case "down", "j":
		m.cursor = m.nextNavigable(m.cursor)
	case "left", "h":
		m.cycle(-1)
	case "right", "l":
		m.cycle(1)
	case "e":
		if m.onEditableRow() {
			m = m.startEdit()
		}
	case "enter":
		m.done = true
	case "esc":
		m.aborted = true
	}
	return m, nil
}

// cycle changes the current row's selected option and discards any edit override,
// so the displayed action and the stored value can never diverge (cycling away
// from an edit drops it; re-editing starts from the default again).
func (m *EnvResolveModel) cycle(delta int) {
	if !m.onNavigable() {
		return
	}
	r := &m.rows[m.cursor]
	r.useEdit = false
	r.edited = ""
	n := len(r.options)
	r.sel = (r.sel + delta%n + n) % n
}

// startEdit opens the inline field prefilled with the value the currently selected
// action would apply — so editing from "keep" starts at the current value, and
// editing from "use <source>" starts at the source value.
func (m EnvResolveModel) startEdit() EnvResolveModel {
	r := m.rows[m.cursor]
	ti := textinput.New()
	ti.CharLimit = envInputCharLimit
	ti.Width = max(10, m.width-8)
	ti.Prompt = styles.InputPrompt.Render("❯ ")
	ti.SetValue(selectedRawValue(r))
	ti.Focus()
	m.input = ti
	m.editing = true
	return m
}

// selectedRawValue is the raw (unstyled) value the row's current action applies, used
// as the edit field's starting point.
func selectedRawValue(r envRow) string {
	if r.useEdit {
		return r.edited
	}
	switch r.options[r.sel].code {
	case optOverwrite, optAdd:
		return r.resolved
	case optAccept, optSkip:
		return r.placeholder
	default: // optKeep
		return r.current
	}
}

func (m EnvResolveModel) updateEdit(msg tea.Msg) (EnvResolveModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.rows[m.cursor].edited = strings.TrimSpace(m.input.Value())
			m.rows[m.cursor].useEdit = true
			m.editing = false
			return m, nil
		case "esc":
			m.editing = false
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// ── navigation over entry rows (skip headers) ────────────────────────────────

func (m EnvResolveModel) navigable(i int) bool {
	return i >= 0 && i < len(m.rows) && !m.rows[i].header
}
func (m EnvResolveModel) onNavigable() bool   { return m.navigable(m.cursor) }
func (m EnvResolveModel) onEditableRow() bool { return m.onNavigable() && m.rows[m.cursor].canEdit }
func (m EnvResolveModel) firstNavigable() int { return m.nextNavigable(-1) }
func (m EnvResolveModel) nextNavigable(from int) int {
	for i := from + 1; i < len(m.rows); i++ {
		if m.navigable(i) {
			return i
		}
	}
	if m.navigable(from) {
		return from
	}
	return m.firstOr(from)
}
func (m EnvResolveModel) prevNavigable(from int) int {
	for i := from - 1; i >= 0; i-- {
		if m.navigable(i) {
			return i
		}
	}
	return from
}
func (m EnvResolveModel) firstOr(fallback int) int {
	for i := 0; i < len(m.rows); i++ {
		if m.navigable(i) {
			return i
		}
	}
	return fallback
}

// View renders the grouped resolve screen or the inline edit field.
func (m EnvResolveModel) View() string {
	if m.editing {
		return m.viewEdit()
	}
	var lines []string
	for i, r := range m.rows {
		if r.header {
			if len(lines) > 0 {
				lines = append(lines, "") // blank line between files
			}
			lines = append(lines, styles.Indent+styles.Bold.Render(r.target)+"   "+
				styles.Muted.Render(r.source))
			continue
		}
		lines = append(lines, m.renderEntry(r, i == m.cursor))
	}
	return strings.Join(lines, "\n")
}

// renderEntry renders one key row with the same highlight as the worktree list and
// the confirm choices: a "▌▸" marker + a subtle full-row tint on the selected row
// (ANSI-free content so the tint fills every cell), and a matching 2-cell blank
// prefix on normal rows so the columns line up. A shared lead space seats the key
// at the same column as those lists.
func (m EnvResolveModel) renderEntry(r envRow, selected bool) string {
	if selected {
		block := " " + m.entryText(r, false)
		if pad := m.width - envRowPrefixWidth - PrintableWidth(block); pad > 0 {
			block += strings.Repeat(" ", pad)
		}
		return styles.SelectedMarker.Render("▌▸") + styles.ListItemTinted.Render(block)
	}
	return styles.Indent + " " + m.entryText(r, true)
}

// envRowPrefixWidth is the 2-cell marker/blank prefix width shared by the selected
// and normal rows (matches the SelectList row prefix).
const envRowPrefixWidth = 2

// entryText builds "KEY  status  current → proposed  [action]" for a row, styled
// (normal rows) or plain (the selected row).
func (m EnvResolveModel) entryText(r envRow, styled bool) string {
	key := fmt.Sprintf("%-20s", r.key)
	status := fmt.Sprintf("%-9s", statusWord(r))
	action, proposed := rowActionValue(r, styled)

	mid := proposed
	switch r.status {
	case domain.EnvKeyMissing:
		// no current value
	case domain.EnvKeyOrphan:
		mid = valText(r.current, styled)
	default:
		mid = valText(r.current, styled) + arrow(styled) + proposed
	}

	if styled {
		key = styles.Bold.Render(key)
		status = statusStyle(r.status).Render(status)
		action = actionStyleFor(r).Render("[" + action + "]")
	} else {
		action = "[" + action + "]"
	}
	return key + "  " + status + "  " + mid + "   " + action
}

func arrow(styled bool) string {
	if styled {
		return styles.Muted.Render(" → ")
	}
	return " → "
}

// statusWord is the short status label shown in the status column.
func statusWord(r envRow) string {
	switch r.status {
	case domain.EnvKeyConflict:
		return "conflict"
	case domain.EnvKeyMissing:
		return "missing"
	case domain.EnvKeyOrphan:
		return "orphan"
	default:
		return "add"
	}
}

func statusStyle(s domain.EnvKeyStatus) lipgloss.Style {
	switch s {
	case domain.EnvKeyConflict, domain.EnvKeyMissing:
		return styles.Warning
	default:
		return styles.Muted
	}
}

// rowActionValue returns the action label and the proposed value for a row.
func rowActionValue(r envRow, styled bool) (action, proposed string) {
	if r.useEdit {
		return "edit", valText(r.edited, styled)
	}
	switch r.options[r.sel].code {
	case optOverwrite:
		return r.options[r.sel].label, valText(r.resolved, styled)
	case optAdd:
		return "add", valText(r.resolved, styled)
	case optAccept:
		return "accept", valText(r.placeholder, styled)
	case optSkip:
		if r.isAdd {
			return "skip", muted("(not added)", styled)
		}
		return "skip", muted("(left missing)", styled)
	case optRemove:
		return "remove", ""
	default: // optKeep
		if r.status == domain.EnvKeyOrphan {
			return "keep", ""
		}
		return "keep", valText(r.current, styled)
	}
}

func valText(v string, styled bool) string {
	if v == "" {
		return muted("(empty)", styled)
	}
	return fmt.Sprintf("%q", v)
}

func muted(s string, styled bool) string {
	if styled {
		return styles.Muted.Render(s)
	}
	return s
}

func (m EnvResolveModel) viewEdit() string {
	r := m.rows[m.cursor]
	var b strings.Builder
	b.WriteString(styles.Indent + styles.Bold.Render("Edit "+r.key) + "\n\n")
	b.WriteString(styles.Indent + m.input.View())
	return b.String()
}

func (m EnvResolveModel) helpActions() []string {
	return []string{domain.HelpSetResolution, domain.HelpEditValue}
}

func (m EnvResolveModel) helpModal() string {
	if m.editing {
		return domain.EnvResolveEditHelp
	}
	return ""
}

// EnvResolveSummary is the breadcrumb summary for a completed resolve step.
func EnvResolveSummary(model any) string {
	m, ok := model.(EnvResolveModel)
	if !ok {
		return ""
	}
	over, filled, pruned, skipped := 0, 0, 0, 0
	for _, r := range m.rows {
		if r.header {
			continue
		}
		if r.useEdit {
			filled++
			continue
		}
		switch r.options[r.sel].code {
		case optOverwrite:
			over++
		case optAccept:
			filled++
		case optRemove:
			pruned++
		case optSkip:
			skipped++
		}
	}
	parts := make([]string, 0, 4)
	if over > 0 {
		parts = append(parts, fmt.Sprintf("%d overwritten", over))
	}
	if filled > 0 {
		parts = append(parts, fmt.Sprintf("%d filled", filled))
	}
	if pruned > 0 {
		parts = append(parts, fmt.Sprintf("%d pruned", pruned))
	}
	if skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", skipped))
	}
	if len(parts) == 0 {
		return "reviewed"
	}
	return strings.Join(parts, ", ")
}
