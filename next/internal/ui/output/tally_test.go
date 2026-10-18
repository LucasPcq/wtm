package output

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestATallyCountsEachStatusInTheColourOfTheWorst(t *testing.T) {
	done := Tallied{Status: domain.StatusDone}
	cases := map[string]struct {
		params TallyParams
		want   string
	}{
		"done":      {TallyParams{Command: "create", Items: []Tallied{done, done}}, "  ✓ create: 2 done\n"},
		"unchanged": {TallyParams{Command: "create", Items: []Tallied{{Status: domain.StatusDone, Reason: domain.ReasonUnchanged}}}, "  = create: 1 unchanged\n"},
		"mixed":     {TallyParams{Command: "create", Items: []Tallied{done, {Status: domain.StatusFailed}, {Status: domain.StatusCancelled}}}, "  ✗ create: 1 done · 1 failed · 1 cancelled\n"},
		"cancelled": {TallyParams{Command: "extract", Items: []Tallied{{Status: domain.StatusCancelled}}}, "  ! extract: 1 cancelled\n"},
		"noted":     {TallyParams{Command: "extract", Items: []Tallied{done}, Note: "with conflicts"}, "  ! extract: 1 done · with conflicts\n"},
		"nothing":   {TallyParams{Command: "extract", Note: "no uncommitted changes in main"}, "  = extract: nothing to do · no uncommitted changes in main\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, c.want, rendered(func(w *bytes.Buffer) { Tally(w, c.params) }))
		})
	}
}
