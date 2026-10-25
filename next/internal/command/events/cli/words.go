package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of the stream's form.
func words(_ events.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case events.PathRepo:
		return form.Words{Flag: "--repo", Label: "Repository", Help: "Watch the repository holding this path instead of the current one"}
	case events.PathAll:
		return form.Words{Flag: "--all", Label: "Every repository", Help: "Watch every repository wtm knows, whatever the current directory or $GIT_DIR"}
	}

	return form.Words{}
}
