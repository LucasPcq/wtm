package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of daemon status's form: it has none.
func words(daemonstatus.Questions, domain.Field) form.Words {
	return form.Words{}
}
