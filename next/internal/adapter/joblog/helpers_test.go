package joblog

import (
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// at is when the lines of a test were printed.
var at = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// texts returns the text of each line.
func texts(lines []run.Line) []string {
	var got []string
	for _, line := range lines {
		got = append(got, line.Text)
	}
	return got
}
