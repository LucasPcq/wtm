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

// The review of #268: Ran said a listed claim not up ran, where the read says it has not.
func TestASourceRanWhenItsStateSaysSo(t *testing.T) {
	claim := runlogs.Source{Listed: true, Job: run.Job{Name: "db", Hold: run.HoldClaim}}
	up := runlogs.Source{Listed: true, Job: run.Job{Name: "api", Up: true, Hold: run.HoldOwn}}

	assert.False(t, claim.Ran(false), "a claim whose service is gone, with no log, has not run")
	assert.Equal(t, runlogs.JobNotStarted, runlogs.JobOf(claim, nil, false).State, "as the read says it")
	assert.True(t, up.Ran(false))
	assert.True(t, claim.Ran(true), "a log says it ran")
}
