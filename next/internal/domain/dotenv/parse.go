// Package dotenv parses and renders .env files without losing a byte of what the user wrote.
package dotenv

import "strings"

// Kind is what a line of a .env file holds.
type Kind string

// The kinds of line; a line that is neither blank nor a valid pair is kept as a comment.
const (
	Pair    Kind = "pair"
	Comment Kind = "comment"
	Blank   Kind = "blank"
)

// Line is one line of a .env file, a pair possibly spanning several physical lines: its kind, its key and unquoted value for a pair, whether it starts with `export`, and its bytes as written (Raw, without the newline).
type Line struct {
	Kind   Kind
	Key    string
	Value  string
	Export bool
	Raw    string
}

const (
	commentPrefix = "#"
	exportPrefix  = "export "
	assign        = "="
	doubleQuote   = '"'
	singleQuote   = '\''
	carriageRet   = "\r"
	interpolation = "$"
	byteOrderMark = string('\ufeff')
)

// Parse returns the lines of a .env file; rendering them back with Render gives data byte for byte.
func Parse(data []byte) []Line {
	text, marked := strings.CutPrefix(string(data), byteOrderMark)
	physical := strings.Split(text, "\n")
	lines := make([]Line, 0, len(physical))
	for i := 0; i < len(physical); i++ {
		line, extra := classify(physical, i)
		lines = append(lines, line)
		i += extra
	}

	if marked {
		lines[0].Raw = byteOrderMark + lines[0].Raw
	}

	return lines
}

// classify returns the line that starts at physical[i], and how many more physical lines it spans.
func classify(physical []string, i int) (Line, int) {
	raw := physical[i]
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return Line{Kind: Blank, Raw: raw}, 0
	}
	if strings.HasPrefix(trimmed, commentPrefix) {
		return Line{Kind: Comment, Raw: raw}, 0
	}
	if pair, extra, ok := parsePair(physical, i); ok {
		return pair, extra
	}

	return Line{Kind: Comment, Raw: raw}, 0
}

// parsePair returns the pair that starts at physical[start], how many more physical lines its value spans, and reports whether the line is a valid pair.
func parsePair(physical []string, start int) (Line, int, bool) {
	first := physical[start]
	content := strings.TrimLeft(first, " \t")
	after, export := strings.CutPrefix(content, exportPrefix)
	if export {
		content = strings.TrimLeft(after, " \t")
	}

	before, rest, found := strings.Cut(content, assign)
	if !found {
		return Line{}, 0, false
	}

	key := strings.TrimSpace(before)
	if !validKey(key) {
		return Line{}, 0, false
	}

	value, extra, ok := parseValue(valueText{rest: rest, physical: physical, start: start})
	if !ok {
		return Line{}, 0, false
	}

	raw := strings.Join(physical[start:start+extra+1], "\n")
	return Line{Kind: Pair, Key: key, Value: value, Export: export, Raw: raw}, extra, true
}

// valueText is what follows the `=` of the pair at physical[start].
type valueText struct {
	rest     string
	physical []string
	start    int
}

// parseValue returns the value of text, how many more physical lines it spans, and reports whether it is well formed (a quoted value is closed).
func parseValue(text valueText) (string, int, bool) {
	trimmed := strings.TrimLeft(text.rest, " \t")
	if trimmed == "" {
		return "", 0, true
	}

	if isQuote(trimmed[0]) {
		return parseQuoted(quotedText{first: trimmed, quote: trimmed[0], physical: text.physical, start: text.start})
	}

	return unquotedValue(text.rest), 0, true
}

// quotedText is a quoted value: first is its first physical line from the opening quote, which is quote.
type quotedText struct {
	first    string
	quote    byte
	physical []string
	start    int
}

// parseQuoted returns the value between the quotes, across physical lines until the closing one, how many more lines it spans, and reports whether it is closed. Escapes are kept as written inside double quotes, so a value round-trips.
func parseQuoted(text quotedText) (string, int, bool) {
	escapes := text.quote == doubleQuote
	var value strings.Builder
	cur := text.first
	line := text.start
	i := 1

	for {
		for i < len(cur) {
			c := cur[i]
			if escapes && c == '\\' && i+1 < len(cur) {
				value.WriteByte(c)
				value.WriteByte(cur[i+1])
				i += 2
				continue
			}
			if c == text.quote {
				return value.String(), line - text.start, true
			}
			value.WriteByte(c)
			i++
		}

		line++
		if line >= len(text.physical) {
			return "", 0, false
		}

		value.WriteByte('\n')
		cur = text.physical[line]
		i = 0
	}
}

// unquotedValue returns an unquoted value without its surrounding blanks and the comment a blank and `#` start.
func unquotedValue(rest string) string {
	for i := 1; i < len(rest); i++ {
		if rest[i] == '#' && (rest[i-1] == ' ' || rest[i-1] == '\t') {
			return strings.TrimSpace(rest[:i])
		}
	}

	return strings.TrimSpace(rest)
}

// validKey reports whether key is a valid variable name: letters, `_`, `.` and digits, not starting with a digit.
func validKey(key string) bool {
	if key == "" {
		return false
	}

	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_', c == '.':
		case c >= '0' && c <= '9' && i > 0:
		default:
			return false
		}
	}

	return true
}

func isQuote(c byte) bool {
	return c == doubleQuote || c == singleQuote
}
