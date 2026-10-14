package form

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

// What only the form says; everything the engine says comes from the catalogue.
const (
	observingText = "Reading the repository…"
	choicesText   = "Loading the choices…"
	refreshText   = "Refreshing…"
	planText      = "Computing the plan…"
	recapName     = "Confirm"
	recapTitle    = "Apply this plan?"
	failedTitle   = "Cannot apply this plan"
	detailIndent  = "    "
	applyLabel    = "Yes, apply"
	applyValue    = "apply"
	notACommand   = "the entry answered no plan"
	notNeeded     = "not needed"
	yesText       = "yes"
	noText        = "no"
	trailFmt      = "%s · %s"
	pairFmt       = "%s=%s"
	listSeparator = ", "
	keySeparator  = "\x00"
	entryOpen     = "["
)

func (s *session) message(code kernel.Code, params kernel.Params) string {
	return s.params.Catalog.Message(code, params)
}

// fieldMessage names the fields by their label, as the form shows them,
// where the command line spells them as flags.
func (s *session) fieldMessage(problem kernel.FieldError) string {
	named := problem
	named.Path = s.labelOfPath(problem.Path)
	named.With = make([]string, 0, len(problem.With))
	for _, path := range problem.With {
		named.With = append(named.With, s.labelOfPath(path))
	}
	return s.params.Catalog.Field(named)
}

// labelOfPath is a path's label, keeping the entry it points into.
func (s *session) labelOfPath(path string) string {
	field, entry, indexed := strings.Cut(path, entryOpen)
	for _, spec := range s.fields {
		if spec.Path != field {
			continue
		}
		if indexed {
			return labelOf(spec) + entryOpen + entry
		}
		return labelOf(spec)
	}
	return path
}

func (s *session) badgesOf(badges []kernel.Badge) []components.Badge {
	shown := make([]components.Badge, 0, len(badges))
	for _, badge := range badges {
		shown = append(shown, s.badgeOf(badge))
	}
	return shown
}

func (s *session) badgeOf(badge kernel.Badge) components.Badge {
	return components.Badge{Text: s.message(badge.Code, badge.Params), Variant: components.BadgeNeutral}
}
