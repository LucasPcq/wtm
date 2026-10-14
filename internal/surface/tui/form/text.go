package form

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/kernel"
)

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

func (s *session) fieldMessage(problem kernel.FieldError) string {
	named := problem
	named.Path = s.labelOfPath(problem.Path)
	named.With = make([]string, 0, len(problem.With))
	for _, path := range problem.With {
		named.With = append(named.With, s.labelOfPath(path))
	}
	return s.params.Catalog.Field(named)
}

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
