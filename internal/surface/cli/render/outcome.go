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
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	headlineFmt    = "%s: %s"
	itemLineFmt    = "%s  %s"
	nothingToDo    = "nothing to do"
	labelDone      = "done"
	labelUnchanged = "unchanged"
	labelFailed    = "failed"
	labelSkipped   = "skipped"
	labelCancelled = "cancelled"
	listSeparator  = ", "
	pairFmt        = "%s " + domain.MoveArrowGlyph + " %s"
)

var yesNo = map[bool]string{true: "yes", false: "no"}

var reasonLabels = map[kernel.Reason]string{
	kernel.ReasonInterrupted: "interrupted",
	kernel.ReasonNotReached:  "not reached",
	kernel.ReasonUnchanged:   "unchanged",
	kernel.ReasonUnsafe:      "unsafe",
	kernel.ReasonLocked:      "locked",
}

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
	items := rowsOf(detail)
	rows := make([]styles.PlanRow, 0, len(items))
	for _, item := range items {
		rows = append(rows, styles.PlanRow{Label: labelOfKey(item.Label), Value: item.Value})
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
}

// Outcome is the conclusion of a run: one headline carrying the worst result
// and the count, the done items only as the command's detail shows them, then
// every other item named with why.
func Outcome(w io.Writer, params OutcomeParams) {
	headline(w, params)
	for _, item := range params.Items {
		outcomeItem(w, outcomeItemParams{Item: item, Detail: params.Detail})
	}
	for _, warning := range params.Warnings {
		Warning(w, WarningMessage(warning))
	}
	NextSteps(w, params.FollowUps)
}

func headline(w io.Writer, params OutcomeParams) {
	if len(params.Items) == 0 {
		Unchanged(w, fmt.Sprintf(headlineFmt, params.Command, nothingToDo))
		return
	}
	count := tally(params.Items)
	line := fmt.Sprintf(headlineFmt, params.Command, rules.Tally(count.parts()...))
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

func (c itemCount) parts() []domain.TallyPart {
	return []domain.TallyPart{
		{Count: c.done, Label: labelDone},
		{Count: c.unchanged, Label: labelUnchanged},
		{Count: c.failed, Label: labelFailed},
		{Count: c.skipped, Label: labelSkipped},
		{Count: c.cancelled, Label: labelCancelled},
	}
}

type outcomeItemParams struct {
	Item   kernel.Item[json.RawMessage]
	Detail Detail
}

func outcomeItem(w io.Writer, params outcomeItemParams) {
	item := params.Item
	switch item.Status {
	case kernel.StatusDone:
		if item.Reason != kernel.ReasonUnchanged {
			params.Detail.Item(w, item)
		}
	case kernel.StatusFailed:
		Error(w, fmt.Sprintf(itemLineFmt, item.Subject, itemWhy(item)))
	case kernel.StatusSkipped, kernel.StatusCancelled:
		Warning(w, fmt.Sprintf(itemLineFmt, item.Subject, itemWhy(item)))
	}
}

func itemWhy(item kernel.Item[json.RawMessage]) string {
	if item.Error != nil {
		return ErrorMessage(item.Error)
	}
	return reasonLabels[item.Reason]
}

// rowsOf reads a detail object as rows in its own key order; a detail that is
// not an object is one row with no label.
func rowsOf(detail json.RawMessage) []AnnounceItem {
	decoder := json.NewDecoder(bytes.NewReader(detail))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return []AnnounceItem{{Value: scalarText(detail)}}
	}
	var rows []AnnounceItem
	for decoder.More() {
		row, ok := nextRow(decoder)
		if !ok {
			return rows
		}
		rows = append(rows, row)
	}
	return rows
}

func nextRow(decoder *json.Decoder) (AnnounceItem, bool) {
	token, err := decoder.Token()
	if err != nil {
		return AnnounceItem{}, false
	}
	key, ok := token.(string)
	if !ok {
		return AnnounceItem{}, false
	}
	var value json.RawMessage
	if err := decoder.Decode(&value); err != nil {
		return AnnounceItem{}, false
	}
	return AnnounceItem{Label: key, Value: scalarText(value)}, true
}

// scalarText shows a string bare, a yes or a no, a list of strings joined and
// a map of strings as its pairs; anything else as its JSON.
func scalarText(value json.RawMessage) string {
	var text string
	if json.Unmarshal(value, &text) == nil {
		return text
	}
	var flag bool
	if json.Unmarshal(value, &flag) == nil {
		return yesNo[flag]
	}
	var list []string
	if json.Unmarshal(value, &list) == nil {
		return joined(list)
	}
	var pairs map[string]string
	if json.Unmarshal(value, &pairs) == nil {
		shown := make([]string, 0, len(pairs))
		for _, key := range slices.Sorted(maps.Keys(pairs)) {
			shown = append(shown, fmt.Sprintf(pairFmt, key, pairs[key]))
		}
		return joined(shown)
	}
	return string(bytes.TrimSpace(value))
}

func joined(list []string) string {
	if len(list) == 0 {
		return domain.SummaryNone
	}
	return strings.Join(list, listSeparator)
}

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
