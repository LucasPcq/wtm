package cli

import (
	"cmp"
	"errors"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// A step is the index of a question of the wizard, in the order asked; stepLabels and stepPaths are indexed by it.
type step int

const (
	stepSource step = iota
	stepFiles
	stepTarget
	stepBranch
	stepFrom
	stepIsolation
	stepFastForward
	stepMode
	stepOnConflict
	stepRecap
	stepCount
)

// stepLabels are the steps' names in the breadcrumb, the trail and the problems the wizard shows.
var stepLabels = [stepCount]string{"Source worktree", "Files", "Target worktree", "Branch name", "Source branch", "Isolation", "Source update", "Mode", "On conflict", "Confirm"}

// stepPaths are the field path each step answers; the branch of a created target is the target's.
var stepPaths = [stepCount]string{"source", "files", "to", "to", "from", "isolation", "fast_forward", "keep", "on_conflict", ""}

// stepDescriptions are the help of the steps whose help does not depend on the answers so far.
var stepDescriptions = [stepCount]string{
	stepSource:      "Which worktree to extract changes from",
	stepFiles:       "Space to toggle, a to select all, / to filter, enter to confirm, esc to go back.",
	stepTarget:      "Where to move the selected files",
	stepBranch:      "Name for the new worktree branch",
	stepFrom:        "Branch to base the new worktree on",
	stepFastForward: "The branch the new worktree starts from is behind origin — fast-forward it before creating?",
	stepMode:        "Move removes the files from the source; copy keeps them.",
}

const (
	createTarget = "\x00new-worktree"
	newWorktree  = "new worktree"
	existingSkip = "the target worktree exists: nothing is created"
	yes          = "yes"
	no           = "no"
	modeMove     = "move"
	modeCopy     = "copy"
	emptyFiles   = "select at least one file"
)

// description is the help of step s on the answers so far.
func (w *wizard) description(s step) string {
	switch {
	case s == stepFiles && len(w.q.Files.Options) == 0:
		return "No changes to extract in " + w.req.Source + " — press esc to pick another worktree."
	case s == stepIsolation:
		return descriptions["isolation"]
	case s == stepOnConflict:
		return w.conflictDescription()
	}

	return stepDescriptions[s]
}

// conflictDescription names the files that conflict with the target and what each answer leaves behind.
func (w *wizard) conflictDescription() string {
	conflicts := w.q.OnConflict.Conflicts
	files := make([]string, 0, len(conflicts))
	for _, conflict := range conflicts {
		files = append(files, conflict.Path)
	}

	verb := " were"
	if len(files) == 1 {
		verb = " was"
	}

	target, source := `"`+w.req.To+`"`, `"`+w.req.Source+`"`
	return strings.Join(files, ", ") + verb + " also changed in " + target + ".\n\n" +
		"Resolving writes conflict markers there; nothing is removed from the source.\n" +
		"Resolve them in " + target + ", then discard the same files in " + source + "; to undo, discard the applied files in " + target + "."
}

// standing is where the answer of step s stands on the questions observed last, with the flag that set it, if one did.
func (w *wizard) standing(s step) form.Standing {
	q, given, creating := w.q, w.given, w.creating()
	st, flagged := form.Standing{}, false
	switch s {
	case stepSource:
		st, flagged = form.Standing{Value: q.Source.Given}, given.Source != ""
	case stepFiles:
		st, flagged = form.Standing{Value: w.fileSummary()}, len(given.Files) > 0
	case stepTarget:
		st, flagged = form.Standing{Value: w.req.To}, given.To != ""
		if creating {
			st.Value = newWorktree
		}
	case stepBranch:
		st, flagged = form.Standing{Value: w.req.To}, given.To != "" && creating
	case stepFrom:
		st, flagged = form.Standing{Value: q.From.Value, Origin: q.From.Origin}, given.From != ""
	case stepIsolation:
		st, flagged = form.Standing{Value: string(q.Isolation.Value), Origin: q.Isolation.Origin, Skip: q.Isolation.Skip}, given.Isolation != ""
	case stepFastForward:
		st, flagged = form.Standing{Value: yesNo(q.FastForward.Value), Origin: q.FastForward.Origin, Skip: q.FastForward.Skip}, given.FastForward != nil
	case stepMode:
		st, flagged = form.Standing{Value: mode(q.Keep.Value), Origin: q.Keep.Origin}, given.Keep != nil
	case stepOnConflict:
		st, flagged = form.Standing{Value: string(q.OnConflict.Value), Origin: q.OnConflict.Origin, Skip: q.OnConflict.Skip}, given.OnConflict != ""
	case stepRecap, stepCount:
	}

	// The branch is the target's: its problems are the target step's while an existing worktree is picked, the branch step's once one is created.
	st.Erred = w.erred(stepPaths[s]) && (stepPaths[s] != "to" || (s == stepBranch) == creating)
	if (s == stepBranch || s == stepFrom) && !creating {
		st.Skip = existingSkip
	}
	if flagged {
		st.Flag = extractFlags.Spell(stepPaths[s])
	}

	return st
}

// fileSummary counts the changes the files answered stand for, a directory standing for every change under it.
func (w *wizard) fileSummary() string {
	if len(w.req.Files) == 0 {
		return ""
	}

	selected, missed := worktree.SelectChanges(w.q.Files.Options, w.req.Files)
	if missed != "" {
		return fileCount(len(w.req.Files))
	}

	return fileCount(len(selected))
}

// erred reports whether a problem names the field at path or one of its entries.
func (w *wizard) erred(path string) bool {
	return slices.ContainsFunc(w.q.Errors, func(problem domain.FieldError) bool {
		return problem.Path == path || strings.HasPrefix(problem.Path, path+"[")
	})
}

// list offers the options of the select step s, starting on the answer so far.
func (w *wizard) list(s step) *form.SelectList {
	q, req := w.q, w.req
	switch s {
	case stepSource:
		return form.NewSelectList(form.SelectParams{Options: optionsOf(q.Source.Options), Start: req.Source})
	case stepTarget:
		options := append([]form.Option{{Label: "+ Create a new worktree…", Value: createTarget}, {Separator: true}}, optionsOf(q.To.Options)...)
		start := req.To
		if w.creating() {
			start = createTarget
		}
		return form.NewSelectList(form.SelectParams{Options: options, Start: start})
	case stepFrom:
		return w.fromList()
	case stepIsolation:
		options := make([]form.Option, 0, len(q.Isolation.Options))
		for _, value := range domain.Names(q.Isolation.Options) {
			options = append(options, form.Option{Label: value, Value: value})
		}
		return remembering(remembered{Options: options, Value: string(q.Isolation.Value), Remembered: string(q.Isolation.Remembered), Ticked: w.ticked(s)})
	case stepFastForward:
		options := []form.Option{{Label: "Fast-forward to origin", Value: yes}, {Separator: true}, {Label: "Keep it as-is", Value: no}}
		ticked := w.ticked(s) || q.FastForward.Origin == domain.OriginRemembered
		return form.NewSelectList(form.SelectParams{Options: options, Start: yesNo(q.FastForward.Value), Toggle: true, ToggleOn: ticked})
	case stepMode:
		options := []form.Option{{Label: "Move — remove the files from " + req.Source, Value: modeMove}, {Label: "Copy — keep the files in " + req.Source, Value: modeCopy}}
		return form.NewSelectList(form.SelectParams{Options: options, Start: mode(q.Keep.Value)})
	case stepOnConflict:
		options := []form.Option{
			{Label: "Abort — leave both worktrees untouched", Value: string(extract.OnConflictAbort)},
			{Label: "Resolve — write conflict markers in " + req.To, Value: string(extract.OnConflictResolve)},
		}
		return form.NewSelectList(form.SelectParams{Options: options, Start: string(q.OnConflict.Value)})
	case stepFiles, stepBranch, stepRecap, stepCount:
	}

	return form.NewSelectList(form.SelectParams{})
}

func optionsOf(found []domain.Option) []form.Option {
	options := make([]form.Option, 0, len(found))
	for _, o := range found {
		options = append(options, form.Option{Label: o.Value, Value: o.Value, Badge: o.Badge})
	}

	return options
}

// fromList offers the source branches, the pinned one first, starting on the answer so far.
func (w *wizard) fromList() *form.SelectList {
	from := w.q.From
	options := optionsOf(from.Options)
	if at := slices.IndexFunc(options, func(o form.Option) bool { return o.Value == from.Pinned }); at > 0 {
		pinned := options[at]
		options = append([]form.Option{pinned}, slices.Delete(options, at, at+1)...)
	}

	return form.NewSelectList(form.SelectParams{Options: options, Start: cmp.Or(from.Value, from.Pinned)})
}

// remembered are the options of a choice, its answer so far, its remembered answer and whether its toggle is ticked.
type remembered struct {
	Options    []form.Option
	Value      string
	Remembered string
	Ticked     bool
}

// remembering offers a choice with its "always use this answer" toggle; a remembered answer starts the list on it, the toggle ticked.
func remembering(params remembered) *form.SelectList {
	if params.Remembered != "" {
		return form.NewSelectList(form.SelectParams{Options: params.Options, Start: params.Remembered, Toggle: true, ToggleOn: true})
	}

	return form.NewSelectList(form.SelectParams{Options: params.Options, Start: params.Value, Toggle: true, ToggleOn: params.Ticked})
}

func (w *wizard) ticked(s step) bool {
	return slices.Contains(w.req.Remember, stepPaths[s])
}

// files offers the source's changes, each tagged with its status, those answered so far picked.
func (w *wizard) files() *form.MultiSelect {
	changes := w.q.Files.Options
	picks := make([]form.Pick, 0, len(changes))
	for _, change := range changes {
		picks = append(picks, form.Pick{Label: change.Label(), Value: change.Path, Tag: change.Status.Tag(), Tone: tone(change.Status), Picked: slices.Contains(w.req.Files, change.Path)})
	}

	return form.NewMultiSelect(form.MultiSelectParams{Options: picks, Empty: emptyFiles})
}

// tone is the colour of a change's tag: an addition, a loss, a risk, or a rename that changes no content.
func tone(status worktree.ChangeStatus) form.Tone {
	switch status {
	case worktree.ChangeUntracked:
		return form.ToneSuccess
	case worktree.ChangeDeleted:
		return form.ToneDanger
	case worktree.ChangeModified:
		return form.ToneWarning
	case worktree.ChangeRenamed:
	}

	return form.ToneMuted
}

// branchProblem checks the branch typed for a created target against the repository, as the wizard shows it.
func (w *wizard) branchProblem(branch string) error {
	if slices.ContainsFunc(w.q.To.Options, func(o domain.Option) bool { return o.Value == branch }) {
		return errors.New(branch + " already has a worktree — pick it as the target worktree")
	}

	req := w.req
	req.To = branch
	questions, err := w.app.Questions(w.ctx, req)
	if err != nil {
		return err
	}

	for _, found := range questions.Errors {
		if found.Path == stepPaths[stepBranch] {
			return errors.New(found.Message(stepLabels[stepBranch]))
		}
	}

	return nil
}

// label spells a field path with the label of the first step that answers it.
func label(path string) string {
	at := slices.Index(stepPaths[:], path)
	if at < 0 {
		return path
	}

	return stepLabels[at]
}

func yesNo(value bool) string {
	if value {
		return yes
	}

	return no
}
