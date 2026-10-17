package env

import (
	"net/url"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

type HookChain struct {
	Hooks   []project.Hook
	WorkDir string
	Env     map[string]string
	Unset   []string
	Output  func(line string)
	Log     string
}

type HookFailure struct {
	Cmd      string
	ExitCode *int
	Err      error
}

func (f *HookFailure) Error() string { return f.Err.Error() }
func (f *HookFailure) Unwrap() error { return f.Err }

const (
	logDir   = "hooks"
	logPhase = "on_create"
	logExt   = ".log"
)

type HookLogPathParams struct {
	StateDir string
	Branch   string
}

func HookLogPath(params HookLogPathParams) string {
	if params.StateDir == "" {
		return ""
	}
	return filepath.Join(params.StateDir, logDir, logPhase+"-"+url.PathEscape(params.Branch)+logExt)
}

type HookVars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

func (v HookVars) Hook(hook project.Hook) project.Hook {
	hook.Cmd = v.Shell(hook.Cmd)
	hook.Cwd = v.Plain(hook.Cwd)
	return hook
}

func (v HookVars) Plain(s string) string {
	return strings.NewReplacer(v.placeholders()...).Replace(s)
}

func (v HookVars) Shell(s string) string {
	pairs := v.placeholders()
	var b strings.Builder
	state := unquoted
	for i := 0; i < len(s); {
		if value, width, found := placeholderAt(s[i:], pairs); found {
			b.WriteString(quoteFor(value, state))
			i += width
			continue
		}
		c := s[i]
		b.WriteByte(c)
		i++
		if c == '\\' && state != singleQuoted && i < len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		state = state.after(c)
	}
	return b.String()
}

func (v HookVars) placeholders() []string {
	return []string{
		"{{worktree}}", v.Worktree,
		"{{branch}}", v.Branch,
		"{{root}}", v.Root,
		"{{from_branch}}", v.FromBranch,
	}
}

func placeholderAt(s string, pairs []string) (string, int, bool) {
	for pair := range slices.Chunk(pairs, 2) {
		if strings.HasPrefix(s, pair[0]) {
			return pair[1], len(pair[0]), true
		}
	}
	return "", 0, false
}

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
