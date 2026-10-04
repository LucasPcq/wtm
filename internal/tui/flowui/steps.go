package flowui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

type (
	loadRequestMsg struct {
		idx     int
		answers flow.Answers
	}
	loadDoneMsg struct {
		idx     int
		content flow.StepContent
		err     error
	}
)

func (p *plan) componentStep(step flow.Step, conditional bool) (components.Step, error) {
	if conditional {
		// A select is the one kind whose whole model can be rebuilt from its
		// answer, so its Skip is folded into the list it draws. Every other kind
		// owns its model — a recap owns its cancel row and the plan its Load
		// fills in, a multi-select owns its checked set — so its Skip gates the
		// step instead of replacing it.
		if step.Kind == flow.StepSelect && step.Load == nil {
			return p.choiceStep(step), nil
		}
		built, err := p.componentStep(step, false)
		if err != nil {
			return components.Step{}, err
		}
		return p.gated(step, built), nil
	}
	switch step.Kind {
	case flow.StepText:
		return p.contentStep(step, func(content flow.StepContent) any { return textInput(step, content) })
	case flow.StepSelect:
		if step.Load != nil {
			return p.loadedSelectStep(step), nil
		}
		return p.contentStep(step, func(content flow.StepContent) any { return selectList(content) })
	case flow.StepBranchSelect:
		return p.branchStep(step)
	case flow.StepTextList:
		return p.contentStep(step, func(content flow.StepContent) any { return TextList(step, content) })
	case flow.StepMultiSelect:
		if step.Load != nil {
			return p.loadedMultiSelectStep(step), nil
		}
		return p.contentStep(step, func(content flow.StepContent) any { return multiSelect(step, content) })
	case flow.StepReorder:
		return p.contentStep(step, func(content flow.StepContent) any { return reorderList(content) })
	case flow.StepRecap:
		return p.recapStep(step), nil
	case flow.StepEnvResolve:
		built, err := p.contentStep(step, func(content flow.StepContent) any { return envResolve(content) })
		// The glossary the model carries as its description reads as a legend.
		built.Callout = true
		return built, err
	}
	return components.Step{}, unsupportedKindErr(step)
}

// contentStep is every kind whose whole model is rebuilt from its content: the
// four differ only in which widget they hand that content to.
func (p *plan) contentStep(step flow.Step, model func(flow.StepContent) any) (components.Step, error) {
	content, err := p.content(step, p.known())
	if err != nil {
		return components.Step{}, err
	}
	built := components.Step{
		Name:    step.Label,
		Model:   model(content),
		Summary: summaryFor(step),
	}
	if step.Build != nil {
		built.Build = func(prev []components.Step) any { return model(p.rebuild(step, prev)) }
	}
	return built, nil
}

func textInput(step flow.Step, content flow.StepContent) components.TextInputModel {
	return components.NewTextInput(components.NewTextInputParams{
		Title:       content.Title,
		Description: content.Description,
		Default:     content.Default,
		Validate:    step.Validate,
	})
}

// TextList is shared with the dashboard, so an entry is refused and badged the
// same way on both surfaces.
func TextList(step flow.Step, content flow.StepContent) components.TextListModel {
	params := components.NewTextListParams{
		Title:       content.Title,
		Description: content.Description,
		Entries:     content.Entries,
		Required:    domain.FlowEntryRequired,
		Check: func(entry components.TextListEntry) (string, error) {
			return flow.CheckEntry(step, flow.EntryCheck{Entry: entry.Entry, Entries: entry.Entries})
		},
	}
	if step.EntryBadge != nil {
		params.Badge = func(entry string) components.Badge {
			return toBadges([]flow.Badge{step.EntryBadge(entry)})[0]
		}
	}
	return components.NewTextList(params)
}

func envResolve(content flow.StepContent) components.EnvResolveModel {
	return components.NewEnvResolve(components.NewEnvResolveParams{
		Title:       content.Title,
		Description: components.EnvResolveGlossary(),
		Files:       content.EnvFiles,
		Defaults:    content.EnvDefaults,
	})
}

func reorderList(content flow.StepContent) components.ReorderListModel {
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

func multiSelect(step flow.Step, content flow.StepContent) components.MultiSelectModel {
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
			Badges:   toBadges(option.Badges),
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

func (p *plan) branchStep(step flow.Step) (components.Step, error) {
	p.candidates = step.Branches
	p.refresh = step.Refresh

	model := func(answers flow.Answers) (any, error) {
		content, err := p.content(step, answers)
		if err != nil {
			return nil, err
		}
		return components.NewSelectList(components.NewSelectListParams{
			Title:       content.Title,
			Description: content.Description,
			Items:       p.branchItems(step, content),
		}), nil
	}

	initial, err := model(p.known())
	if err != nil {
		return components.Step{}, err
	}
	built := components.Step{
		Name:  step.Label,
		Model: initial,
		Build: func(prev []components.Step) any {
			rebuilt, buildErr := model(p.answersFrom(prev))
			if buildErr != nil {
				p.loadErr = buildErr
				return placeholder(step)
			}
			return rebuilt
		},
		CanRefresh: step.Refresh != nil,
		Summary:    summaryFor(step),
	}
	if step.Refresh != nil {
		p.initCmd = tea.Batch(p.initCmd, branchrefresh.CmdFunc(step.Refresh))
		if p.loadingText == "" {
			p.loadingText = domain.LoadingBranchesText
		}
	}
	return built, nil
}

// branchItems applies the step's exclusions over whatever the refresh last
// returned, so narrowing and refreshing do not fight over the same list.
func (p *plan) branchItems(step flow.Step, content flow.StepContent) []components.SelectItem {
	candidates := flow.KeepBranches(p.candidates, content.ExcludeBranches)
	return components.BranchItems(components.BranchItemsParams{
		Candidates:   candidates,
		Pinned:       flow.PinnedAmong(step, content, candidates),
		PinnedSuffix: flow.PinnedSuffix(step),
	})
}

// gated turns a step's Skip into the wizard's entry-time decision while the step
// keeps the model its kind built. The wizard runs Build immediately before
// AutoSkip on every advance, so the decision is refreshed there.
func (p *plan) gated(step flow.Step, built components.Step) components.Step {
	applies := true
	reason := ""
	model := built.Model
	rebuild := built.Build
	built.Build = func(prev []components.Step) any {
		skip, why := step.Skip(p.answersFrom(prev))
		applies, reason = !skip, why
		if rebuild != nil {
			return rebuild(prev)
		}
		return model
	}
	built.AutoSkip = func(components.WizardModel) bool { return !applies }
	built.SkipReason = func() string { return reason }
	return built
}

func (p *plan) choiceStep(step flow.Step) components.Step {
	return components.ChoiceStep(components.ChoiceStepParams{
		Name:    step.Label,
		Summary: summaryFor(step),
		Decide: func(prev []components.Step) (bool, string, components.NewSelectListParams) {
			if skip, reason := step.Skip(p.answersFrom(prev)); skip {
				return false, reason, components.NewSelectListParams{}
			}
			content := p.rebuild(step, prev)
			return true, "", components.NewSelectListParams{
				Title:       content.Title,
				Description: content.Description,
				Items:       toItems(content.Options),
				Start:       content.Start,
			}
		},
	})
}

func (p *plan) recapStep(step flow.Step) components.Step {
	if step.Load != nil {
		return p.loadedRecapStep(step)
	}
	build := func(answers flow.Answers) any {
		content, err := p.content(step, answers)
		if err != nil {
			p.loadErr = err
			return placeholder(step)
		}
		return recapList(content)
	}
	return components.Step{
		Name:    step.Label,
		Model:   build(p.known()),
		Recap:   true,
		Summary: summaryFor(step),
		Build:   func(prev []components.Step) any { return build(p.answersFrom(prev)) },
	}
}

// loadedRecapStep shows an empty recap — where Enter is a no-op — until the loaded
// body replaces it, so a run is never confirmed before its consequences are visible.
func (p *plan) loadedRecapStep(step flow.Step) components.Step {
	built := p.loadedStep(loadedStep{step: step, placeholder: placeholder(step), model: func(content flow.StepContent) any {
		return recapList(content)
	}})
	built.Recap = true
	return built
}

// loadedSelectStep draws the step's title and description with no option until
// its options arrive, so the wizard is on screen before a slow listing answers.
func (p *plan) loadedSelectStep(step flow.Step) components.Step {
	return p.loadedStep(loadedStep{
		step:        step,
		placeholder: selectList(flow.MergeContent(step, flow.StepContent{})),
		model: func(content flow.StepContent) any {
			return selectList(flow.MergeContent(step, content))
		},
	})
}

// loadedMultiSelectStep holds an empty set until the options arrive: the step's
// ValidateSet refuses it, so Enter cannot answer a list nobody has seen yet.
func (p *plan) loadedMultiSelectStep(step flow.Step) components.Step {
	return p.loadedStep(loadedStep{
		step:        step,
		placeholder: multiSelect(step, flow.MergeContent(step, flow.StepContent{})),
		model: func(content flow.StepContent) any {
			return multiSelect(step, flow.MergeContent(step, content))
		},
	})
}

type loadedStep struct {
	step        flow.Step
	placeholder any
	model       func(flow.StepContent) any
}

func (p *plan) loadedStep(loaded loadedStep) components.Step {
	step := loaded.step
	idx := len(p.steps)
	if p.loads == nil {
		p.loads = map[int]loadedStep{}
	}
	p.loads[idx] = loaded

	// The wizard runs OnEnter on every step it advances to, but never on the one it
	// starts on, so a load landing first is fired from the init command instead —
	// otherwise the run would sit on the placeholder for ever.
	if idx == 0 {
		p.initCmd = tea.Batch(p.initCmd, func() tea.Msg {
			return loadRequestMsg{idx: idx, answers: p.known()}
		})
		p.loadingText = step.LoadingMessage
	}

	return components.Step{
		Name:    step.Label,
		Model:   loaded.placeholder,
		Summary: summaryFor(step),
		OnEnter: func(prev []components.Step) tea.Cmd {
			answers := p.answersFrom(prev)
			return func() tea.Msg { return loadRequestMsg{idx: idx, answers: answers} }
		},
	}
}

func (p *plan) handler() components.WizardMsgHandler {
	var handlers []components.WizardMsgHandler
	if p.refresh != nil {
		handlers = append(handlers, branchrefresh.HandlerFunc(p.refresh, &p.candidates))
	}
	if len(p.loads) > 0 {
		handlers = append(handlers, p.loadHandler())
	}
	return combine(handlers...)
}

func (p *plan) loadHandler() components.WizardMsgHandler {
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		switch m := msg.(type) {
		case loadRequestMsg:
			loaded, ok := p.loads[m.idx]
			if !ok {
				return nil, false
			}
			w.UpdateStepModel(m.idx, func(any) any { return loaded.placeholder })
			return tea.Batch(w.StartLoading(loaded.step.LoadingMessage), runLoad(m.idx, loaded.step, m.answers)), true
		case loadDoneMsg:
			loaded, ok := p.loads[m.idx]
			if !ok {
				return nil, false
			}
			content := m.content
			if m.err != nil {
				p.loadErr = m.err
				content = flow.StepContent{Title: loaded.step.Title, Description: m.err.Error()}
			}
			w.UpdateStepModel(m.idx, func(any) any { return loaded.model(content) })
			w.SetLoading(false)
			w.SetBanner(components.WizardBanner{Title: content.Banner.Title, Lines: content.Banner.Lines})
			return nil, true
		}
		return nil, false
	}
}

func runLoad(idx int, step flow.Step, answers flow.Answers) tea.Cmd {
	return func() tea.Msg {
		content, err := step.Load(answers)
		return loadDoneMsg{idx: idx, content: content, err: err}
	}
}

func combine(handlers ...components.WizardMsgHandler) components.WizardMsgHandler {
	if len(handlers) == 0 {
		return nil
	}
	return func(w *components.WizardModel, msg tea.Msg) (tea.Cmd, bool) {
		for _, handle := range handlers {
			if cmd, handled := handle(w, msg); handled {
				return cmd, true
			}
		}
		return nil, false
	}
}

// content merges what a step declares statically with what it derives from the
// answers, so a Build only returns the parts that change.
func (p *plan) content(step flow.Step, answers flow.Answers) (flow.StepContent, error) {
	if step.Build == nil {
		return flow.MergeContent(step, flow.StepContent{}), nil
	}
	built, err := step.Build(answers)
	if err != nil {
		return flow.StepContent{}, err
	}
	return flow.MergeContent(step, built), nil
}

func (p *plan) rebuild(step flow.Step, prev []components.Step) flow.StepContent {
	content, err := p.content(step, p.answersFrom(prev))
	if err != nil {
		p.loadErr = err
		return flow.StepContent{Title: step.Title, Description: step.Description}
	}
	return content
}

func selectList(content flow.StepContent) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       toItems(content.Options),
		Start:       content.Start,
	})
}

func recapList(content flow.StepContent) components.SelectListModel {
	items := append(toItems(content.Options),
		components.SelectItem{Separator: true},
		components.SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue},
	)
	return components.NewSelectList(components.NewSelectListParams{
		Title:       content.Title,
		Description: content.Description,
		Items:       items,
	})
}

func placeholder(step flow.Step) components.SelectListModel {
	return components.NewSelectList(components.NewSelectListParams{Title: step.Title})
}

func toItems(options []flow.Option) []components.SelectItem {
	items := make([]components.SelectItem, 0, len(options))
	for _, option := range options {
		items = append(items, components.SelectItem{
			Label:     option.Label,
			Value:     option.Value,
			Separator: option.Separator,
			Danger:    option.Danger,
			Disabled:  option.Disabled,
			Badges:    toBadges(option.Badges),
		})
	}
	return items
}

func toBadges(badges []flow.Badge) []components.Badge {
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

func summaryFor(step flow.Step) func(any) string {
	if step.Summarize == nil {
		switch step.Kind {
		case flow.StepText:
			return components.TextSummary
		case flow.StepTextList:
			return components.TextListSummary
		case flow.StepMultiSelect:
			return components.MultiSelectSummary(domain.SummaryNone)
		case flow.StepReorder:
			return components.ReorderSummary
		case flow.StepEnvResolve:
			return components.EnvResolveSummary
		}
		return components.SelectSummary
	}
	return func(model any) string { return step.Summarize(answerOf(step.Kind, model)) }
}
