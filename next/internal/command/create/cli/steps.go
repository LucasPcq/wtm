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

var labels = [stepCount]string{"Branches", "Source branch", "Env strategy", "Isolation", "Source update", "If not exists", "Confirm"}

var paths = [stepCount]string{"branches", "from", "env_from", "isolation", "fast_forward", "if_not_exists", ""}

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
		st = form.Standing{Value: yesNo(q.IfNotExists.Value), Skip: q.IfNotExists.Skip}
	case stepRecap, stepCount:
	}
	st.Erred = w.erred(paths[s])
	if flagged {
		st.Flag = createFlags.Spell(paths[s])
	}
	return st
}

func (w *wizard) erred(path string) bool {
	return slices.ContainsFunc(w.q.Errors, func(problem domain.FieldError) bool {
		return problem.Path == path || strings.HasPrefix(problem.Path, path+"[")
	})
}

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

type choiceParams struct {
	Options    []string
	Value      string
	Remembered string
	Ticked     bool
}

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

func (w *wizard) choices(s step) choiceParams {
	ticked := slices.Contains(w.req.Remember, paths[s])
	if s == stepIsolation {
		q := w.q.Isolation
		return choiceParams{Options: create.Names(q.Options), Value: string(q.Value), Remembered: string(q.Remembered), Ticked: ticked}
	}
	q := w.q.EnvFrom
	return choiceParams{Options: create.Names(q.Options), Value: string(q.Value), Remembered: string(q.Remembered), Ticked: ticked}
}

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
