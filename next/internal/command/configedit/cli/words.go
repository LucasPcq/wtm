package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/configedit"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of config edit's form: it has none.
func words(configedit.Questions, domain.Field) form.Words {
	return form.Words{}
}
