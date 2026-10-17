package create

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

func (p ApplyParams) remember(ctx context.Context, items []Item) []wtm.Warning {
	answers := p.answers()
	if len(answers) == 0 || !slices.ContainsFunc(items, created) {
		return nil
	}
	written, err := p.Deps.Remember(ctx, answers)
	if err != nil {
		return []wtm.Warning{{Code: "create.remember_failed", Message: "could not remember the answer: " + err.Error(), Params: wtm.Params{"cause": err.Error()}}}
	}
	if written.Unlocked {
		return []wtm.Warning{{Code: "locks.unavailable", Message: "ran without operation locks"}}
	}
	return nil
}

func created(item Item) bool {
	return item.Status == wtm.StatusDone && item.Reason != wtm.ReasonUnchanged
}

func (p ApplyParams) answers() map[string]string {
	sourceUpdate := project.SourceUpdateKeep
	if p.FastForward {
		sourceUpdate = project.SourceUpdateFastForward
	}
	values := map[string]string{"env_from": string(p.Plan.EnvFrom), "isolation": string(p.Plan.Isolation), "fast_forward": sourceUpdate}
	answers := map[string]string{}
	for _, r := range remembered {
		if value := values[r.field]; value != "" && slices.Contains(p.Plan.Remember, r.field) {
			answers[r.id] = value
		}
	}
	return answers
}
