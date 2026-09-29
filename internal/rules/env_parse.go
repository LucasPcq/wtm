package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ParseEnv parses .env content into logical lines, preserving the exact original
// text of each line in Raw so RenderEnv can reproduce it byte-for-byte. It never
// performs I/O. Values are opaque: no variable expansion, interpolation, or escape
// interpretation. A non-blank, non-comment line that is not a valid KEY=VALUE pair
// (including an unterminated quoted value) is preserved verbatim as a comment.
func ParseEnv(content string) []domain.EnvLine {
	physical := strings.Split(content, "\n")
	lines := make([]domain.EnvLine, 0, len(physical))

	for i := 0; i < len(physical); i++ {
		line, extra := classifyLine(physical, i)
		lines = append(lines, line)
		i += extra
	}

	return lines
}

// classifyLine turns physical[i] into one logical line and reports how many extra
// physical lines it consumed (non-zero only for a multiline quoted value). Blank,
// comment, valid pair, then a verbatim-comment fallback for anything else.
func classifyLine(physical []string, i int) (domain.EnvLine, int) {
	raw := physical[i]
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		return domain.EnvLine{Kind: domain.EnvLineBlank, Raw: raw}, 0
	}
	if strings.HasPrefix(trimmed, domain.EnvCommentPrefix) {
		return domain.EnvLine{Kind: domain.EnvLineComment, Raw: raw}, 0
	}
	if pair, extra, ok := parsePair(physical, i); ok {
		return pair, extra
	}
	return domain.EnvLine{Kind: domain.EnvLineComment, Raw: raw}, 0
}

// RenderEnv serializes logical lines back to .env content. A pair whose Raw is
// still present is emitted verbatim; a pair with no Raw (a new one) is rendered
// canonically. A file whose lines end in CRLF keeps doing so, new lines included.
func RenderEnv(lines []domain.EnvLine) string {
	crlf := endsInCRLF(lines)
	parts := make([]string, len(lines))
	for i, l := range lines {
		part := l.Raw
		if l.Kind == domain.EnvLinePair && l.Raw == "" {
			part = renderPair(l)
		}
		if crlf && i < len(lines)-1 && !strings.HasSuffix(part, domain.EnvCR) {
			part += domain.EnvCR
		}
		parts[i] = part
	}
	return strings.Join(parts, "\n")
}

// endsInCRLF reads the document's line ending off its terminated lines: the
// last one is never terminated, so it has no say.
func endsInCRLF(lines []domain.EnvLine) bool {
	crlf, terminated := 0, 0
	for _, l := range lines[:max(len(lines)-1, 0)] {
		if l.Raw == "" {
			continue
		}
		terminated++
		if strings.HasSuffix(l.Raw, domain.EnvCR) {
			crlf++
		}
	}
	return crlf > 0 && crlf*2 >= terminated
}

// WithEnvValue sets a pair's value by replacing it inside the line as written,
// so its quotes, inline comment, export prefix and line ending all survive. The
// line is re-quoted only when its own quoting cannot hold the new value.
func WithEnvValue(line domain.EnvLine, value string) domain.EnvLine {
	if line.Value == value {
		return line
	}
	span, located := valueSpan(line)
	line.Value = value
	if !located {
		line.Raw = ""
		return line
	}

	prefix, suffix := line.Raw[:span.start], line.Raw[span.end:]
	if kept := span.quoted(value); kept != "" {
		if candidate := prefix + kept + suffix; reparsesTo(candidate, line) {
			line.Raw = candidate
			return line
		}
	}
	line.Raw = prefix + renderValue(value) + suffix
	return line
}

// envValueSpan is where a pair's value sits in its raw line, quotes included.
type envValueSpan struct {
	start, end int
	quote      byte
	// dollar says the value it held already carried a `$`, so a bare `$` in its
	// replacement is the file's own interpolation rather than one wtm adds.
	dollar bool
}

func (s envValueSpan) quoted(value string) string {
	if s.quote != 0 {
		return string(s.quote) + value + string(s.quote)
	}
	if needsQuote(value) || (strings.Contains(value, domain.EnvInterpolation) && !s.dollar) {
		return ""
	}
	return value
}

func valueSpan(line domain.EnvLine) (envValueSpan, bool) {
	raw := line.Raw
	eq := strings.Index(raw, domain.EnvAssign)
	if line.Kind != domain.EnvLinePair || eq < 0 {
		return envValueSpan{}, false
	}
	dollar := strings.Contains(line.Value, domain.EnvInterpolation)

	rest := raw[eq+1:]
	start := eq + 1 + len(rest) - len(strings.TrimLeft(rest, " \t"))
	if start < len(raw) && isEnvQuote(raw[start]) {
		quote := raw[start]
		end := start + 1 + len(line.Value)
		if end >= len(raw) || raw[start+1:end] != line.Value || raw[end] != quote {
			return envValueSpan{}, false
		}
		return envValueSpan{start: start, end: end + 1, quote: quote, dollar: dollar}, true
	}
	if line.Value == "" {
		return envValueSpan{start: eq + 1, end: eq + 1}, true
	}
	end := start + len(line.Value)
	if end > len(raw) || raw[start:end] != line.Value {
		return envValueSpan{}, false
	}
	return envValueSpan{start: start, end: end, dollar: dollar}, true
}

func isEnvQuote(c byte) bool {
	return c == byte(domain.EnvQuoteDouble) || c == byte(domain.EnvQuoteSingle)
}

// reparsesTo checks a rewritten line reads back as the pair it is meant to be:
// a value the quoting cannot hold would otherwise be written and misread.
func reparsesTo(raw string, want domain.EnvLine) bool {
	parsed := ParseEnv(raw)
	if len(parsed) != 1 {
		return false
	}
	got := parsed[0]
	return got.Kind == domain.EnvLinePair && got.Key == want.Key && got.Value == want.Value && got.Export == want.Export
}

// parsePair attempts to read physical[start] — spanning following lines when a
// quoted value is unterminated — as a KEY=VALUE pair. It returns the parsed line,
// the number of extra physical lines consumed beyond start, and ok=false when the
// line is not a valid pair.
func parsePair(physical []string, start int) (domain.EnvLine, int, bool) {
	first := physical[start]

	content := strings.TrimLeft(first, " \t")
	export := false
	if strings.HasPrefix(content, domain.EnvExportPrefix) {
		export = true
		content = strings.TrimLeft(content[len(domain.EnvExportPrefix):], " \t")
	}

	eq := strings.Index(content, domain.EnvAssign)
	if eq < 0 {
		return domain.EnvLine{}, 0, false
	}
	key := strings.TrimSpace(content[:eq])
	if !isValidKey(key) {
		return domain.EnvLine{}, 0, false
	}

	value, extra, ok := parseValue(content[eq+1:], physical, start)
	if !ok {
		return domain.EnvLine{}, 0, false
	}

	raw := first
	if extra > 0 {
		raw = strings.Join(physical[start:start+extra+1], "\n")
	}

	return domain.EnvLine{
		Kind:   domain.EnvLinePair,
		Key:    key,
		Value:  value,
		Export: export,
		Raw:    raw,
	}, extra, true
}

// parseValue extracts the opaque value from the text after '='. A quoted value may
// span physical lines; an unquoted value ends at an inline comment (a '#' preceded
// by whitespace). Returns the value, extra lines consumed, and ok.
func parseValue(rest string, physical []string, start int) (string, int, bool) {
	trimmed := strings.TrimLeft(rest, " \t")
	if trimmed == "" {
		return "", 0, true
	}

	quote := trimmed[0]
	if quote == byte(domain.EnvQuoteDouble) || quote == byte(domain.EnvQuoteSingle) {
		return parseQuoted(trimmed, quote, physical, start)
	}
	return unquotedValue(rest), 0, true
}

// parseQuoted reads a quoted value beginning at the opening quote in first, scanning
// following physical lines until the matching closing quote. Double quotes honor a
// backslash escape when locating the end; single quotes are literal. The returned
// value is the inner text verbatim (escapes and embedded newlines preserved).
func parseQuoted(first string, quote byte, physical []string, start int) (string, int, bool) {
	escapes := quote == byte(domain.EnvQuoteDouble)

	var value strings.Builder
	cur := first
	line := start
	i := 1 // skip the opening quote

	for {
		for i < len(cur) {
			c := cur[i]
			if escapes && c == '\\' && i+1 < len(cur) {
				value.WriteByte(c)
				value.WriteByte(cur[i+1])
				i += 2
				continue
			}
			if c == quote {
				return value.String(), line - start, true
			}
			value.WriteByte(c)
			i++
		}

		line++
		if line >= len(physical) {
			return "", 0, false
		}
		value.WriteByte('\n')
		cur = physical[line]
		i = 0
	}
}

// unquotedValue returns the value of an unquoted assignment, dropping a trailing
// inline comment (a '#' preceded by whitespace) and surrounding whitespace.
func unquotedValue(rest string) string {
	for i := 0; i < len(rest); i++ {
		if rest[i] == '#' && i > 0 && (rest[i-1] == ' ' || rest[i-1] == '\t') {
			return strings.TrimSpace(rest[:i])
		}
	}
	return strings.TrimSpace(rest)
}

// isValidKey reports whether key is a plausible env key: non-empty, letters, digits,
// underscore or dot, not starting with a digit. This rejects prose lines that merely
// happen to contain '='.
func isValidKey(key string) bool {
	if key == "" {
		return false
	}
	for i := 0; i < len(key); i++ {
		c := key[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c == '_', c == '.':
		case c >= '0' && c <= '9':
			if i == 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// renderPair emits a mutated pair in canonical form: an optional export prefix, the
// key, '=', and the value quoted only when required to round-trip.
func renderPair(l domain.EnvLine) string {
	var b strings.Builder
	if l.Export {
		b.WriteString(domain.EnvExportPrefix)
	}
	b.WriteString(l.Key)
	b.WriteString(domain.EnvAssign)
	b.WriteString(renderValue(l.Value))
	return b.String()
}

// renderValue quotes a value only when leaving it bare would not parse back to
// the same text. A `$` takes single quotes, where no dotenv reader expands it.
func renderValue(v string) string {
	if strings.Contains(v, domain.EnvInterpolation) && !strings.ContainsRune(v, domain.EnvQuoteSingle) {
		sq := string(domain.EnvQuoteSingle)
		return sq + v + sq
	}
	if !needsQuote(v) {
		return v
	}
	dq := string(domain.EnvQuoteDouble)
	return dq + strings.ReplaceAll(v, dq, `\"`) + dq
}

func needsQuote(v string) bool {
	if v == "" {
		return false
	}
	if strings.ContainsAny(v, " \t\n\"") {
		return true
	}
	switch v[0] {
	case byte(domain.EnvQuoteSingle), byte(domain.EnvQuoteDouble), '#':
		return true
	}
	return false
}
