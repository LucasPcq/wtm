package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of daemon restart's form: it has none.
func words(daemonrestart.Questions, domain.Field) form.Words {
	return form.Words{}
}
