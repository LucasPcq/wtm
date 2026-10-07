// Package flowui runs a flow.Session as a wtm wizard. It is the only place that
// knows both vocabularies: a flow never sees a model, the wizard never sees a
// service.
package flowui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

type Params struct {
	// Stderr renders on stderr, for a command whose stdout the shell wrapper consumes.
	Stderr bool
}

type Prompter struct {
	params Params
}

func New(params Params) Prompter { return Prompter{params: params} }

func (Prompter) Interactive() bool { return true }

func (Prompter) Confirm(params flow.ConfirmParams) (bool, error) {
	if params.YesLabel == "" {
		return components.RunStandaloneConfirm(components.NewConfirm(components.NewConfirmParams{
			Title:       params.Title,
			Description: params.Description,
			Warning:     params.Warning,
			DefaultYes:  params.DefaultYes,
		}))
	}
	choice, err := components.RunStandaloneSelect(components.NewSelectList(components.NewSelectListParams{
		Title:       params.Title,
		Description: flow.ConfirmDescription(params),
		Items:       confirmItems(params),
	}))
	return err == nil && choice == confirmYesValue, err
}

// confirmItems leads with the outcome that changes nothing unless the caller
// asked for the opposite: a decision with a destructive side is never the
// highlighted default.
func confirmItems(params flow.ConfirmParams) []components.SelectItem {
	yes := components.SelectItem{Label: params.YesLabel, Value: confirmYesValue}
	no := components.SelectItem{Label: params.NoLabel, Value: confirmNoValue}
	if params.DefaultYes {
		return []components.SelectItem{yes, {Separator: true}, no}
	}
	return []components.SelectItem{no, {Separator: true}, yes}
}

const (
	confirmYesValue = "yes"
	confirmNoValue  = "no"
)

func (p Prompter) Ask(session flow.Session) (flow.Answers, error) {
	if p.params.Stderr {
		styles.UseRendererOn(os.Stderr)
	}

	plan, err := build(session)
	if err != nil {
		return flow.Answers{}, err
	}
	if plan.entered == 0 {
		return plan.known(), nil
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:       plan.steps,
		Stderr:      p.params.Stderr,
		ErrLabel:    session.ErrLabel,
		InitCmd:     plan.initCmd,
		OnMsg:       plan.handler(),
		Loading:     plan.initCmd != nil,
		LoadingText: plan.loadingText,
	})
	if err != nil {
		return flow.Answers{}, err
	}
	if plan.loadErr != nil {
		return flow.Answers{}, plan.loadErr
	}
	return plan.read(final)
}

func unsupportedKindErr(step flow.Step) error {
	return fmt.Errorf("flowui: step %q has no renderer for kind %d", step.Key, step.Kind)
}

type binding struct {
	key  string
	kind flow.StepKind
	// settled is a step answered before the wizard opened, its answer already
	// known; recalled is one remembered behind a condition, settled against the
	// answers before it, since its Skip still reads them. Neither is entered.
	settled  bool
	recalled bool
	step     flow.Step
}

type plan struct {
	steps    []components.Step
	bindings []binding
	presets  flow.Answers
	// settled are the steps answered before the wizard started: irrelevant, or
	// remembered with nothing earlier to decide whether they apply.
	settled map[string]flow.Answer
	// candidates backs every branch step, so a refresh replaces the list once.
	candidates  []domain.BranchCandidate
	refresh     func() []domain.BranchCandidate
	initCmd     tea.Cmd
	loadingText string
	loads       map[int]loadedStep
	loadErr     error
	// entered counts the steps the wizard will actually put on screen.
	entered int
}

func build(session flow.Session) (*plan, error) {
	p := &plan{presets: session.Presets, settled: map[string]flow.Answer{}}

	for _, step := range session.Steps {
		if answer, preset := session.Presets.Get(step.Key); preset {
			p.settledStep(step, settledLine(step, answer, flagSuffix(step, answer)))
			continue
		}
		if _, recalled := flow.Recalled(step); recalled {
			p.recall(step)
			continue
		}

		// The wizard neither builds nor auto-skips the step it opens on, so a
		// conditional step that would land there is decided here instead,
		// against what is already known.
		conditional := step.Skip != nil
		if conditional && p.entered == 0 {
			if skip, reason := step.Skip(p.known()); skip {
				p.settled[step.Key] = flow.Answer{Skipped: true, SkipReason: reason}
				p.ruledStep(step, reason)
				continue
			}
			conditional = false
		}

		built, err := p.componentStep(step, conditional)
		if err != nil {
			return nil, err
		}
		p.steps = append(p.steps, built)
		p.bindings = append(p.bindings, binding{key: step.Key, kind: step.Kind, step: step})
		p.entered++
	}
	return p, nil
}

// settledStep keeps a step nobody is asked in its place in the wizard, so the
// trail reads it where it would have been asked and the counter never skips it.
func (p *plan) settledStep(step flow.Step, line string) {
	p.steps = append(p.steps, components.Step{Name: step.Label, Model: placeholder(step), Settled: line})
	p.bindings = append(p.bindings, binding{key: step.Key, kind: step.Kind, settled: true, step: step})
}

// ruledStep keeps a step ruled out before the wizard opened where it stands, as
// one ruled out on entry is; a step with no reason to give stays unlisted.
func (p *plan) ruledStep(step flow.Step, reason string) {
	if reason == "" {
		return
	}
	p.steps = append(p.steps, components.Step{Name: step.Label, Model: placeholder(step), Ruled: reason})
	p.bindings = append(p.bindings, binding{key: step.Key, kind: step.Kind, settled: true, step: step})
}

// recall settles a remembered step now when nothing asked later can change
// whether it applies, and otherwise on entry, against the answers before it.
func (p *plan) recall(step flow.Step) {
	if step.Skip == nil || p.entered == 0 {
		answer, _ := flow.Settle(step, p.known())
		p.settled[step.Key] = answer
		if answer.Recalled {
			p.settledStep(step, settledLine(step, answer, domain.RecapRememberedSuffix))
			return
		}
		p.ruledStep(step, answer.SkipReason)
		return
	}
	line, reason := "", ""
	p.steps = append(p.steps, components.Step{
		Name:  step.Label,
		Model: placeholder(step),
		Build: func(prev []components.Step) any {
			answer, _ := flow.Settle(step, p.answersFrom(prev))
			line, reason = "", answer.SkipReason
			if answer.Recalled {
				line = settledLine(step, answer, domain.RecapRememberedSuffix)
			}
			return placeholder(step)
		},
		AutoSkip:       func(components.WizardModel) bool { return true },
		SettledSummary: func() string { return line },
		SkipReason:     func() string { return reason },
	})
	p.bindings = append(p.bindings, binding{key: step.Key, kind: step.Kind, recalled: true, step: step})
}

// settledLine is what the trail says of an answer nobody was asked: the step's
// own summary, as an answered step reads, and what settled it.
func settledLine(step flow.Step, answer flow.Answer, suffix string) string {
	summary := answer.Value
	switch {
	case step.Summarize != nil:
		summary = step.Summarize(answer)
	case len(answer.Values) > 0:
		summary = flow.SummarizeSet(answer)
	}
	if summary == "" {
		summary = domain.SummaryNone
	}
	return summary + suffix
}

func flagSuffix(step flow.Step, answer flow.Answer) string {
	flag := step.Flag
	if step.PresetFlag != nil {
		flag = step.PresetFlag(answer)
	}
	if flag == "" {
		return ""
	}
	return fmt.Sprintf(domain.TrailFlagSuffixFmt, flag)
}

func (p *plan) known() flow.Answers {
	answers := p.presets
	for key, answer := range p.settled {
		answers = answers.With(key, answer)
	}
	return answers
}

func (p *plan) answersFrom(prev []components.Step) flow.Answers {
	answers := p.known()
	for i, b := range p.bindings {
		if i >= len(prev) {
			break
		}
		answers = answers.With(b.key, p.answerAt(b, prev[i].Model, answers))
	}
	return answers
}

func (p *plan) answerAt(b binding, model any, answers flow.Answers) flow.Answer {
	if b.settled {
		answer, _ := answers.Get(b.key)
		return answer
	}
	if b.recalled {
		answer, _ := flow.Settle(b.step, answers)
		return answer
	}
	return flow.Asked(b.step, answerOf(b.kind, model))
}

func (p *plan) read(final components.WizardModel) (flow.Answers, error) {
	steps := final.Steps()
	answers := p.known()
	for i, b := range p.bindings {
		if i >= len(steps) {
			break
		}
		if b.settled {
			continue
		}
		if b.recalled {
			answers = answers.With(b.key, p.answerAt(b, steps[i].Model, answers))
			continue
		}
		answer := flow.Asked(b.step, answerOf(b.kind, steps[i].Model))
		if answer.Value == domain.WizardCancelValue {
			return flow.Answers{}, domain.ErrUserAborted
		}
		if final.Skipped(i) {
			answer = flow.Answer{Skipped: true, SkipReason: skipReasonOf(steps[i])}
		}
		answers = answers.With(b.key, answer)
	}
	return answers, nil
}

func skipReasonOf(step components.Step) string {
	if step.SkipReason == nil {
		return ""
	}
	return step.SkipReason()
}

// answerOf is the single place where the wizard's model-per-kind switch is crossed.
func answerOf(kind flow.StepKind, model any) flow.Answer {
	switch kind {
	case flow.StepText:
		if text, ok := model.(components.TextInputModel); ok {
			return flow.Answer{Value: text.Value(), Asked: true}
		}
	case flow.StepSelect, flow.StepBranchSelect, flow.StepRecap:
		if list, ok := model.(components.SelectListModel); ok {
			return flow.Answer{Value: list.Value(), Asked: true, Remember: list.Remembering()}
		}
	case flow.StepTextList:
		if list, ok := model.(components.TextListModel); ok {
			return flow.Answer{Values: list.Values(), Asked: true}
		}
	case flow.StepMultiSelect:
		if list, ok := model.(components.MultiSelectModel); ok {
			return flow.Answer{Values: list.Values(), Asked: true}
		}
	case flow.StepReorder:
		if list, ok := model.(components.ReorderListModel); ok {
			return flow.Answer{Values: list.Values(), Asked: true}
		}
	case flow.StepEnvResolve:
		if resolve, ok := model.(components.EnvResolveModel); ok {
			return flow.Answer{EnvDecisions: resolve.Decisions(), Asked: true}
		}
	}
	return flow.Answer{}
}
