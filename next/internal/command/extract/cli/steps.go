package cli

import (
	"cmp"
	"errors"
	"slices"
	"strings"

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

// stepLabels are the steps' names in the breadcrumb, the trail and the problems the wizard shows, and stepPaths the field path each step answers (the branch of a created target is the target's), both read from extract's form.
var stepLabels, stepPaths = steps(schema)

// steps are the label and path of each field of form, a step per field in its order, then the recap.
func steps(form domain.Form) (labels, paths [stepCount]string) {
	for at, field := range form.Fields {
		labels[at], paths[at] = field.Spec().Label, field.Spec().Path
	}
	labels[stepRecap] = "Confirm"

	return labels, paths
}

const (
	createTarget = "\x00new-worktree"
	newWorktree  = "new worktree"
	existingSkip = "the target worktree exists: nothing is created"
	yes          = "yes"
	no           = "no"
	modeCopy     = "copy"
	emptyFiles   = "select at least one file"
)

// field is the field step s answers, on the questions observed last; nil for the recap.
func (w *wizard) field(s step) domain.Field {
	fields := w.q.Form().Fields
	if int(s) >= len(fields) {
		return nil
	}

	return fields[s]
}

// description is the help of step s on the answers so far.
func (w *wizard) description(s step) string {
	if field := w.field(s); field != nil {
		return field.Spec().Help
	}

	return ""
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
	switch field := w.field(s).(type) {
	case domain.SelectField:
		return w.selectList(s, field)
	case domain.ConfirmField:
		return w.confirmList(s, field)
	case domain.MultiSelectField, domain.TextField, domain.TextListField, nil:
	}

	return form.NewSelectList(form.SelectParams{})
}

// selectList offers the options of field; the target's first option, a worktree to create, is picked by the sentinel createTarget.
func (w *wizard) selectList(s step, field domain.SelectField) *form.SelectList {
	switch s {
	case stepTarget:
		created, worktrees := field.Options[0], field.Options[1:]
		options := append([]form.Option{{Label: created.Label, Value: createTarget}, {Separator: true}}, optionsOf(worktrees)...)
		start := w.req.To
		if w.creating() {
			start = createTarget
		}
		return form.NewSelectList(form.SelectParams{Options: options, Start: start})
	case stepFrom:
		return fromList(field)
	case stepIsolation:
		return remembering(remembered{Options: optionsOf(field.Options), Value: field.Value, Remembered: field.Remembered, Ticked: w.ticked(s)})
	}

	return form.NewSelectList(form.SelectParams{Options: optionsOf(field.Options), Start: field.Value})
}

// confirmList offers the two answers of field: the source update with its toggle, the mode move first.
func (w *wizard) confirmList(s step, field domain.ConfirmField) *form.SelectList {
	start := field.No.Value
	if field.Value {
		start = field.Yes.Value
	}

	if s == stepFastForward {
		options := []form.Option{optionOf(field.Yes), {Separator: true}, optionOf(field.No)}
		ticked := w.ticked(s) || field.Origin == domain.OriginRemembered
		return form.NewSelectList(form.SelectParams{Options: options, Start: start, Toggle: true, ToggleOn: ticked})
	}

	return form.NewSelectList(form.SelectParams{Options: []form.Option{optionOf(field.No), optionOf(field.Yes)}, Start: start})
}

func optionsOf(found []domain.Option) []form.Option {
	options := make([]form.Option, 0, len(found))
	for _, o := range found {
		options = append(options, optionOf(o))
	}

	return options
}

func optionOf(o domain.Option) form.Option {
	return form.Option{Label: cmp.Or(o.Label, o.Value), Value: o.Value, Badge: o.Badge}
}

// fromList offers the source branches, the pinned one first, starting on the answer so far.
func fromList(field domain.SelectField) *form.SelectList {
	options := optionsOf(field.Options)
	if at := slices.IndexFunc(options, func(o form.Option) bool { return o.Value == field.Pinned }); at > 0 {
		pinned := options[at]
		options = append([]form.Option{pinned}, slices.Delete(options, at, at+1)...)
	}

	return form.NewSelectList(form.SelectParams{Options: options, Start: cmp.Or(field.Value, field.Pinned)})
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
	if held := w.q.NewTarget(branch); len(held) > 0 {
		return errors.New(held[0].Message(stepLabels[stepBranch]))
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
