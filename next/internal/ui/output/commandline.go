package output

import (
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

const shellSpecial = " \t\n'\"$`\\|&;<>()*?[]{}~#!"

// ApplyNote is the note beside the command that applies the plan a dry run just showed.
const ApplyNote = "apply this plan"

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

// A CommandLine spells a follow-up as the command line that carries it out, word by word, unquoted: the one function that knows every command, which cmd/wtm-next gives.
type CommandLine func(followUp domain.FollowUp) []string

// A View is what a command draws its outcome from beside the outcome: the directory its paths are shown from, and how its follow-ups are spelled.
type View struct {
	Cwd         string
	CommandLine CommandLine
}

// FollowUpStep is followUp as a next step: its command line, spelled by spell, and its message.
func FollowUpStep(followUp domain.FollowUp, spell CommandLine) NextStep {
	return NextStep{Command: Line(spell(followUp)), Note: followUp.Message}
}

// FollowUpSteps returns the next step that repairs what err left behind, its follow-up spelled by spell; none when err is nil or has no follow-up.
func FollowUpSteps(err *domain.Error, spell CommandLine) []NextStep {
	if err == nil || err.FollowUp == nil {
		return nil
	}
	return []NextStep{FollowUpStep(*err.FollowUp, spell)}
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
