package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of stop-jobs' form.
func words(_ daemonstopjobs.Questions, field domain.Field) form.Words {
	if field.Spec().Path == domain.PathRepo {
		return form.Words{Flag: "--repo", Label: "Repository", Help: "Only this repository's jobs (a path in it)"}
	}
	return form.Words{}
}
