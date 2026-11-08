package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of daemon stop's form: it has none.
func words(daemonstop.Questions, domain.Field) form.Words {
	return form.Words{}
}
