package render

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	headlineFmt     = "%s: %s"
	itemLineFmt     = "%s  %s"
	nothingToDo     = "nothing to do"
	reasonSeparator = " · "
)

type OutcomeParams struct {
	Command   string
	Plan      json.RawMessage
	Items     []kernel.Item[json.RawMessage]
	Warnings  []kernel.Warning
	FollowUps []NextStepParams
	Detail    Detail
	Catalog   text.Catalog
}

func Outcome(w io.Writer, params OutcomeParams) {
	headline(w, params)
	applied := appliedOf(params)
	if len(applied.rows) > 0 {
		Blank(w)
		plan := styles.RenderPlan(styles.PlanParams{Rows: applied.rows, Warnings: applied.warnings, Width: max(TerminalWidthOf(w)-len(Indent), 0)})
		for line := range strings.Lines(plan) {
			indented(w, strings.TrimSuffix(line, "\n"))
		}
	}
	if len(applied.rows) == 0 && len(applied.warnings) > 0 {
		Blank(w)
		for _, warning := range applied.warnings {
			Warning(w, warning)
		}
	}
	unfinished := slices.DeleteFunc(slices.Clone(params.Items), finished)
	if len(unfinished) > 0 {
		Blank(w)
	}
	for _, item := range unfinished {
		outcomeItem(w, outcomeItemParams{Item: item, Catalog: params.Catalog})
	}
	next := append(applied.next, params.FollowUps...)
	if len(next) > 0 {
		Blank(w)
		NextSteps(w, next)
	}
}

type applied struct {
	rows     []styles.PlanRow
	warnings []string
	next     []NextStepParams
}

func appliedOf(params OutcomeParams) applied {
	var result applied
	if len(params.Plan) > 0 && len(params.Items) > 0 {
		result.rows = params.Detail.Plan(params.Plan)
	}
	for _, item := range params.Items {
		if !standing(item) {
			continue
		}
		view := params.Detail.Item(item)
		result.rows = append(result.rows, view.Rows...)
		result.warnings = append(result.warnings, view.Anomalies...)
		result.next = append(result.next, view.Next...)
	}
	for _, item := range params.Items {
		for _, warning := range item.Warnings {
			result.warnings = append(result.warnings, fmt.Sprintf(itemLineFmt, item.Subject, params.Catalog.Message(warning.Code, warning.Params)))
		}
	}
	for _, warning := range params.Warnings {
		result.warnings = append(result.warnings, params.Catalog.Message(warning.Code, warning.Params))
	}
	return result
}

func finished(item kernel.Item[json.RawMessage]) bool {
	return item.Status == kernel.StatusDone
}

func standing(item kernel.Item[json.RawMessage]) bool {
	return item.Status == kernel.StatusDone || item.Reason == kernel.ReasonIncomplete
}

func headline(w io.Writer, params OutcomeParams) {
	if len(params.Items) == 0 {
		Unchanged(w, fmt.Sprintf(headlineFmt, params.Command, nothingToDo))
		return
	}
	count := tally(params.Items)
	line := fmt.Sprintf(headlineFmt, params.Command, rules.Tally(count.parts(params.Catalog)...))
	switch {
	case count.failed > 0:
		Error(w, line)
	case count.skipped+count.cancelled > 0:
		Warning(w, line)
	case count.done == 0:
		Unchanged(w, line)
	default:
		Success(w, line)
	}
}

type itemCount struct {
	done, unchanged, failed, skipped, cancelled int
}

func tally(items []kernel.Item[json.RawMessage]) itemCount {
	var count itemCount
	for _, item := range items {
		count = count.add(item)
	}
	return count
}

func (c itemCount) add(item kernel.Item[json.RawMessage]) itemCount {
	switch {
	case item.Status == kernel.StatusDone && item.Reason == kernel.ReasonUnchanged:
		c.unchanged++
	case item.Status == kernel.StatusDone:
		c.done++
	case item.Status == kernel.StatusFailed:
		c.failed++
	case item.Status == kernel.StatusSkipped:
		c.skipped++
	case item.Status == kernel.StatusCancelled:
		c.cancelled++
	}
	return c
}

func (c itemCount) parts(catalog text.Catalog) []domain.TallyPart {
	return []domain.TallyPart{
		{Count: c.done, Label: catalog.Status(kernel.StatusDone)},
		{Count: c.unchanged, Label: catalog.Reason(kernel.ReasonUnchanged)},
		{Count: c.failed, Label: catalog.Status(kernel.StatusFailed)},
		{Count: c.skipped, Label: catalog.Status(kernel.StatusSkipped)},
		{Count: c.cancelled, Label: catalog.Status(kernel.StatusCancelled)},
	}
}

type outcomeItemParams struct {
	Item    kernel.Item[json.RawMessage]
	Catalog text.Catalog
}

func outcomeItem(w io.Writer, params outcomeItemParams) {
	item := params.Item
	switch item.Status {
	case kernel.StatusFailed:
		Error(w, fmt.Sprintf(itemLineFmt, item.Subject, itemWhy(params)))
	case kernel.StatusSkipped, kernel.StatusCancelled:
		Warning(w, fmt.Sprintf(itemLineFmt, item.Subject, itemWhy(params)))
	case kernel.StatusDone:
	}
}

func itemWhy(params outcomeItemParams) string {
	item := params.Item
	if item.Error == nil {
		return params.Catalog.Reason(item.Reason)
	}
	why := Wording{Catalog: params.Catalog}.Error(item.Error)
	if item.Reason != kernel.ReasonIncomplete {
		return why
	}
	return why + reasonSeparator + params.Catalog.Reason(item.Reason)
}
