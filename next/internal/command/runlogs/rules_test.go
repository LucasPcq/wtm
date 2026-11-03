package runlogs_test

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
)

func TestAResumedFollowDropsTheLinesSentBefore(t *testing.T) {
	sent := lines("one", "two", "three", "four")
	blank := run.Line{At: at, Text: ""}
	frame := run.Line{At: at, Text: "at handler"}
	for name, c := range map[string]struct {
		last  run.Line
		sent  int
		again []run.Line
		kept  []string
		gap   bool
	}{
		"overlap":        {last: sent[1], sent: 1, again: sent, kept: []string{"three", "four"}},
		"nothing_new":    {last: sent[3], sent: 1, again: sent, kept: nil},
		"gap":            {last: sent[0], sent: 1, again: sent[2:], kept: []string{"three", "four"}, gap: true},
		"same_ms_twice":  {last: frame, sent: 2, again: []run.Line{frame, frame, frame, {At: at.Add(time.Second), Text: "next"}}, kept: []string{"at handler", "next"}},
		"same_ms_blanks": {last: blank, sent: 1, again: []run.Line{blank, blank}, kept: []string{""}},
		"last_rotated":   {last: frame, sent: 2, again: []run.Line{frame, {At: at.Add(time.Second), Text: "next"}}, kept: []string{"next"}, gap: true},
		"last_not_there": {last: run.Line{At: at, Text: "x"}, sent: 1, again: []run.Line{frame, {At: at.Add(time.Second), Text: "next"}}, kept: []string{"next"}, gap: true},
	} {
		t.Run(name, func(t *testing.T) {
			resume := runlogs.Resume{Last: c.last, Sent: c.sent}
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
