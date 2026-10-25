package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of extract's form; the target is asked twice, as a worktree to pick (a select), then as the branch of one to create (a text).
func words(q extract.Questions, field domain.Field) form.Words {
	path := field.Spec().Path
	newWorktree := form.NewWorktree{EnvHelp: "Where the new target's .env files come from: config, example, main or parent; config.toml's strategy by default", Configured: q.Configured, Behind: q.Behind, Many: q.Many}
	if said, ok := newWorktree.Words(path); ok {
		return said
	}

	source, to := q.Source.Given, q.To.Given
	switch path {
	case extract.PathSource:
		return form.Words{Flag: "<source>", Label: "Source worktree", Help: "Which worktree to extract changes from"}
	case extract.PathFiles:
		return form.Words{Flag: "--files", Label: "Files", Help: filesHelp(q), Empty: "Select at least one file"}
	case extract.PathTo:
		if _, typed := field.(domain.TextField); typed {
			return form.Words{Flag: "--to", Label: "Branch name", Help: "Name for the new worktree branch"}
		}
		return form.Words{Flag: "--to", Label: "Target worktree", Help: "Target worktree branch; created if it does not exist", Options: map[string]form.OptionWords{
			"": {Label: "+ Create a new worktree…", Short: "new worktree"},
		}}
	case extract.PathFrom:
		return form.Words{Flag: "--from", Label: "Source branch", Help: "Parent branch when creating the target worktree"}
	case extract.PathKeep:
		return form.Words{Flag: "--keep", Label: "Mode", Help: "Copy instead of move (keep the changes in the source)", Options: map[string]form.OptionWords{
			"true":  {Label: "Copy — keep the files in " + source, Short: "copy"},
			"false": {Label: "Move — remove the files from " + source, Short: "move"},
		}}
	case extract.PathOnConflict:
		return form.Words{Flag: "--on-conflict", Label: "On conflict", Help: conflictHelp(q), Options: map[string]form.OptionWords{
			string(extract.OnConflictAbort):   {Label: "Abort — leave both worktrees untouched"},
			string(extract.OnConflictResolve): {Label: "Resolve — write conflict markers in " + to},
		}}
	}

	return form.Words{}
}

func filesHelp(q extract.Questions) string {
	if q.Source.Given != "" && len(q.Files.Options) == 0 {
		return "No changes to extract in " + q.Source.Given + " — press esc to pick another worktree."
	}

	return "Files to extract, or a directory to take everything below it"
}

// conflictHelp names the files that conflict with the target and what each answer leaves behind; while nothing conflicts, it says what each answer does.
func conflictHelp(q extract.Questions) string {
	conflicts := q.OnConflict.Conflicts
	if len(conflicts) == 0 {
		return "On conflict: abort (default) or resolve (write conflict markers in the target)"
	}

	files := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		files = append(files, conflict.Path)
	}

	verb := " were"
	if len(files) == 1 {
		verb = " was"
	}

	target, source := `"`+q.To.Given+`"`, `"`+q.Source.Given+`"`
	return strings.Join(files, ", ") + verb + " also changed in " + target + ".\n\n" +
		"Resolving writes conflict markers there; nothing is removed from the source.\n" +
		"Resolve them in " + target + ", then discard the same files in " + source + "; to undo, discard the applied files in " + target + "."
}
