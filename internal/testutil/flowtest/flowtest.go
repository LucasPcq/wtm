// Package flowtest provides test doubles for the flow surfaces: a Prompter that
// answers from a script and a Presenter that records what was shown.
package flowtest

import (
	"context"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

type ScriptedPrompter struct {
	Answers map[string]string
	// Sets answers a StepMultiSelect, StepReorder or StepTextList step, whose answer is a set.
	Sets map[string][]string
	// EnvDecisions answers a StepEnvResolve step.
	EnvDecisions map[string][]domain.EnvFileDecision
	// Remember ticks "always use this answer" on these steps.
	Remember  map[string]bool
	Abort     bool
	Confirmed bool

	Asked   []string
	Content map[string]flow.StepContent
	// Confirms counts the standalone confirmations asked, so a test can assert
	// that a step said nothing rather than only what it answered.
	Confirms int
}

// Ask walks the session like a real host does — honoring Skip, Build and Load — and
// answers each step from the script.
func (p *ScriptedPrompter) Ask(session flow.Session) (flow.Answers, error) {
	if p.Abort {
		return flow.Answers{}, domain.ErrUserAborted
	}
	if p.Content == nil {
		p.Content = map[string]flow.StepContent{}
	}

	if err := buildBeforeAsking(session); err != nil {
		return flow.Answers{}, err
	}

	answers := session.Presets
	for _, step := range session.Steps {
		if _, known := answers.Get(step.Key); known {
			continue
		}
		if answer, settled := flow.Settle(step, answers); settled {
			answers = answers.With(step.Key, answer)
			continue
		}
		content, err := stepContent(step, answers)
		if err != nil {
			return flow.Answers{}, err
		}
		p.Content[step.Key] = content

		if values, scripted := p.Sets[step.Key]; scripted {
			if step.Kind == flow.StepTextList {
				accepted, err := acceptEntries(step, values)
				if err != nil {
					return flow.Answers{}, err
				}
				values = accepted
			}
			// A real host refuses to advance on a failed validation; a double that
			// skipped it would let a flow ship a rule nothing ever runs.
			if step.ValidateSet != nil {
				if err := step.ValidateSet(values); err != nil {
					return flow.Answers{}, err
				}
			}
			p.Asked = append(p.Asked, step.Key)
			answers = answers.With(step.Key, flow.Answer{Values: values, Asked: true})
			continue
		}
		if decisions, scripted := p.EnvDecisions[step.Key]; scripted {
			p.Asked = append(p.Asked, step.Key)
			answers = answers.With(step.Key, flow.Answer{EnvDecisions: decisions, Asked: true})
			continue
		}
		value, scripted := p.Answers[step.Key]
		if !scripted {
			return flow.Answers{}, fmt.Errorf("nothing scripted for step %q", step.Key)
		}
		if step.Validate != nil {
			if err := step.Validate(value); err != nil {
				return flow.Answers{}, err
			}
		}
		p.Asked = append(p.Asked, step.Key)
		answers = answers.With(step.Key, flow.Asked(step, flow.Answer{Value: value, Asked: true, Remember: p.remembers(step)}))
	}
	return answers, nil
}

// buildBeforeAsking is the pass the wizard makes before it opens: every step is
// built from the presets alone, so a Build that fails on an answer not given yet
// aborts the session before its first question.
func buildBeforeAsking(session flow.Session) error {
	for _, step := range session.Steps {
		if _, preset := session.Presets.Get(step.Key); preset || step.Build == nil {
			continue
		}
		if _, err := step.Build(session.Presets); err != nil {
			return fmt.Errorf("step %q cannot be built before the session opens: %w", step.Key, err)
		}
	}
	return nil
}

func stepContent(step flow.Step, answers flow.Answers) (flow.StepContent, error) {
	switch {
	case step.Build != nil:
		return step.Build(answers)
	case step.Load != nil:
		return step.Load(answers)
	}
	return flow.StepContent{Title: step.Title, Description: step.Description, Options: step.Options}, nil
}

// remembers is the toggle as the wizard shows it: --ask opens it ticked.
func (p *ScriptedPrompter) remembers(step flow.Step) bool {
	if ticked, scripted := p.Remember[step.Key]; scripted {
		return ticked
	}
	return step.Memory.Reask && step.Memory.Value != ""
}

func (p *ScriptedPrompter) Confirm(flow.ConfirmParams) (bool, error) {
	p.Confirms++
	return p.Confirmed, nil
}

func (p *ScriptedPrompter) Interactive() bool { return true }

// AskedKeys joins the steps that were asked, for a one-line assertion.
func (p *ScriptedPrompter) AskedKeys() string { return strings.Join(p.Asked, ",") }

type Recorder struct {
	Stages    []string
	Hooks     []string
	Beats     []domain.HookBeat
	Notices   []flow.Notice
	Statuses  []flow.Notice
	Published []domain.Event
	// Unheard makes the Recorder a publisher no daemon listens to.
	Unheard bool
	// From is the origin it hands a request to the daemon; zero names none.
	From domain.EventOrigin
}

// Publish makes a Recorder the flow's Publisher too, so one double records
// what a run showed and what it reported to the bus.
func (r *Recorder) Publish(_ context.Context, event domain.Event) {
	r.Published = append(r.Published, event)
}

func (r *Recorder) Listening() bool { return !r.Unheard }

func (r *Recorder) Origin(context.Context) (domain.EventOrigin, bool) {
	return r.From, r.From.Repo.CommonDir != ""
}

func (r *Recorder) PublishedTypes() []domain.EventType {
	types := make([]domain.EventType, 0, len(r.Published))
	for _, event := range r.Published {
		types = append(types, event.Type)
	}
	return types
}

func (r *Recorder) Stage(ctx context.Context, params flow.StageParams) error {
	r.Stages = append(r.Stages, params.Message)
	return params.Work(ctx)
}

func (r *Recorder) HookPhase(params flow.HookPhaseParams) error {
	r.Hooks = append(r.Hooks, params.Title)
	var sink strings.Builder
	return params.Run(flow.HookSink{Output: &sink, OnHook: func(beat domain.HookBeat) {
		r.Beats = append(r.Beats, beat)
	}})
}

func (r *Recorder) Notice(notice flow.Notice) { r.Notices = append(r.Notices, notice) }

func (r *Recorder) Status(notice flow.Notice) { r.Statuses = append(r.Statuses, notice) }

func acceptEntries(step flow.Step, values []string) ([]string, error) {
	var accepted []string
	for _, value := range values {
		entry, err := flow.CheckEntry(step, flow.EntryCheck{Entry: value, Entries: accepted})
		if err != nil {
			return nil, err
		}
		accepted = append(accepted, entry)
	}
	return accepted, nil
}
