package output

import (
	"strconv"
	"time"
)

// Span returns d in its largest unit, rounded down: seconds, minutes, hours or days (12m, 3h).
func Span(d time.Duration) string {
	switch {
	case d < time.Minute:
		return strconv.Itoa(max(int(d.Seconds()), 0)) + "s"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + "m"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + "h"
	}
	return strconv.Itoa(int(d.Hours()/24)) + "d"
}

// Counted is n of a thing, its noun one or many as n says: "1 worktree", "3 worktrees".
func Counted(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
