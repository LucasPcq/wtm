package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of daemon restart's form.
func words(_ daemonrestart.Questions, field domain.Field) form.Words {
	if field.Spec().Path == daemonrestart.PathForce {
		return form.Words{Flag: "--force", Label: "Force", Help: "Restart a newer wtm's daemon as this build all the same, its stacks left untracked"}
	}
	return form.Words{}
}
