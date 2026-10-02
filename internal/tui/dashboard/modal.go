package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
	"github.com/LucasPcq/wtm/internal/tui/flowui"
	tea "github.com/charmbracelet/bubbletea"
)

// modalPadding is the horizontal padding of the modal box, modalMargin the rows
// it leaves to the frame underneath so a question never reads as a new page.
const (
	modalPadding = 2
	modalMargin  = 4
)

// marker registers a clickable region on rendered content. The zone manager
// implements it; a test can pass its own.
type marker interface {
	Mark(id, content string) string
}

// modalShape is how a session is laid out, not what it asks: a stepper walks the
// questions one at a time like the CLI wizard, a form puts a whole session on
// screen at once for a decision the user takes in one go.
type modalShape int

const (
	modalStepper modalShape = iota
	modalForm
)

// promptReply carries the session's answers back to the flow goroutine blocked in
// Prompter.Ask.
type promptReply struct {
	answers flow.Answers
	err     error
}

type modalParams struct {
	Title   string
	Shape   modalShape
	Session flow.Session
	Reply   chan<- promptReply
	Width   int
	Height  int
}

type modal struct {
	open    bool
	shape   modalShape
	title   string
	session flow.Session
	answers flow.Answers
	reply   chan<- promptReply

	width  int
	height int

	index    int
	kind     flow.StepKind
	content  flow.StepContent
	text     components.TextInputModel
	list     components.SelectListModel
	multi    components.MultiSelectModel
	textList components.TextListModel
	reorder  components.ReorderListModel

	rows   []formRow
	focus  int
	chosen flow.Answers
	// lifted holds the refusals the user acknowledged, keyed by blocker, so a
	// rebuild of the rows never silently un-ticks one.
	lifted     map[string]bool
	generation int

	loading string
}

type modalLoadedMsg struct {
	index   int
	content flow.StepContent
	err     error
}

func newModal(params modalParams) (modal, tea.Cmd) {
	mo := modal{
		open:    true,
		shape:   params.Shape,
		title:   params.Title,
		session: params.Session,
		answers: params.Session.Presets,
		chosen:  params.Session.Presets,
		reply:   params.Reply,
		width:   params.Width,
		height:  params.Height,
		index:   -1,
	}
	if params.Shape == modalForm {
		mo.loading = domain.DashboardModalPreparing
		return mo, buildFormCmd(buildFormParams{Session: mo.session, Chosen: mo.chosen, Title: mo.title, Generation: mo.generation})
	}
	return mo.advance()
}

func (mo modal) resize(width, height int) modal {
	mo.width, mo.height = width, height
	mo.list.SetSize(components.SetSizeParams{Width: mo.bodyWidth(), Height: mo.bodyHeight()})
	mo.multi.SetSize(components.SetSizeParams{Width: mo.bodyWidth(), Height: mo.bodyHeight()})
	mo.text.SetWidth(mo.bodyWidth())
	mo.textList.SetWidth(mo.bodyWidth())
	return mo
}

func (mo modal) bodyWidth() int {
	return max(rules.ModalWidth(mo.width)-domain.DashboardModalChrome-modalPadding, 1)
}

// bodyHeight leaves the surrounding frame visible: a modal that filled the screen
// would read as a new page rather than as a question over the dashboard.
func (mo modal) bodyHeight() int {
	return max(mo.height-domain.DashboardModalChrome-modalMargin, 1)
}

// A preset is the only thing this surface may pass over on its own: it was
// answered before the session started and is not a question. An answer the user
// gave here is one they may come back to, and a skip is a question the answers
// removed — both have to be reconsidered on the way forward, since what decided
// them is exactly what stepping back changes. components.WizardModel.goBack
// clears its skip flags for the same reason.
func (mo modal) advance() (modal, tea.Cmd) {
	for index := mo.index + 1; index < len(mo.session.Steps); index++ {
		step := mo.session.Steps[index]
		if answer, known := mo.answers.Get(step.Key); known && !answer.Asked && !answer.Skipped {
			continue
		}
		if step.Skip != nil {
			if skip, reason := step.Skip(mo.answers); skip {
				mo.answers = mo.answers.With(step.Key, flow.Answer{Skipped: true, SkipReason: reason})
				continue
			}
		}
		mo.index = index
		return mo.enter(step)
	}
	return mo.submit(mo.answers)
}

func (mo modal) enter(step flow.Step) (modal, tea.Cmd) {
	mo.kind = step.Kind
	if step.Load != nil {
		mo.loading = loadingMessage(step)
		return mo, loadStepCmd(mo.index, step, mo.answers)
	}
	content, err := stepContent(step, mo.answers)
	if err != nil {
		return mo.fail(err)
	}
	return mo.show(step, content)
}

func (mo modal) show(step flow.Step, content flow.StepContent) (modal, tea.Cmd) {
	mo.loading, mo.content = "", content
	switch step.Kind {
	case flow.StepText:
		mo.text = components.NewTextInput(components.NewTextInputParams{
			Title:       content.Title,
			Description: content.Description,
			Default:     content.Default,
			Validate:    step.Validate,
		})
		mo.text.SetWidth(mo.bodyWidth())
		return mo, mo.text.Init()
	case flow.StepRecap:
		// A recap is a confirmation, so it is drawn with the buttons every other
		// confirmation uses rather than as one more list to pick from.
		mo.rows, _ = formSection(formSectionParams{Step: step, Content: content, Answers: mo.answers, Chosen: mo.answers, Title: mo.title})
		mo.focus = clampFocus(mo.rows, 0)
		return mo, nil
	case flow.StepSelect:
		mo.list = newSelectList(content)
	case flow.StepBranchSelect:
		mo.list = components.NewSelectList(components.NewSelectListParams{
			Title:       content.Title,
			Description: content.Description,
			Items:       branchItems(step, content),
		})
	case flow.StepTextList:
		mo.textList = flowui.TextList(step, mo.reentries(step, content))
		mo.textList.SetWidth(mo.bodyWidth())
		return mo, mo.textList.Init()
	case flow.StepMultiSelect:
		mo.multi = newMultiSelect(step, mo.reselect(step, content))
		mo.multi.SetSize(components.SetSizeParams{Width: mo.bodyWidth(), Height: mo.bodyHeight()})
		return mo, mo.multi.Init()
	case flow.StepReorder:
		mo.reorder = newReorderList(content)
		return mo, mo.reorder.Init()
	default:
		return mo.fail(fmt.Errorf(domain.DashboardUnsupportedStepFmt, step.Key, step.Kind))
	}
	mo.list.SetSize(components.SetSizeParams{Width: mo.bodyWidth(), Height: mo.bodyHeight()})
	return mo, nil
}

// back returns to the previous question. A step answered from a preset or skipped
// was never asked, so it is not a place the user can go back to.
func (mo modal) back() (modal, tea.Cmd) {
	for index := mo.index - 1; index >= 0; index-- {
		step := mo.session.Steps[index]
		if answer, known := mo.answers.Get(step.Key); !known || !answer.Asked {
			continue
		}
		mo.index = index
		return mo.enter(step)
	}
	return mo.cancel()
}

func (mo modal) submit(answers flow.Answers) (modal, tea.Cmd) {
	return mo.close(), replyCmd(mo.reply, promptReply{answers: answers})
}

func (mo modal) cancel() (modal, tea.Cmd) {
	return mo.close(), replyCmd(mo.reply, promptReply{err: domain.ErrUserAborted})
}

func (mo modal) fail(err error) (modal, tea.Cmd) {
	return mo.close(), replyCmd(mo.reply, promptReply{err: err})
}

func (mo modal) close() modal { return modal{} }

func (mo modal) update(msg tea.Msg) (modal, tea.Cmd) {
	switch msg := msg.(type) {
	case modalLoadedMsg:
		return mo.applyLoaded(msg)
	case formReadyMsg:
		return mo.applyForm(msg)
	case tea.KeyMsg:
		// A modal waiting on I/O has nothing to answer with yet, and the flow it
		// would answer is busy producing it.
		if mo.loading != "" {
			return mo, nil
		}
		return mo.updateStepper(msg)
	}
	return mo, nil
}

func (mo modal) applyLoaded(msg modalLoadedMsg) (modal, tea.Cmd) {
	if msg.index != mo.index {
		return mo, nil
	}
	if msg.err != nil {
		return mo.fail(msg.err)
	}
	return mo.show(mo.session.Steps[msg.index], msg.content)
}

func (mo modal) updateStepper(msg tea.KeyMsg) (modal, tea.Cmd) {
	if mo.usesRows() {
		return mo.updateForm(msg)
	}
	if mo.kind == flow.StepMultiSelect {
		var cmd tea.Cmd
		mo.multi, cmd = mo.multi.Update(msg)
		switch {
		case mo.multi.Aborted():
			return mo.back()
		case mo.multi.Done():
			return mo.answerValues(mo.multi.Values())
		}
		return mo, cmd
	}

	if mo.kind == flow.StepTextList {
		var cmd tea.Cmd
		mo.textList, cmd = mo.textList.Update(msg)
		switch {
		case mo.textList.Aborted():
			return mo.back()
		case mo.textList.Done():
			return mo.answerValues(mo.textList.Values())
		}
		return mo, cmd
	}

	if mo.kind == flow.StepReorder {
		var cmd tea.Cmd
		mo.reorder, cmd = mo.reorder.Update(msg)
		switch {
		case mo.reorder.Aborted():
			return mo.back()
		case mo.reorder.Done():
			return mo.answerValues(mo.reorder.Values())
		}
		return mo, cmd
	}

	if mo.kind == flow.StepText {
		var cmd tea.Cmd
		mo.text, cmd = mo.text.Update(msg)
		switch {
		case mo.text.Aborted():
			return mo.back()
		case mo.text.Done():
			return mo.answer(mo.text.Value())
		}
		return mo, cmd
	}

	var cmd tea.Cmd
	mo.list, cmd = mo.list.Update(msg)
	switch {
	case mo.list.Aborted():
		return mo.back()
	case mo.list.Chosen():
		if value := mo.list.Value(); value != domain.WizardCancelValue {
			return mo.answer(value)
		}
		return mo.cancel()
	}
	return mo, cmd
}

func (mo modal) answer(value string) (modal, tea.Cmd) {
	mo.answers = mo.answers.With(mo.session.Steps[mo.index].Key, flow.Answer{Value: value, Asked: true})
	return mo.advance()
}

func (mo modal) answerValues(values []string) (modal, tea.Cmd) {
	mo.answers = mo.answers.With(mo.session.Steps[mo.index].Key, flow.Answer{Values: values, Asked: true})
	return mo.advance()
}

// usesRows reports whether the modal is showing its own rows rather than a
// widget: a form always is, a stepper is on its recap.
func (mo modal) usesRows() bool {
	return mo.shape == modalForm || mo.kind == flow.StepRecap
}

// reselect re-applies what the user already checked onto a rebuilt step. The
// content's own Selected flags are the flow's opening proposal, which must not
// overwrite an edit made before stepping back — a step re-entered with its boxes
// reset silently discards the answer the user came back to adjust.
func (mo modal) reselect(step flow.Step, content flow.StepContent) flow.StepContent {
	answer, known := mo.answers.Get(step.Key)
	if !known || !answer.Asked {
		return content
	}
	checked := make(map[string]bool, len(answer.Values))
	for _, value := range answer.Values {
		checked[value] = true
	}
	options := append([]flow.Option(nil), content.Options...)
	for index := range options {
		options[index].Selected = checked[options[index].Value]
	}
	content.Options = options
	return content
}

// reentries puts back the list the user typed before stepping forward: Build
// only knows the entries the request carried.
func (mo modal) reentries(step flow.Step, content flow.StepContent) flow.StepContent {
	if answer, known := mo.answers.Get(step.Key); known && answer.Asked {
		content.Entries = answer.Values
	}
	return content
}

func newMultiSelect(step flow.Step, content flow.StepContent) components.MultiSelectModel {
	items := make([]components.MultiSelectItem, 0, len(content.Options))
	for _, option := range content.Options {
		if option.Separator {
			continue
		}
		items = append(items, components.MultiSelectItem{
			Label:    option.Label,
			Value:    option.Value,
			Selected: option.Selected,
			Tag:      option.Tag,
			Variant:  components.TagVariantOf(option.Tone),
			Badges:   selectBadges(option.Badges),
		})
	}
	return components.NewMultiSelect(components.NewMultiSelectParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       items,
		Validate:    step.ValidateSet,
		Start:       content.Start,
	})
}

func newSelectList(content flow.StepContent) components.SelectListModel {
	items := make([]components.SelectItem, 0, len(content.Options))
	for _, option := range content.Options {
		items = append(items, components.SelectItem{
			Label:     option.Label,
			Value:     option.Value,
			Separator: option.Separator,
			Danger:    option.Danger,
			Badges:    selectBadges(option.Badges),
		})
	}
	return components.NewSelectList(components.NewSelectListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       items,
		Start:       content.Start,
	})
}

func selectBadges(badges []flow.Badge) []components.Badge {
	if len(badges) == 0 {
		return nil
	}
	rendered := make([]components.Badge, 0, len(badges))
	for _, badge := range badges {
		rendered = append(rendered, components.Badge{
			Text:    badge.Text,
			Variant: components.BadgeVariantOf(badge.Tone),
		})
	}
	return rendered
}

// branchItems applies the content's exclusions over the step's candidates, the
// same way flowui does — a narrowed picker must show the same list on both
// surfaces.
func branchItems(step flow.Step, content flow.StepContent) []components.SelectItem {
	candidates := flow.KeepBranches(step.Branches, content.ExcludeBranches)
	pinned := ""
	for _, candidate := range candidates {
		if candidate.Name == step.Pinned {
			pinned = step.Pinned
			break
		}
	}
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   candidates,
		Pinned:       pinned,
		PinnedSuffix: domain.PinnedSuffixDefault,
	})
}

// stepContent merges what a step declares statically with what it derives from
// the answers, so a Build only returns the parts that change.
func stepContent(step flow.Step, answers flow.Answers) (flow.StepContent, error) {
	build := step.Build
	if build == nil {
		build = step.Load
	}
	if build == nil {
		return flow.MergeContent(step, flow.StepContent{}), nil
	}
	built, err := build(answers)
	if err != nil {
		return flow.StepContent{}, err
	}
	return flow.MergeContent(step, built), nil
}

func loadingMessage(step flow.Step) string {
	if step.LoadingMessage != "" {
		return step.LoadingMessage
	}
	return domain.DashboardModalPreparing
}

func loadStepCmd(index int, step flow.Step, answers flow.Answers) tea.Cmd {
	return func() tea.Msg {
		content, err := step.Load(answers)
		return modalLoadedMsg{index: index, content: content, err: err}
	}
}

func replyCmd(reply chan<- promptReply, answer promptReply) tea.Cmd {
	return func() tea.Msg {
		reply <- answer
		return nil
	}
}

// box renders the modal and says where it goes; View pastes it over the frame,
// so the dashboard stays visible around the question.
func (mo modal) box(zones marker) (string, domain.Rect) {
	body := mo.body(zones)
	rect := rules.ComputeModalRect(rules.ModalRectParams{
		ScreenWidth:   mo.width,
		ScreenHeight:  mo.height,
		ContentHeight: len(body),
	})
	if rect.Width <= domain.DashboardModalChrome || rect.Height <= domain.DashboardModalChrome {
		return "", domain.Rect{}
	}

	lines := body
	if height := rect.Height - domain.DashboardModalChrome; len(lines) > height {
		lines = lines[:height]
	}
	return styles.DashboardModal.
		Width(rect.Width - domain.DashboardModalChrome).
		Render(strings.Join(lines, "\n")), rect
}

func (mo modal) body(zones marker) []string {
	lines := []string{styles.DashboardModalTitle.Render(truncate(mo.title, mo.bodyWidth())), ""}
	switch {
	case mo.loading != "":
		lines = append(lines, styles.DashboardEmpty.Render(truncate(mo.loading, mo.bodyWidth())))
	case mo.usesRows():
		lines = append(lines, mo.formBody(zones)...)
	case mo.kind == flow.StepText:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.text.View(), "\n")...)
	case mo.kind == flow.StepTextList:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.textList.View(), "\n")...)
	case mo.kind == flow.StepMultiSelect:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.multi.View(), "\n")...)
	case mo.kind == flow.StepReorder:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.reorder.View(), "\n")...)
	default:
		lines = append(lines, mo.stepHeader()...)
		lines = append(lines, strings.Split(mo.list.View(), "\n")...)
	}
	return append(lines, "", styles.DashboardModalHint.Render(truncate(mo.hint(), mo.bodyWidth())))
}

// stepHeader carries what the widgets do not draw themselves: the question and
// what it entails. The CLI wizard renders the same two around its own steps.
func (mo modal) stepHeader() []string {
	var lines []string
	if mo.content.Title != "" && mo.content.Title != mo.title {
		lines = append(lines, styles.Bold.Render(truncate(mo.content.Title, mo.bodyWidth())))
		// The question and what it entails are two different things to read; run
		// together they read as one wrapped sentence.
		if mo.content.Description != "" {
			lines = append(lines, "")
		}
	}
	for _, line := range strings.Split(mo.content.Description, "\n") {
		if mo.content.Description == "" {
			break
		}
		lines = append(lines, styles.Muted.Render(truncate(line, mo.bodyWidth())))
	}
	if len(lines) == 0 {
		return nil
	}
	return append(lines, "")
}

func (mo modal) hint() string {
	switch {
	case mo.usesRows() && mo.shape == modalStepper:
		return domain.DashboardStepperRowsHint
	case mo.usesRows() && mo.hasToggles():
		return domain.DashboardFormHint
	case mo.usesRows():
		return domain.DashboardConfirmHint
	case mo.kind == flow.StepText:
		return domain.DashboardStepperTextHint
	case mo.kind == flow.StepTextList:
		return domain.DashboardStepperListHint
	case mo.kind == flow.StepMultiSelect:
		return domain.DashboardStepperMultiHint
	case mo.kind == flow.StepReorder:
		return domain.DashboardStepperReorderHint
	}
	return domain.DashboardStepperHint
}

// newReorderList draws a step whose options are already the answer: what it
// collects is the sequence they end up in.
func newReorderList(content flow.StepContent) components.ReorderListModel {
	items := make([]components.ReorderItem, 0, len(content.Options))
	for _, option := range content.Options {
		if option.Separator {
			continue
		}
		items = append(items, components.ReorderItem{Label: option.Label, Value: option.Value})
	}
	return components.NewReorderList(components.NewReorderListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       items,
	})
}
