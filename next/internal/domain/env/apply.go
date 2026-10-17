package env

import (
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
)

const collisionHorizon = 20

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

func reducePortValue(params reducePortParams) string {
	value := reduceOriginValue(reduceOriginParams{Value: params.Value, JobLabel: params.JobLabel, Project: params.Project, Base: params.Base})
	if value != params.Value {
		return value
	}
	var at []number
	for _, n := range numbers(params.Value) {
		gap := n.value - params.Base
		if gap >= 0 && gap%params.Block == 0 && gap/params.Block <= collisionHorizon {
			at = append(at, n)
		}
	}
	if len(at) != 1 {
		return params.Value
	}
	return replaceAt(replaceParams{Value: params.Value, At: at[0].at, Width: at[0].width, To: params.Base})
}

type number struct {
	at    int
	width int
	value int
}

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

func isDigitAt(value string, i int) bool {
	return i >= 0 && i < len(value) && value[i] >= '0' && value[i] <= '9'
}

type replaceParams struct {
	Value string
	At    int
	Width int
	To    int
}

func replaceAt(params replaceParams) string {
	return params.Value[:params.At] + strconv.Itoa(params.To) + params.Value[params.At+params.Width:]
}
