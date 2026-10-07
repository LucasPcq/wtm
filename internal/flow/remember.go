package flow

import (
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// Memory is what a StepSelect may remember of its answer in this repository. A
// recap, a confirmation and a destructive option never are: only the values
// rules.RememberableValues lists for ID can be kept.
type Memory struct {
	// ID is the key under [wizard.remembered]; empty never remembers.
	ID string
	// Value is the remembered answer Recall laid on the step.
	Value string
	// Reask asks the question anyway (--ask), the toggle already on.
	Reask bool
}

type RecallParams struct {
	Session    Session
	Remembered map[string]string
	Ask        bool
}

func Recall(params RecallParams) Session {
	session := params.Session
	steps := slices.Clone(session.Steps)
	for i, step := range steps {
		if step.Memory.ID == "" {
			continue
		}
		steps[i].Memory.Value = params.Remembered[step.Memory.ID]
		steps[i].Memory.Reask = params.Ask
	}
	session.Steps = steps
	return session
}

// Settle answers a step nobody has to be asked: one its Skip rules out, or one
// whose answer is remembered. A preset is not its business: a flag always wins,
// so a surface looks for one first.
func Settle(step Step, answers Answers) (Answer, bool) {
	if step.Skip != nil {
		if skip, reason := step.Skip(answers); skip {
			return Answer{Skipped: true, SkipReason: reason}, true
		}
	}
	if value, ok := Recalled(step); ok {
		return Answer{Value: value, Recalled: true}, true
	}
	return Answer{}, false
}

// Recalled is the answer a step is settled with instead of being asked. A value
// the step no longer offers is ignored and the question asked, never guessed.
func Recalled(step Step) (string, bool) {
	memory := step.Memory
	if memory.Value == "" || memory.Reask {
		return "", false
	}
	return memory.Value, Rememberable(step, memory.Value)
}

func Rememberable(step Step, value string) bool {
	if step.Kind != StepSelect || step.Memory.ID == "" {
		return false
	}
	allowed, known := rules.RememberableValues(step.Memory.ID)
	if !known || !slices.Contains(allowed, value) {
		return false
	}
	if len(step.Options) == 0 {
		return true
	}
	return slices.ContainsFunc(step.Options, func(option Option) bool {
		return option.Value == value && !option.Separator && !option.Disabled && !option.Danger
	})
}

// Asked is how a host hands back an answer it asked: a tick on a value the step
// cannot remember (its "config default", say) is dropped rather than promised in
// the recap, and a remembered question left unticked is marked to be forgotten.
func Asked(step Step, answer Answer) Answer {
	answer.Remember = answer.Remember && Rememberable(step, answer.Value)
	answer.Forget = !answer.Remember && step.Memory.ID != "" && step.Memory.Value != ""
	return answer
}

// Remembering is what a confirmed session asks to keep and to forget: a ticked
// answer is kept, and a remembered question asked again and left unticked is
// forgotten.
func Remembering(session Session, answers Answers) rules.RememberedChange {
	change := rules.RememberedChange{Remember: map[string]string{}}
	for _, step := range session.Steps {
		answer, known := answers.Get(step.Key)
		if step.Memory.ID == "" || !known || !answer.Asked || answer.Skipped {
			continue
		}
		answer = Asked(step, answer)
		if answer.Remember {
			change.Remember[step.Memory.ID] = answer.Value
			continue
		}
		if answer.Forget {
			change.Forget = append(change.Forget, step.Memory.ID)
		}
	}
	return change
}

// RememberedMark qualifies a recap line with the answer's place in memory.
func RememberedMark(answers Answers, key string) string {
	answer, _ := answers.Get(key)
	switch {
	case answer.Recalled:
		return domain.RecapRememberedSuffix
	case answer.Remember:
		return domain.RecapWillRememberSuffix
	case answer.Forget:
		return domain.RecapWillForgetSuffix
	}
	return ""
}

// RememberedHint closes a recap that skipped a remembered question with the way
// to be asked it again.
func RememberedHint(answers Answers, keys ...string) []string {
	for _, key := range keys {
		if answer, _ := answers.Get(key); answer.Recalled {
			return []string{"", domain.RecapRememberedHint}
		}
	}
	return nil
}

type OriginParams struct {
	Answers Answers
	Key     string
	// Flag says the request carried the answer; Fallback is what a Resolve
	// default counts as.
	Flag     bool
	Fallback domain.AnswerOrigin
}

// OriginOf says what settled a rememberable answer; false for a skipped step,
// which settled nothing.
func OriginOf(params OriginParams) (domain.AnswerOrigin, bool) {
	answer, known := params.Answers.Get(params.Key)
	switch {
	case !known || answer.Skipped:
		return "", false
	case params.Flag:
		return domain.AnswerOriginFlag, true
	case answer.Recalled:
		return domain.AnswerOriginRemembered, true
	case answer.Asked:
		return domain.AnswerOriginPrompt, true
	}
	return params.Fallback, true
}
