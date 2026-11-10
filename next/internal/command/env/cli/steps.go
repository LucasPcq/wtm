package cli

import (
	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type session = form.Session[envcmd.Request, envcmd.Questions, envcmd.Outcome]

// special returns the step env asks its own way: the worktrees, those out of step and the caller's checked by default. The default pick confirmed untouched is the command's choice, not the user's.
func special(s *session, at int, field domain.Field) form.Step {
	if field.Spec().Path != domain.PathWorktree {
		return nil
	}
	step := s.MultiSelectStep(at)
	step.Chosen = true
	return step
}
