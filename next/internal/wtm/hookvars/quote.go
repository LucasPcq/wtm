package hookvars

import "strings"

type quoting int

const (
	unquoted quoting = iota
	singleQuoted
	doubleQuoted
)

func (q quoting) after(c byte) quoting {
	switch {
	case c == '\'' && q == unquoted:
		return singleQuoted
	case c == '\'' && q == singleQuoted:
		return unquoted
	case c == '"' && q == unquoted:
		return doubleQuoted
	case c == '"' && q == doubleQuoted:
		return unquoted
	}
	return q
}

var doubleQuoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")

func quoteFor(value string, state quoting) string {
	switch state {
	case singleQuoted:
		return strings.ReplaceAll(value, "'", `'\''`)
	case doubleQuoted:
		return doubleQuoteEscaper.Replace(value)
	case unquoted:
	}
	return shellQuote(value)
}

func shellQuote(value string) string {
	if value != "" && strings.IndexFunc(value, unsafeInShell) < 0 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func unsafeInShell(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("_@%+=:,./-", r)
}
