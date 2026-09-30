// Package flowtest provides test doubles for the flow surfaces: a Prompter that
// answers from a script and a Presenter that records what was shown.
package flowtest

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
)

type ScriptedPrompter struct {
	Answers map[string]string
	// Sets answers a StepMultiSelect step, whose answer is a set rather than a value.
	Sets      map[string][]string
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

	answers := session.Presets
	for _, step := range session.Steps {
		if _, known := answers.Get(step.Key); known {
			continue
		}
		if step.Skip != nil {
			if skip, reason := step.Skip(answers); skip {
				answers = answers.With(step.Key, flow.Answer{Skipped: true, SkipReason: reason})
				continue
			}
		}
		content, err := stepContent(step, answers)
		if err != nil {
			return flow.Answers{}, err
		}
		p.Content[step.Key] = content

		if values, scripted := p.Sets[step.Key]; scripted {
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
		answers = answers.With(step.Key, flow.Answer{Value: value, Asked: true})
	}
	return answers, nil
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

func (p *ScriptedPrompter) Confirm(flow.ConfirmParams) (bool, error) {
	p.Confirms++
	return p.Confirmed, nil
}

func (p *ScriptedPrompter) Interactive() bool { return true }

// AskedKeys joins the steps that were asked, for a one-line assertion.
func (p *ScriptedPrompter) AskedKeys() string { return strings.Join(p.Asked, ",") }

type Recorder struct {
	Stages   []string
	Hooks    []string
	Beats    []domain.HookBeat
	Notices  []flow.Notice
	Statuses []flow.Notice
}

func (r *Recorder) Stage(params flow.StageParams) error {
	r.Stages = append(r.Stages, params.Message)
	return params.Work()
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
