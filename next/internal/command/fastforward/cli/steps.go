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

// worktreesStep asks the worktrees, those behind origin checked by default. The default pick confirmed untouched is the command's choice, not the user's: it names none, so a worktree the command cannot move safely is kept rather than refusing the run.
type worktreesStep struct {
	*form.MultiSelectStep
	s  *session
	at int
}

// Answer records the worktrees picked, or none when the default pick is confirmed as it was.
func (w *worktreesStep) Answer() string {
	field, _ := w.s.Field(w.at).(domain.MultiSelectField)
	picks, _ := w.Widget().(*widget.MultiSelect)
	picked := picks.Values()

	answer := domain.Answered{Path: domain.PathWorktree, Values: picked}
	if field.Origin == domain.OriginDefault && slices.Equal(picked, field.Picked) {
		answer.Values = nil
	}
	w.s.Answer(w.at, answer)
	return strings.Join(picked, ", ")
}
