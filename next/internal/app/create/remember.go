package create

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

func (a applying) remember(ctx context.Context, items []Item) []wtm.Warning {
	answers := a.Request.answers()
	if len(answers) == 0 || !slices.ContainsFunc(items, created) {
		return nil
	}
	written, err := a.Deps.Remember(ctx, answers)
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

func (r Request) answers() map[string]string {
	sourceUpdate := project.SourceUpdateKeep
	if r.FastForward != nil && *r.FastForward {
		sourceUpdate = project.SourceUpdateFastForward
	}
	ids := map[string]string{"env_from": project.RememberEnvStrategy, "isolation": project.RememberIsolation, "fast_forward": project.RememberSourceUpdate}
	values := map[string]string{"env_from": string(r.EnvFrom), "isolation": string(r.Isolation), "fast_forward": sourceUpdate}
	answers := map[string]string{}
	for _, field := range r.Remember {
		if value := values[field]; value != "" {
			answers[ids[field]] = value
		}
	}
	return answers
}
