package cli

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
)

type session = form.Session[fastforward.Request, fastforward.Questions, fastforward.Outcome]

// special returns the step fast-forward asks its own way: the worktrees.
func special(s *session, at int, field domain.Field) form.Step {
	if field.Spec().Path != domain.PathWorktree {
		return nil
	}
	return &worktreesStep{MultiSelectStep: s.MultiSelectStep(at), s: s, at: at}
}

// worktreesStep asks the worktrees, those behind origin checked by default. The default pick confirmed untouched is the command's choice, not the user's: it answers every worktree, so a worktree it cannot move safely is left as it is rather than refusing the run.
type worktreesStep struct {
	*form.MultiSelectStep
	s  *session
	at int
}

// Answer records the worktrees picked, or every worktree when the default pick is confirmed as it was.
func (w *worktreesStep) Answer() string {
	field, _ := w.s.Field(w.at).(domain.MultiSelectField)
	picks, _ := w.Widget().(*widget.MultiSelect)
	picked := picks.Values()

	if field.Origin == domain.OriginDefault && slices.Equal(picked, field.Picked) {
		w.s.Answer(w.at, domain.Answered{Path: domain.PathWorktree}, domain.Answered{Path: fastforward.PathAll, Values: domain.SaidYes(true)})
	} else {
		w.s.Answer(w.at, domain.Answered{Path: domain.PathWorktree, Values: picked})
	}
	return strings.Join(picked, ", ")
}
