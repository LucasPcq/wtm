package output

import (
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const shellSpecial = " \t\n'\"$`\\|&;<>()*?[]{}~#!"

// The notes of the next steps that replay a dry run: ApplyNote beside the command that applies the plan just shown, ReplanNote beside the dry run to run again when the facts changed.
const (
	ApplyNote  = "apply this plan"
	ReplanNote = "the plan the facts give now"
)

// Quoted returns each of words quoted for the shell.
func Quoted(words []string) []string {
	out := make([]string, 0, len(words))
	for _, word := range words {
		out = append(out, Quote(word))
	}
	return out
}

// Quote returns word as a POSIX shell reads it back: unchanged when no character of it is special, in single quotes otherwise.
func Quote(word string) string {
	if word != "" && !strings.ContainsAny(word, shellSpecial) {
		return word
	}
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

// Line returns the command line of argv, each word quoted for the shell.
func Line(argv []string) string {
	return strings.Join(Quoted(argv), " ")
}

// GoStep is the next step that enters the worktree of branch, the branch quoted for the shell.
func GoStep(branch string) NextStep {
	return NextStep{Command: Line([]string{"wtm", "go", branch})}
}

// FollowUpSteps returns the next step that repairs what err left behind, its follow-up's command line and message; none when err is nil or has no follow-up.
func FollowUpSteps(err *domain.Error) []NextStep {
	if err == nil || err.FollowUp == nil {
		return nil
	}
	return []NextStep{{Command: Line(err.FollowUp.Argv), Note: err.FollowUp.Message}}
}

// Shown is a path to show and the directory the command runs in.
type Shown struct {
	Cwd  string
	Path string
}

// ShownPath returns the path of s relative to its working directory, or the path as it is when it has none or the two share no root.
func ShownPath(s Shown) string {
	relative, err := filepath.Rel(s.Cwd, s.Path)
	if err != nil || s.Cwd == "" {
		return s.Path
	}
	return relative
}
