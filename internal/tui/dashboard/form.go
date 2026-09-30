package dashboard

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

type formRowKind int

const (
	formText formRowKind = iota
	formChoice
	formBlocker
	formButton
)

// formRow is one line of a form modal. It describes what to draw and what
// activating it answers; whether it is ticked lives in the modal, so a rebuild of
// the rows never drops what the user already lifted.
type formRow struct {
	kind    formRowKind
	stepKey string
	value   string
	label   string
	danger  bool
	confirm bool
}

func (r formRow) interactive() bool { return r.kind != formText }

type formReadyMsg struct {
	generation int
	rows       []formRow
	// answers is what the form on screen amounts to, defaults included: submitting
	// it is confirming the whole thing.
	answers flow.Answers
	err     error
}

type buildFormParams struct {
	Session flow.Session
	Chosen  flow.Answers
	// Title is the modal's own, so a step whose title repeats it does not have it
	// written twice.
	Title      string
	Generation int
}

func buildFormCmd(params buildFormParams) tea.Cmd {
	return func() tea.Msg {
		rows, answers, err := buildFormRows(params)
		return formReadyMsg{generation: params.Generation, rows: rows, answers: answers, err: err}
	}
}

// buildFormRows lays a whole session out at once. Each step is rendered against
// the answers of the ones before it, so a choice the user changes is reflected by
// what the later sections say.
func buildFormRows(params buildFormParams) ([]formRow, flow.Answers, error) {
	session, chosen := params.Session, params.Chosen
	answers := session.Presets
	var rows []formRow

	for _, step := range session.Steps {
		if _, preset := session.Presets.Get(step.Key); preset {
			continue
		}
		if step.Skip != nil {
			if skip, _ := step.Skip(answers); skip {
				continue
			}
		}

		content, err := stepContent(step, answers)
		if err != nil {
			return nil, flow.Answers{}, err
		}
		section, answer := formSection(formSectionParams{
			Title:   params.Title,
			Step:    step,
			Content: content,
			Chosen:  chosen,
			Answers: answers,
		})
		rows = append(rows, section...)
		answers = answers.With(step.Key, answer)
	}
	return rows, answers, nil
}

type formSectionParams struct {
	Step    flow.Step
	Content flow.StepContent
	Chosen  flow.Answers
	Answers flow.Answers
	// Title is the modal's own; a step heading that repeats it is left out.
	Title string
}

// formSection renders one step and reports the answer the later steps must read.
func formSection(params formSectionParams) ([]formRow, flow.Answer) {
	if params.Step.Kind == flow.StepRecap {
		rows := heading(headingParams{Content: withoutBlockerLines(params.Content), Title: params.Title})
		rows = append(rows, blockerRows(params.Content)...)
		buttons := confirmButtons(confirmButtonsParams{
			StepKey: params.Step.Key,
			Content: params.Content,
		})
		// The leading button is what the recap answers with until one is pressed, so
		// a submit that went through no button confirms what the form leads with.
		return append(rows, buttons...), flow.Answer{Value: buttons[0].value, Asked: true}
	}

	rows := heading(headingParams{Content: params.Content, Title: params.Title})

	value := selectedValue(params)
	for _, option := range params.Content.Options {
		if option.Separator {
			continue
		}
		rows = append(rows, formRow{
			kind:    formChoice,
			stepKey: params.Step.Key,
			value:   option.Value,
			label:   option.Label,
			danger:  option.Danger,
		})
	}
	return append(rows, formRow{kind: formText}), flow.Answer{Value: value, Asked: true}
}

// selectedValue is what the step is currently answered with: the user's own
// choice, else the answer the flow resolves to when nobody is asked — never one
// this surface picked for itself.
func selectedValue(params formSectionParams) string {
	if answer, ok := params.Chosen.Get(params.Step.Key); ok {
		return answer.Value
	}
	if params.Step.Resolve != nil {
		if answer, err := params.Step.Resolve(params.Answers); err == nil {
			return answer.Value
		}
	}
	return ""
}

// withoutBlockerLines drops from the prose what the acknowledgements below it
// already state, so a refusal is never read twice.
func withoutBlockerLines(content flow.StepContent) flow.StepContent {
	stated := make(map[string]bool, len(content.Blockers))
	for _, blocker := range content.Blockers {
		stated[blocker.Label] = true
	}

	var kept []string
	for _, line := range strings.Split(content.Description, "\n") {
		if stated[line] || (len(kept) == 0 && line == "") {
			continue
		}
		kept = append(kept, line)
	}
	content.Description = strings.Join(kept, "\n")
	return content
}

// heading gives a section's title the same blank line under it that a panel
// title gets, so the question and what it entails are not one block of text.
type headingParams struct {
	Content flow.StepContent
	// Title is the modal's own, which the box already draws above the body: a
	// step whose title is the same question would otherwise write it twice, one
	// blank line apart. modal.stepHeader leaves it out for the same reason.
	Title string
}

func heading(params headingParams) []formRow {
	content := params.Content
	var rows []formRow
	if content.Title != "" && content.Title != params.Title {
		rows = append(rows, formRow{kind: formText, label: content.Title})
		if content.Description != "" {
			rows = append(rows, formRow{kind: formText})
		}
	}
	for _, line := range strings.Split(content.Description, "\n") {
		rows = append(rows, formRow{kind: formText, label: line})
	}
	return append(rows, formRow{kind: formText})
}

// blockerRows turns each refusal into its own acknowledgement. Nothing is ticked
// for the user, and the buttons below them stay inert until they all are.
func blockerRows(content flow.StepContent) []formRow {
	if len(content.Blockers) == 0 {
		return nil
	}
	rows := []formRow{{kind: formText, label: domain.DashboardBlockersTitle}}
	for _, blocker := range content.Blockers {
		rows = append(rows, formRow{
			kind:  formBlocker,
			value: blocker.Key,
			label: blocker.Label,
		})
	}
	return append(rows, formRow{kind: formText})
}

type confirmButtonsParams struct {
	StepKey string
	Content flow.StepContent
}

// confirmButtons gives every outcome a recap names its own button, so a decision
// between two of them (keep local / force-push) can be taken either way — folding
// them into one button answers for the user and throws the rest away. Blockers
// are the one exception: they stand in the way of the dangerous outcome, which is
// then the only way through, and a harmless button beside it would confirm
// something else than what was acknowledged.
func confirmButtons(params confirmButtonsParams) []formRow {
	options := params.Content.Options
	blocked := len(params.Content.Blockers) > 0
	if option, ok := dangerOption(params.Content); ok && blocked {
		options = []flow.Option{option}
	}

	rows := make([]formRow, 0, len(options)+1)
	for _, option := range options {
		if option.Separator {
			continue
		}
		rows = append(rows, formRow{
			kind:    formButton,
			confirm: true,
			stepKey: params.StepKey,
			value:   option.Value,
			label:   option.Label,
			danger:  option.Danger || blocked,
		})
	}
	if len(rows) == 0 {
		rows = append(rows, formRow{
			kind:    formButton,
			confirm: true,
			stepKey: params.StepKey,
			label:   domain.DashboardConfirmLabel,
		})
	}
	return append(rows, formRow{kind: formButton, label: domain.WizardCancelLabel})
}

func dangerOption(content flow.StepContent) (flow.Option, bool) {
	for _, option := range content.Options {
		if option.Danger {
			return option, true
		}
	}
	return flow.Option{}, false
}

func (mo modal) applyForm(msg formReadyMsg) (modal, tea.Cmd) {
	if msg.generation != mo.generation {
		return mo, nil
	}
	if msg.err != nil {
		return mo.fail(msg.err)
	}
	mo.loading = ""
	mo.rows, mo.answers = msg.rows, msg.answers
	mo.focus = clampFocus(mo.rows, mo.focus)
	return mo, nil
}

func clampFocus(rows []formRow, focus int) int {
	for index := focus; index < len(rows); index++ {
		if rows[index].interactive() {
			return index
		}
	}
	for index := min(focus, len(rows)) - 1; index >= 0; index-- {
		if rows[index].interactive() {
			return index
		}
	}
	return 0
}

func (mo modal) updateForm(msg tea.KeyMsg) (modal, tea.Cmd) {
	switch msg.String() {
	case keyUp, keyVimUp:
		return mo.moveFocus(-1), nil
	case keyDown, keyVimDown:
		return mo.moveFocus(1), nil
	case keySpace, keyEnter:
		return mo.activate(mo.focus)
	case keyEscape:
		// In a stepper the rows are one step among others; backing out of them is
		// going back a question, not dropping the run.
		if mo.shape == modalStepper {
			return mo.back()
		}
		return mo.cancel()
	}
	return mo, nil
}

func (mo modal) moveFocus(delta int) modal {
	for index := mo.focus + delta; index >= 0 && index < len(mo.rows); index += delta {
		if mo.rows[index].interactive() {
			mo.focus = index
			return mo
		}
	}
	return mo
}

func (mo modal) activate(index int) (modal, tea.Cmd) {
	if index < 0 || index >= len(mo.rows) {
		return mo, nil
	}
	row := mo.rows[index]
	mo.focus = index

	switch row.kind {
	case formChoice:
		mo.chosen = mo.chosen.With(row.stepKey, flow.Answer{Value: row.value, Asked: true})
		mo.generation++
		return mo, buildFormCmd(buildFormParams{Session: mo.session, Chosen: mo.chosen, Title: mo.title, Generation: mo.generation})
	case formBlocker:
		return mo.lift(row.value), nil
	case formButton:
		if !row.confirm {
			return mo.cancel()
		}
		if !mo.blockersLifted() {
			return mo, nil
		}
		if mo.shape == modalStepper {
			return mo.answer(row.value)
		}
		// A recap offers one button per outcome, so the whole form is submitted
		// answered with the one that was pressed — never with the one it led with.
		return mo.submit(mo.answers.With(row.stepKey, flow.Answer{Value: row.value, Asked: true}))
	}
	return mo, nil
}

// lift copies the acknowledgements: the model is passed by value, and a shared
// map would tick a box in frames the user has already left behind.
func (mo modal) lift(key string) modal {
	lifted := make(map[string]bool, len(mo.lifted)+1)
	for k, v := range mo.lifted {
		lifted[k] = v
	}
	lifted[key] = !lifted[key]
	mo.lifted = lifted
	return mo
}

// hasToggles reports whether anything in the form is toggled rather than
// pressed, which is what the hint line has to teach.
func (mo modal) hasToggles() bool {
	for _, row := range mo.rows {
		if row.kind == formBlocker || row.kind == formChoice {
			return true
		}
	}
	return false
}

func (mo modal) blockersLifted() bool {
	for _, row := range mo.rows {
		if row.kind == formBlocker && !mo.lifted[row.value] {
			return false
		}
	}
	return true
}

func (mo modal) formBody(zones marker) []string {
	lines := make([]string, 0, len(mo.rows))
	for index, row := range mo.rows {
		line := mo.renderRow(index, row)
		if row.interactive() {
			line = zones.Mark(modalRowZone(index), line)
		}
		lines = append(lines, line)
	}
	return lines
}

func (mo modal) renderRow(index int, row formRow) string {
	switch row.kind {
	case formText:
		return mo.renderText(row.label)
	case formButton:
		return mo.renderButton(index, row)
	case formChoice:
		return mo.renderFocusable(index, glyphChoice(mo.answers.Value(row.stepKey) == row.value)+" "+row.label, styles.DashboardRow)
	}
	return mo.renderFocusable(index, glyphCheck(mo.lifted[row.value])+" "+row.label, styles.DashboardRow)
}

// renderText weights a recap field over its label, so a block of them reads as
// values with names rather than as a paragraph. Anything that is not a field is
// left exactly as the flow wrote it.
func (mo modal) renderText(text string) string {
	label, value, ok := rules.SplitRecapField(text)
	if !ok {
		return truncate(text, mo.bodyWidth())
	}
	return styles.DashboardLabel.Render(label) +
		styles.DashboardRecapValue.Render(truncate(value, max(mo.bodyWidth()-lipgloss.Width(label), 0)))
}

// renderButton keeps an action recognizable by its brackets rather than by a
// filled block: it sits in the same focus ring as the rows above it, and a
// colored slab there reads as noise. The one filled button on the dashboard is
// the header's, which stands alone.
func (mo modal) renderButton(index int, row formRow) string {
	label, style := fmt.Sprintf(domain.DashboardButtonFmt, row.label), styles.DashboardRow
	switch {
	case row.confirm && !mo.blockersLifted():
		label, style = label+domain.DashboardBlockedSuffix, styles.DashboardDisabled
	case row.danger:
		style = styles.DashboardDanger
	}
	return mo.renderFocusable(index, label, style)
}

// renderFocusable draws a modal row with the dashboard's one focus language: the
// accent bar and a tinted span, exactly what the worktree list does, so what the
// keyboard is on reads the same everywhere.
func (mo modal) renderFocusable(index int, text string, style lipgloss.Style) string {
	inner := max(mo.bodyWidth()-rowBarWidth, 0)
	if index != mo.focus {
		return rowIndent + style.Render(truncate(text, inner))
	}
	return styles.DashboardRowBar.Render(rowBar+" ") +
		styles.DashboardRowSelected.Width(inner).Render(truncate(text, inner))
}

func glyphChoice(selected bool) string {
	if selected {
		return domain.DashboardGlyphChoiceOn
	}
	return domain.DashboardGlyphChoiceOff
}

func glyphCheck(checked bool) string {
	if checked {
		return domain.DashboardGlyphCheckOn
	}
	return domain.DashboardGlyphCheckOff
}
