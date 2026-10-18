package output

import (
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Tallied is how one item of a run ended, as [Tally] counts it.
type Tallied struct {
	Status domain.Status
	Reason domain.Reason
}

// TallyParams is a run to count: its command, how each of its items ended, and a note after the count, one the reader should look at when the run has items.
type TallyParams struct {
	Command string
	Items   []Tallied
	Note    string
}

// Tally writes the headline of a run to w: its items counted by status, an item done without a change counted as unchanged, in the colour of the worst; nothing to do when it has none.
func Tally(w io.Writer, params TallyParams) {
	note := ""
	if params.Note != "" {
		note = Dot + params.Note
	}

	if len(params.Items) == 0 {
		UnchangedLine(w, params.Command+": "+NothingToDo+note)
		return
	}

	count := map[string]int{}
	for _, item := range params.Items {
		count[tallyKey(item)]++
	}

	var parts []string
	for _, key := range []string{"done", "unchanged", "failed", "skipped", "cancelled"} {
		if count[key] > 0 {
			parts = append(parts, strconv.Itoa(count[key])+" "+key)
		}
	}

	line := params.Command + ": " + strings.Join(parts, Dot) + note
	switch {
	case count["failed"] > 0:
		ErrorLine(w, line)
	case count["skipped"]+count["cancelled"] > 0 || note != "":
		WarningLine(w, line)
	case count["done"] == 0:
		UnchangedLine(w, line)
	default:
		SuccessLine(w, line)
	}
}

func tallyKey(item Tallied) string {
	if item.Status == domain.StatusDone && item.Reason == domain.ReasonUnchanged {
		return "unchanged"
	}

	return string(item.Status)
}
