package flow

// Unattended is the Prompter of a run that cannot ask: --yes, no terminal, or a
// machine output format. It never falls back to a picker.
type Unattended struct{}

func (Unattended) Ask(session Session) (Answers, error) {
	answers := session.Presets
	for _, step := range session.Steps {
		if _, known := answers.Get(step.Key); known {
			continue
		}
		if answer, settled := Settle(step, answers); settled {
			answers = answers.With(step.Key, answer)
			continue
		}
		if step.Resolve == nil {
			return Answers{}, requiredErr(step)
		}
		answer, err := step.Resolve(answers)
		if err != nil {
			return Answers{}, err
		}
		answers = answers.With(step.Key, answer)
	}
	return answers, nil
}

func (Unattended) Confirm(ConfirmParams) (bool, error) { return false, nil }

func (Unattended) Interactive() bool { return false }
