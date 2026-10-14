package render

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	"unicode"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	headlineFmt   = "%s: %s"
	itemLineFmt   = "%s  %s"
	nothingToDo   = "nothing to do"
	listSeparator = ", "
	pairFmt       = "%s " + domain.MoveArrowGlyph + " %s"
	jsonNull      = "null"
)

var yesNo = map[bool]string{true: "yes", false: "no"}

// Detail renders what belongs to one command in its results: the rows of the
// plan a dry run and the form's recap show, drawn alike by styles.RenderPlan,
// and a done item worth a line of its own — a path, a URL.
type Detail interface {
	Plan(detail json.RawMessage) []styles.PlanRow
	Item(w io.Writer, item kernel.Item[json.RawMessage])
}

// GenericDetail is the detail of a command with nothing of its own to show: the
// plan's fields as rows, in their order, each done item folded into the count.
type GenericDetail struct{}

func (GenericDetail) Plan(detail json.RawMessage) []styles.PlanRow {
	fields := fieldsOf(detail)
	rows := make([]styles.PlanRow, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, styles.PlanRow{Label: labelOfKey(field.key), Values: valuesOf(field.value)})
	}
	return rows
}

func (GenericDetail) Item(io.Writer, kernel.Item[json.RawMessage]) {}

type OutcomeParams struct {
	Command   string
	Items     []kernel.Item[json.RawMessage]
	Warnings  []kernel.Warning
	FollowUps []NextStepParams
	Detail    Detail
	Catalog   text.Catalog
}

// Outcome is the conclusion of a run: one headline carrying the worst result
// and the count, the done items only as the command's detail shows them, then
// every other item named with why.
func Outcome(w io.Writer, params OutcomeParams) {
	headline(w, params)
	for _, item := range params.Items {
		outcomeItem(w, outcomeItemParams{Item: item, Detail: params.Detail, Catalog: params.Catalog})
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

// rowsOf reads a detail object as rows in its own key order; a detail that is
// not an object is one row with no label.
func rowsOf(detail json.RawMessage) []AnnounceItem {
	fields := fieldsOf(detail)
	rows := make([]AnnounceItem, 0, len(fields))
	for _, field := range fields {
		rows = append(rows, AnnounceItem{Label: field.key, Value: joined(valuesOf(field.value))})
	}
	return rows
}

type field struct {
	key   string
	value json.RawMessage
}

// fieldsOf reads a detail object's fields in its own key order; a detail that
// is not an object is one field with no key.
func fieldsOf(detail json.RawMessage) []field {
	decoder := json.NewDecoder(bytes.NewReader(detail))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return []field{{value: detail}}
	}
	var fields []field
	for decoder.More() {
		next, ok := nextField(decoder)
		if !ok {
			return fields
		}
		fields = append(fields, next)
	}
	return fields
}

func nextField(decoder *json.Decoder) (field, bool) {
	token, err := decoder.Token()
	if err != nil {
		return field{}, false
	}
	key, ok := token.(string)
	if !ok {
		return field{}, false
	}
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return field{}, false
	}
	return field{key: key, value: value}, true
}

// valuesOf shows a string bare, a yes or a no, a list of strings as its items
// and a map of strings as its pairs; nothing as none, anything else as its JSON.
func valuesOf(value json.RawMessage) []string {
	trimmed := bytes.TrimSpace(value)
	if string(trimmed) == jsonNull {
		return []string{domain.SummaryNone}
	}
	var text string
	if json.Unmarshal(trimmed, &text) == nil {
		return []string{text}
	}
	var flag bool
	if json.Unmarshal(trimmed, &flag) == nil {
		return []string{yesNo[flag]}
	}
	var list []string
	if json.Unmarshal(trimmed, &list) == nil {
		return orNone(list)
	}
	var pairs map[string]string
	if json.Unmarshal(trimmed, &pairs) == nil {
		shown := make([]string, 0, len(pairs))
		for _, key := range slices.Sorted(maps.Keys(pairs)) {
			shown = append(shown, fmt.Sprintf(pairFmt, key, pairs[key]))
		}
		return orNone(shown)
	}
	return []string{string(trimmed)}
}

func orNone(values []string) []string {
	if len(values) == 0 {
		return []string{domain.SummaryNone}
	}
	return values
}

func joined(values []string) string { return strings.Join(values, listSeparator) }

// labelOfKey spells a key of a plan as a label: base_branch, base-branch and
// baseBranch read Base branch.
func labelOfKey(key string) string {
	var b strings.Builder
	for i, r := range key {
		switch {
		case r == '_' || r == '-':
			b.WriteRune(' ')
		case i == 0:
			b.WriteRune(unicode.ToUpper(r))
		case unicode.IsUpper(r):
			b.WriteRune(' ')
			b.WriteRune(unicode.ToLower(r))
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
