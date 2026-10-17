package form

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/LucasPcq/wtm/next/internal/wtm"
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

type mark struct {
	asked bool
	ruled string
	line  string
}

type standing struct {
	value   string
	flagged bool
	origin  create.Origin
	skip    string
}

func (w *wizard) standing(s step) standing {
	q := w.q
	switch s {
	case stepBranches:
		return standing{value: strings.Join(q.Branches.Given, ", "), flagged: len(w.given.Branches) > 0}
	case stepFrom:
		return standing{value: q.From.Value, flagged: w.given.From != "", origin: q.From.Origin}
	case stepEnvFrom:
		return standing{value: string(q.EnvFrom.Value), flagged: w.given.EnvFrom != "", origin: q.EnvFrom.Origin}
	case stepIsolation:
		return standing{value: string(q.Isolation.Value), flagged: w.given.Isolation != "", origin: q.Isolation.Origin, skip: q.Isolation.Skip}
	case stepFastForward:
		return standing{value: yesNo(q.FastForward.Value), flagged: w.given.FastForward != nil, origin: q.FastForward.Origin, skip: q.FastForward.Skip}
	case stepIfNotExists:
		return standing{value: yesNo(q.IfNotExists.Value), skip: q.IfNotExists.Skip}
	case stepRecap, stepCount:
	}
	return standing{}
}

func (w *wizard) decide(s step) mark {
	st := w.standing(s)
	value := cmp.Or(st.value, none)
	switch {
	case s == stepRecap:
		return mark{asked: true}
	case st.skip != "":
		return mark{ruled: st.skip}
	case w.erred(paths[s]):
		return mark{asked: true}
	case st.flagged:
		return mark{line: value + trail + output.Flag(paths[s])}
	case w.answered[s]:
		return mark{line: value}
	case st.origin == create.OriginRemembered:
		return mark{line: value + trail + string(create.OriginRemembered)}
	}
	return mark{asked: true}
}

func (w *wizard) asks() bool {
	return slices.ContainsFunc([]step{stepBranches, stepFrom, stepEnvFrom, stepIsolation, stepFastForward, stepIfNotExists}, func(s step) bool { return w.decide(s).asked })
}

func (w *wizard) summarized(s step, shown string) string {
	offered := w.standing(s)
	kept := offered.origin == create.OriginConfig || offered.origin == create.OriginDefault
	if kept && shown == offered.value {
		return shown + trail + string(offered.origin)
	}
	return shown
}

func (w *wizard) erred(path string) bool {
	return slices.ContainsFunc(w.q.Errors, func(problem wtm.FieldError) bool {
		return problem.Path == path || strings.HasPrefix(problem.Path, path+"[")
	})
}

func (w *wizard) sourceList() *selectList {
	from := w.q.From
	options := make([]option, 0, len(from.Options))
	for _, o := range from.Options {
		options = append(options, option{Label: o.Value, Value: o.Value, Badge: o.Badge})
	}
	if at := slices.IndexFunc(options, func(o option) bool { return o.Value == from.Pinned }); at > 0 {
		pinned := options[at]
		options = append([]option{pinned}, slices.Delete(options, at, at+1)...)
	}
	return newSelectList(selectParams{Options: options, Start: cmp.Or(from.Value, from.Pinned)})
}

type choiceParams struct {
	Options    []string
	Value      string
	Remembered string
	Ticked     bool
}

func choiceList(params choiceParams) *selectList {
	options := make([]option, 0, len(params.Options))
	for _, value := range params.Options {
		options = append(options, option{Label: value, Value: value})
	}
	if params.Remembered != "" {
		return newSelectList(selectParams{Options: options, Start: params.Remembered, Toggle: true, ToggleOn: true})
	}
	return newSelectList(selectParams{Options: options, Start: params.Value, Toggle: true, ToggleOn: params.Ticked})
}

func (w *wizard) choices(s step) choiceParams {
	ticked := slices.Contains(w.req.Remember, paths[s])
	if s == stepIsolation {
		q := w.q.Isolation
		return choiceParams{Options: names(q.Options), Value: string(q.Value), Remembered: string(q.Remembered), Ticked: ticked}
	}
	q := w.q.EnvFrom
	return choiceParams{Options: names(q.Options), Value: string(q.Value), Remembered: string(q.Remembered), Ticked: ticked}
}

func (w *wizard) branchProblem(entries []string, from int) error {
	req := w.req
	req.Branches = entries
	questions, err := w.app.CreateQuestions(w.ctx, req)
	if err != nil {
		return err
	}
	for _, found := range questions.Errors {
		if found.Path == paths[stepBranches] {
			return problem(found)
		}
		index, entry := strings.CutPrefix(found.Path, paths[stepBranches]+"[")
		at, err := strconv.Atoi(strings.TrimSuffix(index, "]"))
		if entry && err == nil && at >= from {
			return problem(found)
		}
	}
	return nil
}

func problem(found wtm.FieldError) error {
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
	at := slices.IndexFunc(w.q.Branches.Options, func(o create.Option) bool { return o.Value == entry })
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

func names[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}
