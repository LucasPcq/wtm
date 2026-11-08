package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run ps' form.
func words(_ runps.Questions, field domain.Field) form.Words {
	if field.Spec().Path == domain.PathRepo {
		return form.Words{Flag: "--repo", Label: "Repository", Help: "Only this repository's jobs (a path in it)"}
	}

	return form.Words{}
}
