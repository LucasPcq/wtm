package cli

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
)

type session = form.Session[prune.Request, prune.Questions, prune.Outcome]

// special returns the step prune asks its own way: the worktrees.
func special(s *session, at int, field domain.Field) form.Step {
	if field.Spec().Path != domain.PathWorktree {
		return nil
	}
	return &worktreesStep{MultiSelectStep: s.MultiSelectStep(at), s: s, at: at}
}

// worktreesStep asks the worktrees, those safe to remove checked by default. The default pick confirmed untouched is the command's choice, not the user's: it names no worktree, so one unsafe to remove is left as it is rather than refusing the run.
type worktreesStep struct {
	*form.MultiSelectStep
	s  *session
	at int
}

// Answer records the worktrees picked, or none when the default pick is confirmed as it was; a step that is not the multi-select it expects answers as the engine's does.
func (w *worktreesStep) Answer() string {
	field, isField := w.s.Field(w.at).(domain.MultiSelectField)
	picks, isPicks := w.Widget().(*widget.MultiSelect)
	if !isField || !isPicks {
		return w.MultiSelectStep.Answer()
	}
	picked := picks.Values()

	if field.Origin == domain.OriginDefault && slices.Equal(picked, field.Picked) {
		w.s.Answer(w.at, domain.Answered{Path: domain.PathWorktree})
	} else {
		w.s.Answer(w.at, domain.Answered{Path: domain.PathWorktree, Values: picked})
	}
	return strings.Join(picked, ", ")
}
