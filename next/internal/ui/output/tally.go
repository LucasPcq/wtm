package output

import (
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Tallied is how one item of a run ended, as [Tally] counts it, and the noun it is counted by when it is not one of the run's own subjects (a drop owed a prune made), none for those.
type Tallied struct {
	Status domain.Status
	Reason domain.Reason
	Noun   string
}

// TallyParams is a run to count: its command, how each of its items ended, and a note after the count, one the reader should look at when the run has items.
type TallyParams struct {
	Command string
	Items   []Tallied
	Note    string
}

// Tally writes the headline of a run to w: its items counted by status, an item done without a change counted as unchanged, then the items of each noun counted by it (1 owed drop made), in the colour of the worst; nothing to do when it has none.
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
	var nouns []string
	for _, item := range params.Items {
		count[tallyKey(item)]++
		if item.Noun != "" && !slices.Contains(nouns, item.Noun) {
			nouns = append(nouns, item.Noun)
		}
	}

	var parts []string
	for _, key := range []string{"done", "unchanged", "failed", "skipped", "cancelled"} {
		if count[key] > 0 {
			parts = append(parts, strconv.Itoa(count[key])+" "+key)
		}
	}
	for _, noun := range nouns {
		for _, said := range []string{"made", "failed", "not made"} {
			if n := count[noun+"\x00"+said]; n > 0 {
				parts = append(parts, strconv.Itoa(n)+" "+noun+plural(n)+" "+said)
			}
		}
	}

	line := params.Command + ": " + strings.Join(parts, Dot) + note
	failed, unmade, made := count["failed"], count["skipped"]+count["cancelled"], count["done"]
	for _, noun := range nouns {
		failed, unmade, made = failed+count[noun+"\x00failed"], unmade+count[noun+"\x00not made"], made+count[noun+"\x00made"]
	}
	switch {
	case failed > 0:
		ErrorLine(w, line)
	case unmade > 0 || note != "":
		WarningLine(w, line)
	case made == 0:
		UnchangedLine(w, line)
	default:
		SuccessLine(w, line)
	}
}

func tallyKey(item Tallied) string {
	switch {
	case item.Noun != "" && item.Status == domain.StatusDone:
		return item.Noun + "\x00made"
	case item.Noun != "" && item.Status == domain.StatusFailed:
		return item.Noun + "\x00failed"
	case item.Noun != "":
		return item.Noun + "\x00not made"
	case item.Status == domain.StatusDone && item.Reason == domain.ReasonUnchanged:
		return "unchanged"
	}

	return string(item.Status)
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
