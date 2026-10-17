package cli

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// A step is the index of a question of the wizard, in the order asked; labels and paths are indexed by it.
type step int

const (
	stepBranches step = iota
	stepFrom
	stepEnvFrom
	stepIsolation
	stepFastForward
	stepIfNotExists
	stepRecap
	stepCount
)

// labels are the steps' names in the breadcrumb, the trail and the problems the wizard shows, and paths the field path each step answers, both read from create's form.
var labels, paths = steps(schema)

// steps are the label and path of each field of form, a step per field in its order, then the recap.
func steps(form domain.Form) (labels, paths [stepCount]string) {
	for at, field := range form.Fields {
		labels[at], paths[at] = field.Spec().Label, field.Spec().Path
	}
	labels[stepRecap] = "Confirm"

	return labels, paths
}

// standing is where the answer of step s stands on the questions observed last, with the flag that set it, if one did.
func (w *wizard) standing(s step) form.Standing {
	q := w.q
	st, flagged := form.Standing{}, false
	switch s {
	case stepBranches:
		st, flagged = form.Standing{Value: strings.Join(q.Branches.Given, ", ")}, len(w.given.Branches) > 0
	case stepFrom:
		st, flagged = form.Standing{Value: q.From.Value, Origin: q.From.Origin}, w.given.From != ""
	case stepEnvFrom:
		st, flagged = form.Standing{Value: string(q.EnvFrom.Value), Origin: q.EnvFrom.Origin}, w.given.EnvFrom != ""
	case stepIsolation:
		st, flagged = form.Standing{Value: string(q.Isolation.Value), Origin: q.Isolation.Origin, Skip: w.field(s).Spec().Skip}, w.given.Isolation != ""
	case stepFastForward:
		st, flagged = form.Standing{Value: yesNo(q.FastForward.Value), Origin: q.FastForward.Origin, Skip: w.field(s).Spec().Skip}, w.given.FastForward != nil
	case stepIfNotExists:
		// The question skips a step only its flag sets; given, the flag stands in the trail instead.
		st, flagged = form.Standing{Value: yesNo(q.IfNotExists.Value)}, w.given.IfNotExists
		if !flagged {
			st.Skip = w.field(s).Spec().Skip
		}
	case stepRecap, stepCount:
	}

	st.Erred = w.erred(paths[s])
	if flagged {
		st.Flag = createFlags.Spell(paths[s])
	}

	return st
}

// erred reports whether a problem names the field at path or one of its entries.
func (w *wizard) erred(path string) bool {
	return len(domain.ProblemsOf(w.q.Errors, path)) > 0
}

// field is the field step s answers, on the questions observed last; nil for the recap.
func (w *wizard) field(s step) domain.Field {
	fields := w.q.Form().Fields
	if int(s) >= len(fields) {
		return nil
	}

	return fields[s]
}

// selectField is the select field step s answers, empty when it is not one.
func (w *wizard) selectField(s step) domain.SelectField {
	field, isSelect := w.field(s).(domain.SelectField)
	if !isSelect {
		return domain.SelectField{}
	}

	return field
}

// sourceList offers the source branches, the pinned one first, starting on the answer so far.
func (w *wizard) sourceList() *form.SelectList {
	from := w.selectField(stepFrom)
	options := make([]form.Option, 0, len(from.Options))
	for _, o := range from.Options {
		options = append(options, form.Option{Label: cmp.Or(o.Label, o.Value), Value: o.Value, Badge: o.Badge})
	}

	if at := slices.IndexFunc(options, func(o form.Option) bool { return o.Value == from.Pinned }); at > 0 {
		pinned := options[at]
		options = append([]form.Option{pinned}, slices.Delete(options, at, at+1)...)
	}

	return form.NewSelectList(form.SelectParams{Options: options, Start: cmp.Or(from.Value, from.Pinned)})
}

// choiceList offers the choice step s with its "always use this answer" toggle; a remembered answer starts the list on it, the toggle ticked.
func (w *wizard) choiceList(s step) *form.SelectList {
	choice := w.selectField(s)
	options := make([]form.Option, 0, len(choice.Options))
	for _, o := range choice.Options {
		options = append(options, form.Option{Label: cmp.Or(o.Label, o.Value), Value: o.Value})
	}

	if choice.Remembered != "" {
		return form.NewSelectList(form.SelectParams{Options: options, Start: choice.Remembered, Toggle: true, ToggleOn: true})
	}

	return form.NewSelectList(form.SelectParams{Options: options, Start: choice.Value, Toggle: true, ToggleOn: slices.Contains(w.req.Remember, paths[s])})
}

// branchProblem checks the branches typed against the repository: the first problem of the list itself or of an entry from check.From on, as the wizard shows it.
func (w *wizard) branchProblem(check form.EntryCheck) error {
	req := w.req
	req.Branches = check.Entries
	questions, err := w.app.Questions(w.ctx, req)
	if err != nil {
		return err
	}

	for _, found := range questions.Errors {
		if found.Path == paths[stepBranches] {
			return problem(found)
		}

		index, entry := strings.CutPrefix(found.Path, paths[stepBranches]+"[")
		at, err := strconv.Atoi(strings.TrimSuffix(index, "]"))
		if entry && err == nil && at >= check.From {
			return problem(found)
		}
	}

	return nil
}

func problem(found domain.FieldError) error {
	return errors.New(found.Message(label(found.Path)))
}

// label spells a field path with its step's label, keeping an entry's index: branches[1] reads Branches[1].
func label(path string) string {
	name, index, indexed := strings.Cut(path, "[")
	at := slices.Index(paths[:], name)
	if at < 0 {
		return path
	}

	if indexed {
		return labels[at] + "[" + index
	}

	return labels[at]
}

// branchBadge is the badge of a branch typed that the repository holds, "" for a new one.
func (w *wizard) branchBadge(entry string) string {
	at := slices.IndexFunc(w.q.Branches.Options, func(o domain.Option) bool { return o.Value == entry })
	if at < 0 {
		return ""
	}

	return w.q.Branches.Options[at].Badge
}

func yesNo(yes bool) string {
	if yes {
		return "yes"
	}

	return "no"
}
