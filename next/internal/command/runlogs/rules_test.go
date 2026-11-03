package runlogs_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestAResumedFollowDropsTheLinesSentBefore(t *testing.T) {
	sent := lines("one", "two", "three", "four")
	sameTime := []run.Line{{At: at, Text: "a"}, {At: at, Text: "b"}, {At: at, Text: "c"}}
	for name, c := range map[string]struct {
		last  run.Line
		again []run.Line
		kept  []string
		gap   bool
	}{
		"overlap":       {last: sent[1], again: sent, kept: []string{"three", "four"}},
		"nothing_new":   {last: sent[3], again: sent, kept: nil},
		"gap":           {last: sent[0], again: sent[2:], kept: []string{"three", "four"}, gap: true},
		"same_time":     {last: sameTime[1], again: sameTime, kept: []string{"c"}},
		"last_not_sent": {last: run.Line{At: at, Text: "x"}, again: append(sameTime, sent[1]), kept: []string{"two"}},
	} {
		t.Run(name, func(t *testing.T) {
			resume := runlogs.Resume{Last: c.last}
			var kept []string
			for _, line := range c.again {
				if !resume.Drops(line) {
					kept = append(kept, line.Text)
				}
			}
			assert.Equal(t, c.kept, kept)
			assert.Equal(t, c.gap, resume.Gap)
		})
	}
}
