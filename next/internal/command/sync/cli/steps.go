package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

type session = form.Session[sync.Request, sync.Questions, sync.Outcome]

// special returns the step sync asks its own way: the worktrees, those that need a sync checked by default. The default pick confirmed untouched is the command's choice, not the user's: it names none, so a worktree the command cannot rebase safely is kept rather than refusing the run.
func special(s *session, at int, field domain.Field) form.Step {
	if field.Spec().Path != domain.PathWorktree {
		return nil
	}
	step := s.MultiSelectStep(at)
	step.Chosen = true
	return step
}
