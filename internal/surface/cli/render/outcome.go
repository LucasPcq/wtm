package render

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	headlineFmt = "%s: %s"
	itemLineFmt = "%s  %s"
	nothingToDo = "nothing to do"
)

type OutcomeParams struct {
	Command   string
	Items     []kernel.Item[json.RawMessage]
	Warnings  []kernel.Warning
	FollowUps []NextStepParams
	Detail    Detail
	Catalog   text.Catalog
}

func Outcome(w io.Writer, params OutcomeParams) {
	headline(w, params)
	for _, item := range params.Items {
		outcomeItem(w, outcomeItemParams{Item: item, Detail: params.Detail, Catalog: params.Catalog})
	}
	for _, item := range params.Items {
		for _, warning := range item.Warnings {
			Warning(w, fmt.Sprintf(itemLineFmt, item.Subject, params.Catalog.Message(warning.Code, warning.Params)))
		}
	}
	for _, warning := range params.Warnings {
		Warning(w, params.Catalog.Message(warning.Code, warning.Params))
	}
	NextSteps(w, params.FollowUps)
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
	Detail  Detail
	Catalog text.Catalog
}

func outcomeItem(w io.Writer, params outcomeItemParams) {
	item := params.Item
	switch item.Status {
	case kernel.StatusDone:
		if item.Reason != kernel.ReasonUnchanged {
			params.Detail.Item(w, item)
		}
	case kernel.StatusFailed:
		Error(w, fmt.Sprintf(itemLineFmt, item.Subject, itemWhy(params)))
	case kernel.StatusSkipped, kernel.StatusCancelled:
		Warning(w, fmt.Sprintf(itemLineFmt, item.Subject, itemWhy(params)))
	}
}

func itemWhy(params outcomeItemParams) string {
	if params.Item.Error != nil {
		return Wording{Catalog: params.Catalog}.Error(params.Item.Error)
	}
	return params.Catalog.Reason(params.Item.Reason)
}
