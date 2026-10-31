package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type session = form.Session[fastforward.Request, fastforward.Questions, fastforward.Outcome]

// special returns the step fast-forward asks its own way: the worktrees, those behind origin checked by default. The default pick confirmed untouched is the command's choice, not the user's: it names none, so a worktree the command cannot move safely is kept rather than refusing the run.
func special(s *session, at int, field domain.Field) form.Step {
	if field.Spec().Path != domain.PathWorktree {
		return nil
	}
	step := s.MultiSelectStep(at)
	step.Chosen = true
	return step
}
