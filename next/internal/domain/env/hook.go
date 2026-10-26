package env

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// A HookChain is the hooks a new worktree runs, in order, and how they run: in WorkDir, with Env set and the Unset variables removed from the inherited environment, each line of output passed to Output and appended to the log file Log (none when empty). Reap stops, once a hook returns, what it left running in the background, as a terminal's job control would not: exec's commands, never a hook, which may start a server on purpose.
type HookChain struct {
	Hooks   []project.Hook
	WorkDir string
	Env     map[string]string
	Unset   []string
	Output  func(line string)
	Log     string
	Reap    bool
}

// A HookFailure is a hook of a chain that failed: its command, its exit code when it ran to an exit, and the cause.
type HookFailure struct {
	Cmd      string
	ExitCode *int
	Err      error
}

// Error returns the message of the cause.
func (f *HookFailure) Error() string { return f.Err.Error() }

// Unwrap returns the cause, so errors.Is and errors.As see through the failure.
func (f *HookFailure) Unwrap() error { return f.Err }

// The log of a worktree's hooks is hooks/<hooks>-<branch>.log under the state directory.
const (
	logDir = "hooks"
	logExt = ".log"
)

// The lists of hooks config.toml declares, as their logs name them.
const (
	OnCreateHooks = "on_create"
	OnCleanHooks  = "on_clean"
)

// HookLogPathParams names a branch's hook log: the project's state directory, the branch and the list of hooks it logs (OnCreateHooks, OnCleanHooks).
type HookLogPathParams struct {
	StateDir string
	Branch   string
	Hooks    string
}

// HookLogPath returns the path of the log the hooks of the branch write to, with the branch escaped into one file name; it is empty when there is no state directory.
func HookLogPath(params HookLogPathParams) string {
	if params.StateDir == "" {
		return ""
	}
	return filepath.Join(params.StateDir, logDir, params.Hooks+"-"+worktree.StateSegment(params.Branch)+logExt)
}

// HookVars are the values a hook's {{worktree}}, {{branch}}, {{root}} and {{from_branch}} placeholders stand for.
type HookVars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

// Hook returns hook with its placeholders replaced: quoted for the shell in its command, as they are in its working directory.
func (v HookVars) Hook(hook project.Hook) project.Hook {
	hook.Cmd = v.shell(hook.Cmd)
	hook.Cwd = v.plain(hook.Cwd)
	return hook
}

// plain returns s with its placeholders replaced by their values as they are.
func (v HookVars) plain(s string) string {
	return strings.NewReplacer(v.placeholders()...).Replace(s)
}

// shell returns the shell command s with its placeholders replaced by their values, each quoted for where it stands — unquoted, inside single quotes or inside double quotes — so a value is always one word the shell reads literally.
func (v HookVars) shell(s string) string {
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
		// An escaped character is copied with its backslash and never opens or closes a quote; inside single quotes a backslash escapes nothing.
		if c == '\\' && state != singleQuoted && i < len(s) {
			b.WriteByte(s[i])
			i++
			continue
		}
		state = state.after(c)
	}
	return b.String()
}

// placeholders returns each placeholder followed by its value, the shape strings.NewReplacer takes.
func (v HookVars) placeholders() []string {
	return []string{
		"{{worktree}}", v.Worktree,
		"{{branch}}", v.Branch,
		"{{root}}", v.Root,
		"{{from_branch}}", v.FromBranch,
	}
}

// placeholderAt returns the value of the placeholder s starts with and the placeholder's width, and reports whether s starts with one.
func placeholderAt(s string, pairs []string) (string, int, bool) {
	for pair := range slices.Chunk(pairs, 2) {
		if strings.HasPrefix(s, pair[0]) {
			return pair[1], len(pair[0]), true
		}
	}
	return "", 0, false
}

// quoting is the quote a shell command is inside at some point as it is read.
type quoting int

const (
	unquoted quoting = iota
	singleQuoted
	doubleQuoted
)

// after returns the quoting once the byte c has been read in quoting q.
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

// doubleQuoteEscaper escapes the characters a shell still interprets inside double quotes.
var doubleQuoteEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "$", `\$`, "`", "\\`")

// quoteFor returns value quoted so the shell reads it literally where state says it stands.
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

// shellQuote returns value as one shell word: as it is when every character is safe unquoted, else in single quotes.
func shellQuote(value string) string {
	if value != "" && strings.IndexFunc(value, unsafeInShell) < 0 {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// unsafeInShell reports whether the shell may give r a meaning when it stands unquoted.
func unsafeInShell(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return false
	}
	return !strings.ContainsRune("_@%+=:,./-", r)
}
