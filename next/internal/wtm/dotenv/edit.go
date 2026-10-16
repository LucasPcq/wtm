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
		if line.Value == params.Value {
			return params.Lines, false
		}
		out := slices.Clone(params.Lines)
		out[i] = line.WithValue(params.Value)
		return out, true
	}
	at := len(params.Lines)
	for at > 0 && params.Lines[at-1].Kind == Blank {
		at--
	}
	return slices.Insert(slices.Clone(params.Lines), at, Line{Kind: Pair, Key: params.Key, Value: params.Value}), true
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
