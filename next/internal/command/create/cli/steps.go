package cli

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
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

// labels are the steps' names in the breadcrumb, the trail and the problems the wizard shows.
var labels = [stepCount]string{"Branches", "Source branch", "Env strategy", "Isolation", "Source update", "If not exists", "Confirm"}

// paths are the field path each step answers.
var paths = [stepCount]string{"branches", "from", "env_from", "isolation", "fast_forward", "if_not_exists", ""}

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
		st, flagged = form.Standing{Value: string(q.Isolation.Value), Origin: q.Isolation.Origin, Skip: q.Isolation.Skip}, w.given.Isolation != ""
	case stepFastForward:
		st, flagged = form.Standing{Value: yesNo(q.FastForward.Value), Origin: q.FastForward.Origin, Skip: q.FastForward.Skip}, w.given.FastForward != nil
	case stepIfNotExists:
		// The question skips a step only its flag sets; given, the flag stands in the trail instead.
		st, flagged = form.Standing{Value: yesNo(q.IfNotExists.Value)}, w.given.IfNotExists
		if !flagged {
			st.Skip = q.IfNotExists.Skip
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
	return slices.ContainsFunc(w.q.Errors, func(problem domain.FieldError) bool {
		return problem.Path == path || strings.HasPrefix(problem.Path, path+"[")
	})
}

// sourceList offers the source branches, the pinned one first, starting on the answer so far.
func (w *wizard) sourceList() *form.SelectList {
	from := w.q.From
	options := make([]form.Option, 0, len(from.Options))
	for _, o := range from.Options {
		options = append(options, form.Option{Label: o.Value, Value: o.Value, Badge: o.Badge})
	}

	if at := slices.IndexFunc(options, func(o form.Option) bool { return o.Value == from.Pinned }); at > 0 {
		pinned := options[at]
		options = append([]form.Option{pinned}, slices.Delete(options, at, at+1)...)
	}

	return form.NewSelectList(form.SelectParams{Options: options, Start: cmp.Or(from.Value, from.Pinned)})
}

// choiceParams are the options of a choice, its answer so far, its remembered answer and whether its toggle is ticked.
type choiceParams struct {
	Options    []string
	Value      string
	Remembered string
	Ticked     bool
}

// choiceList offers a choice with its "always use this answer" toggle; a remembered answer starts the list on it, the toggle ticked.
func choiceList(params choiceParams) *form.SelectList {
	options := make([]form.Option, 0, len(params.Options))
	for _, value := range params.Options {
		options = append(options, form.Option{Label: value, Value: value})
	}

	if params.Remembered != "" {
		return form.NewSelectList(form.SelectParams{Options: options, Start: params.Remembered, Toggle: true, ToggleOn: true})
	}

	return form.NewSelectList(form.SelectParams{Options: options, Start: params.Value, Toggle: true, ToggleOn: params.Ticked})
}

// choices are the options and answers of the choice step s: the isolation, else the env strategy.
func (w *wizard) choices(s step) choiceParams {
	ticked := slices.Contains(w.req.Remember, paths[s])

	if s == stepIsolation {
		q := w.q.Isolation
		return choiceParams{Options: create.Names(q.Options), Value: string(q.Value), Remembered: string(q.Remembered), Ticked: ticked}
	}

	q := w.q.EnvFrom
	return choiceParams{Options: create.Names(q.Options), Value: string(q.Value), Remembered: string(q.Remembered), Ticked: ticked}
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
