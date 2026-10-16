// Package hookvars fills the template variables of a hook command, each value quoted for the spot it lands in.
package hookvars

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/project"
)

type Vars struct {
	Worktree   string
	Branch     string
	Root       string
	FromBranch string
}

func (v Vars) Hook(hook project.Hook) project.Hook {
	hook.Cmd = v.Shell(hook.Cmd)
	hook.Cwd = v.Plain(hook.Cwd)
	return hook
}

func (v Vars) Plain(s string) string {
	return strings.NewReplacer(v.placeholders()...).Replace(s)
}

func (v Vars) Shell(s string) string {
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

func (v Vars) placeholders() []string {
	return []string{
		"{{worktree}}", v.Worktree,
		"{{branch}}", v.Branch,
		"{{root}}", v.Root,
		"{{from_branch}}", v.FromBranch,
	}
}

func placeholderAt(s string, pairs []string) (string, int, bool) {
	for i := 0; i < len(pairs); i += 2 {
		if strings.HasPrefix(s, pairs[i]) {
			return pairs[i+1], len(pairs[i]), true
		}
	}
	return "", 0, false
}
