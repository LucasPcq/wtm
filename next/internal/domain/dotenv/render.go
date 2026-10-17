package dotenv

import "strings"

// Render returns lines as a .env file: each line's Raw, or for a pair without one, the pair rendered in the file's line endings.
func Render(lines []Line) []byte {
	crlf := endsInCRLF(lines)
	parts := make([]string, len(lines))
	for i, line := range lines {
		parts[i] = line.Raw
		if line.Kind != Pair || line.Raw != "" {
			continue
		}
		parts[i] = renderPair(line)
		if crlf && i < len(lines)-1 {
			parts[i] += carriageRet
		}
	}

	return []byte(strings.Join(parts, "\n"))
}

// endsInCRLF reports whether at least half the terminated lines of the file end in CRLF, so a line wtm adds follows them.
func endsInCRLF(lines []Line) bool {
	crlf, terminated := 0, 0
	for _, line := range lines[:max(len(lines)-1, 0)] {
		if line.Raw == "" {
			continue
		}
		terminated++
		if strings.HasSuffix(line.Raw, carriageRet) {
			crlf++
		}
	}

	return crlf > 0 && crlf*2 >= terminated
}

// renderPair returns the line spelling the pair.
func renderPair(line Line) string {
	prefix := ""
	if line.Export {
		prefix = exportPrefix
	}

	return prefix + line.Key + assign + renderValue(line.Value)
}

// renderValue returns value in the first form that parses back to it: bare, double-quoted, single-quoted — single-quoted first when it holds a `$`, so no tool interpolates it — else double-quoted with its quotes escaped.
func renderValue(value string) string {
	single := string(singleQuote) + value + string(singleQuote)
	double := string(doubleQuote) + value + string(doubleQuote)
	forms := []string{value, double, single}
	if strings.Contains(value, interpolation) {
		forms = []string{single, value, double}
	}

	for _, form := range forms {
		if readsAs(form, value) {
			return form
		}
	}

	return string(doubleQuote) + strings.ReplaceAll(value, `"`, `\"`) + string(doubleQuote)
}

// readsAs reports whether form, as the value of a pair, parses back to value; a bare form other tools would read differently does not.
func readsAs(form, value string) bool {
	if form == value && unsafeBare(value) {
		return false
	}

	parsed := Parse([]byte("K=" + form))
	return len(parsed) == 1 && parsed[0].Kind == Pair && parsed[0].Value == value
}

// needsQuote reports whether value cannot be written bare.
func needsQuote(value string) bool {
	return unsafeBare(value) || strings.Contains(value, `"`)
}

// unsafeBare reports whether value written bare would be misread: it holds a blank or a newline, or starts with a quote or `#`.
func unsafeBare(value string) bool {
	if value == "" {
		return false
	}

	return strings.ContainsAny(value, " \t\n") || isQuote(value[0]) || value[0] == '#'
}
