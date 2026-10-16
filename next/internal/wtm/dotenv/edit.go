package dotenv

import (
	"slices"
	"strings"
)

func (l Line) WithValue(value string) Line {
	if l.Value == value {
		return l
	}
	span, located := l.valueSpan()
	edited := l
	edited.Value = value
	if !located {
		edited.Raw = ""
		return edited
	}
	prefix, suffix := l.Raw[:span.start], l.Raw[span.end:]
	if kept := span.quoted(value); kept != "" && reparsesTo(prefix+kept+suffix, edited) {
		edited.Raw = prefix + kept + suffix
		return edited
	}
	edited.Raw = prefix + renderValue(value) + suffix
	return edited
}

type SetParams struct {
	Lines []Line
	Key   string
	Value string
}

func Set(params SetParams) ([]Line, bool) {
	for i, line := range params.Lines {
		if line.Kind != Pair || line.Key != params.Key {
			continue
		}
		edited := line.WithValue(params.Value)
		if line.Value == params.Value || edited.Raw == line.Raw {
			return params.Lines, false
		}
		out := slices.Clone(params.Lines)
		out[i] = edited
		return out, true
	}
	return params.appended(), true
}

func (params SetParams) appended() []Line {
	end := len(params.Lines)
	for end > 0 && params.Lines[end-1].Kind == Blank {
		end--
	}
	for at := end; at >= 0; at-- {
		out := params.insertedAt(at)
		if params.heldBy(out) {
			return out
		}
	}
	return params.insertedAt(end)
}

func (params SetParams) insertedAt(at int) []Line {
	out := slices.Insert(slices.Clone(params.Lines), at, Line{Kind: Pair, Key: params.Key, Value: params.Value})
	previous := at - 1
	if at == len(params.Lines) && previous >= 0 && out[previous].Raw != "" && endsInCRLF(params.Lines) && !strings.HasSuffix(out[previous].Raw, carriageRet) {
		out[previous].Raw += carriageRet
	}
	return out
}

func (params SetParams) heldBy(lines []Line) bool {
	pairs, held := 0, false
	for _, line := range Parse(Render(lines)) {
		if line.Kind != Pair {
			continue
		}
		pairs++
		held = held || line.Key == params.Key && line.Value == params.Value
	}
	return held && pairs == countPairs(lines)
}

func countPairs(lines []Line) int {
	count := 0
	for _, line := range lines {
		if line.Kind == Pair {
			count++
		}
	}
	return count
}

type valueSpan struct {
	start, end int
	quote      byte
	dollar     bool
	spaced     bool
}

func (s valueSpan) quoted(value string) string {
	if s.quote != 0 {
		return string(s.quote) + value + string(s.quote)
	}
	if strings.Contains(value, interpolation) && !s.dollar {
		return ""
	}
	if needsQuote(value) && !s.spaced {
		return ""
	}
	return value
}

func (l Line) valueSpan() (valueSpan, bool) {
	eq := strings.Index(l.Raw, assign)
	if l.Kind != Pair || eq < 0 {
		return valueSpan{}, false
	}
	dollar := strings.Contains(l.Value, interpolation)
	rest := l.Raw[eq+1:]
	start := eq + 1 + len(rest) - len(strings.TrimLeft(rest, " \t"))
	if start < len(l.Raw) && isQuote(l.Raw[start]) {
		return l.quotedSpan(start)
	}
	if l.Value == "" {
		return valueSpan{start: eq + 1, end: eq + 1}, true
	}
	end := start + len(l.Value)
	if end > len(l.Raw) || l.Raw[start:end] != l.Value {
		return valueSpan{}, false
	}
	return valueSpan{start: start, end: end, dollar: dollar, spaced: strings.ContainsAny(l.Value, " \t")}, true
}

func (l Line) quotedSpan(start int) (valueSpan, bool) {
	quote := l.Raw[start]
	end := start + 1 + len(l.Value)
	if end >= len(l.Raw) || l.Raw[start+1:end] != l.Value || l.Raw[end] != quote {
		return valueSpan{}, false
	}
	return valueSpan{start: start, end: end + 1, quote: quote, dollar: strings.Contains(l.Value, interpolation)}, true
}

func reparsesTo(raw string, want Line) bool {
	parsed := Parse([]byte(raw))
	if len(parsed) != 1 {
		return false
	}
	got := parsed[0]
	return got.Kind == Pair && got.Key == want.Key && got.Value == want.Value && got.Export == want.Export
}
