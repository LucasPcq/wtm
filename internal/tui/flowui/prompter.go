// Package flowui runs a flow.Session as a wtm wizard. It is the only place that
// knows both vocabularies: a flow never sees a model, the wizard never sees a
// service.
package flowui

import (
	"context"
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

// Prompter keeps the run's context for the reads a wizard does in the
// background (the branch refresh): Ask belongs to flow.Prompter, which takes none.
type Prompter struct {
	ctx    context.Context
	params Params
}

func New(ctx context.Context, params Params) Prompter { return Prompter{ctx: ctx, params: params} }

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
	if len(plan.steps) == 0 {
		return plan.known(), nil
	}

	final, err := components.RunWizard(components.RunWizardParams{
		Steps:       plan.steps,
		Stderr:      p.params.Stderr,
		ErrLabel:    session.ErrLabel,
		InitCmd:     plan.initCmd,
		OnMsg:       plan.handler(p.ctx),
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
}

type plan struct {
	steps    []components.Step
	bindings []binding
	presets  flow.Answers
	// skips are the steps resolved as irrelevant before the wizard started.
	skips map[string]string
	// candidates backs every branch step, so a refresh replaces the list once.
	candidates  []domain.BranchCandidate
	refresh     func() []domain.BranchCandidate
	initCmd     tea.Cmd
	loadingText string
	loads       map[int]loadedStep
	loadErr     error
}

func build(session flow.Session) (*plan, error) {
	p := &plan{presets: session.Presets, skips: map[string]string{}}

	for _, step := range session.Steps {
		if _, preset := session.Presets.Get(step.Key); preset {
			continue
		}

		// The wizard neither builds nor auto-skips step 0, so a conditional step that
		// would land there is decided here instead, against what is already known.
		conditional := step.Skip != nil
		if conditional && len(p.steps) == 0 {
			if skip, reason := step.Skip(p.known()); skip {
				p.skips[step.Key] = reason
				continue
			}
			conditional = false
		}

		built, err := p.componentStep(step, conditional)
		if err != nil {
			return nil, err
		}
		p.steps = append(p.steps, built)
		p.bindings = append(p.bindings, binding{key: step.Key, kind: step.Kind})
	}
	return p, nil
}

func (p *plan) known() flow.Answers {
	answers := p.presets
	for key, reason := range p.skips {
		answers = answers.With(key, flow.Answer{Skipped: true, SkipReason: reason})
	}
	return answers
}

func (p *plan) answersFrom(prev []components.Step) flow.Answers {
	answers := p.known()
	for i, b := range p.bindings {
		if i >= len(prev) {
			break
		}
		answers = answers.With(b.key, answerOf(b.kind, prev[i].Model))
	}
	return answers
}

func (p *plan) read(final components.WizardModel) (flow.Answers, error) {
	steps := final.Steps()
	answers := p.known()
	for i, b := range p.bindings {
		if i >= len(steps) {
			break
		}
		answer := answerOf(b.kind, steps[i].Model)
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
			return flow.Answer{Value: list.Value(), Asked: true}
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
