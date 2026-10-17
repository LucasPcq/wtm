package env

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
)

// CollisionHorizon is how many offset blocks above a base port a number may sit and still be read as that port shifted by an earlier worktree; a collision of two ports further apart than that many worktrees is not reported, since no project reaches it.
const CollisionHorizon = 20

// applyPorts returns lines with the value of every key an entry rewrites replaced by its new value; lines no entry rewrites are returned as they are.
func applyPorts(lines []dotenv.Line, entries []Entry) []dotenv.Line {
	byKey := map[string]string{}
	for _, entry := range entries {
		if entry.Status == StatusRewrite {
			byKey[entry.Key] = entry.NewValue
		}
	}
	if len(byKey) == 0 {
		return lines
	}

	out := make([]dotenv.Line, len(lines))
	for i, line := range lines {
		value, rewritten := byKey[line.Key]
		out[i] = line
		if line.Kind == dotenv.Pair && rewritten {
			out[i] = line.WithValue(value)
		}
	}
	return out
}

type reducePortParams struct {
	Value    string
	Base     int
	Block    int
	JobLabel string
	Project  string
}

// reducePortValue brings a value already settled for some worktree back to the base port, so it can be settled again for this one: a route of the job is rewound to its local origin, else the single number sitting a whole number of blocks above the base becomes the base. A value with no such number, or with several, is returned as it is.
func reducePortValue(params reducePortParams) string {
	value := reduceOriginValue(reduceOriginParams{Value: params.Value, JobLabel: params.JobLabel, Project: params.Project, Base: params.Base})
	if value != params.Value {
		return value
	}

	var at []number
	for _, n := range numbers(params.Value) {
		gap := n.value - params.Base
		if gap >= 0 && gap%params.Block == 0 && gap/params.Block <= CollisionHorizon {
			at = append(at, n)
		}
	}
	if len(at) != 1 {
		return params.Value
	}

	return replaceAt(replaceParams{Value: params.Value, At: at[0].at, Width: at[0].width, To: params.Base})
}

// A number is a run of digits found in a value: its byte offset, its width and what it reads as.
type number struct {
	at    int
	width int
	value int
}

// numbers returns every run of digits in value, in order.
func numbers(value string) []number {
	var found []number
	for i := 0; i < len(value); {
		if !isDigitAt(value, i) {
			i++
			continue
		}

		end := i
		for isDigitAt(value, end) {
			end++
		}
		if n, err := strconv.Atoi(value[i:end]); err == nil {
			found = append(found, number{at: i, width: end - i, value: n})
		}
		i = end
	}
	return found
}

// portOffsets returns the byte offsets at which port appears in value as a whole number, never as part of a longer one.
func portOffsets(value string, port int) []int {
	needle := strconv.Itoa(port)
	var offsets []int
	for from := 0; ; {
		i := strings.Index(value[from:], needle)
		if i < 0 {
			return offsets
		}

		at := from + i
		if !isDigitAt(value, at-1) && !isDigitAt(value, at+len(needle)) {
			offsets = append(offsets, at)
		}
		from = at + len(needle)
	}
}

// isDigitAt reports whether value holds an ASCII digit at index i; an index out of range holds none.
func isDigitAt(value string, i int) bool {
	return i >= 0 && i < len(value) && value[i] >= '0' && value[i] <= '9'
}

type replaceParams struct {
	Value string
	At    int
	Width int
	To    int
}

// replaceAt returns Value with the Width bytes at offset At replaced by the number To.
func replaceAt(params replaceParams) string {
	return params.Value[:params.At] + strconv.Itoa(params.To) + params.Value[params.At+params.Width:]
}
