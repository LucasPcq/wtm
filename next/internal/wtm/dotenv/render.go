package dotenv

import "strings"

func Render(lines []Line) []byte {
	crlf := endsInCRLF(lines)
	parts := make([]string, len(lines))
	for i, line := range lines {
		part := line.Raw
		if line.Kind == Pair && line.Raw == "" {
			part = renderPair(line)
		}
		if crlf && i < len(lines)-1 && !strings.HasSuffix(part, carriageRet) {
			part += carriageRet
		}
		parts[i] = part
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
	if strings.Contains(value, interpolation) && !strings.ContainsRune(value, singleQuote) {
		return string(singleQuote) + value + string(singleQuote)
	}
	if !needsQuote(value) {
		return value
	}
	return string(doubleQuote) + strings.ReplaceAll(value, `"`, `\"`) + string(doubleQuote)
}

func needsQuote(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, " \t\n\"") {
		return true
	}
	return isQuote(value[0]) || value[0] == '#'
}
