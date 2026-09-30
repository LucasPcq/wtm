package rules

import (
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// TemplateVars holds the variables available for interpolation in hook commands.
type TemplateVars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

// ResolveTemplateVars substitutes template variables in a hook command's Cmd and
// Cwd fields. Cmd is a /bin/sh line, so each value is quoted for the spot it
// lands in; Cwd is a path handed to the OS as is.
func ResolveTemplateVars(hook domain.HookCommand, vars TemplateVars) domain.HookCommand {
	hook.Cmd = InterpolateShell(hook.Cmd, vars)
	hook.Cwd = Interpolate(hook.Cwd, vars)
	return hook
}

func placeholders(vars TemplateVars) []string {
	return []string{
		"{{worktree}}", vars.Worktree,
		"{{branch}}", vars.Branch,
		"{{root}}", vars.Root,
		"{{from_branch}}", vars.FromBranch,
	}
}

// Interpolate replaces {{worktree}}, {{branch}}, {{root}}, {{from_branch}} in s.
func Interpolate(s string, vars TemplateVars) string {
	if s == "" {
		return s
	}
	return strings.NewReplacer(placeholders(vars)...).Replace(s)
}

type shellQuoting int

const (
	quotingNone shellQuoting = iota
	quotingSingle
	quotingDouble
)

// InterpolateShell is Interpolate for a shell line: a value is data, never
// syntax, so a quote or a `$` in a path reaches the command as spelled on disk.
// The quoting follows the one the template put around the placeholder, so a
// hook already written as `cd "{{worktree}}"` keeps working instead of gaining
// literal quotes.
func InterpolateShell(s string, vars TemplateVars) string {
	pairs := placeholders(vars)
	var b strings.Builder
	state := quotingNone
	for i := 0; i < len(s); {
		if value, width, found := placeholderAt(s[i:], pairs); found {
			b.WriteString(quoteFor(value, state))
			i += width
			continue
		}
		c := s[i]
		b.WriteByte(c)
		i++
		switch {
		case c == '\\' && state != quotingSingle && i < len(s):
			b.WriteByte(s[i])
			i++
		case c == '\'' && state == quotingNone:
			state = quotingSingle
		case c == '\'' && state == quotingSingle:
			state = quotingNone
		case c == '"' && state == quotingNone:
			state = quotingDouble
		case c == '"' && state == quotingDouble:
			state = quotingNone
		}
	}
	return b.String()
}

func placeholderAt(s string, pairs []string) (value string, width int, found bool) {
	for i := 0; i < len(pairs); i += 2 {
		if strings.HasPrefix(s, pairs[i]) {
			return pairs[i+1], len(pairs[i]), true
		}
	}
	return "", 0, false
}

func quoteFor(value string, state shellQuoting) string {
	switch state {
	case quotingSingle:
		return strings.ReplaceAll(value, "'", `'\''`)
	case quotingDouble:
		return doubleQuoteEscaper.Replace(value)
	default:
		return shellQuote(value)
	}
}

var doubleQuoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")

// shellQuote renders value as one shell word, untouched when it holds nothing
// the shell would read.
func shellQuote(value string) string {
	if value != "" && strings.IndexFunc(value, isUnsafeShellRune) < 0 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func isUnsafeShellRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("_@%+=:,./-", r)
}
