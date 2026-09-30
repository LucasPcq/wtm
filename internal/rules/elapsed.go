package rules

import (
	"strconv"
	"strings"
	"time"
)

// ParseElapsed reads the `etime` field of ps — `[[dd-]hh:]mm:ss` — into a
// duration. Anything it cannot read in full is refused rather than approximated:
// the value decides whether a process group is the one an index recorded or one
// whose id was recycled, and a guess there signals a stranger.
func ParseElapsed(field string) (time.Duration, bool) {
	rest := strings.TrimSpace(field)
	if rest == "" {
		return 0, false
	}

	days := 0
	if dash := strings.IndexByte(rest, '-'); dash >= 0 {
		parsed, ok := parseElapsedField(rest[:dash])
		if !ok {
			return 0, false
		}
		days = parsed
		rest = rest[dash+1:]
	}

	parts := strings.Split(rest, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return 0, false
	}
	if days > 0 && len(parts) != 3 {
		return 0, false
	}

	total := time.Duration(days) * 24 * time.Hour
	for _, unit := range []time.Duration{time.Hour, time.Minute, time.Second}[3-len(parts):] {
		value, ok := parseElapsedField(parts[0])
		if !ok {
			return 0, false
		}
		total += time.Duration(value) * unit
		parts = parts[1:]
	}
	return total, true
}

func parseElapsedField(field string) (int, bool) {
	value, err := strconv.Atoi(field)
	if err != nil || value < 0 {
		return 0, false
	}
	return value, true
}
