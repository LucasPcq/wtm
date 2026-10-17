package dotenv

import "strings"

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

func renderPair(line Line) string {
	prefix := ""
	if line.Export {
		prefix = exportPrefix
	}
	return prefix + line.Key + assign + renderValue(line.Value)
}

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

func readsAs(form, value string) bool {
	if form == value && unsafeBare(value) {
		return false
	}
	parsed := Parse([]byte("K=" + form))
	return len(parsed) == 1 && parsed[0].Kind == Pair && parsed[0].Value == value
}

func needsQuote(value string) bool {
	return unsafeBare(value) || strings.Contains(value, `"`)
}

func unsafeBare(value string) bool {
	if value == "" {
		return false
	}
	return strings.ContainsAny(value, " \t\n") || isQuote(value[0]) || value[0] == '#'
}
