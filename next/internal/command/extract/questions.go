package extract

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

type OnConflict string

const (
	OnConflictAbort   OnConflict = "abort"
	OnConflictResolve OnConflict = "resolve"
)

func OnConflicts() []OnConflict {
	return []OnConflict{OnConflictAbort, OnConflictResolve}
}

type Request struct {
	Source      string            `json:"source"`
	Files       []string          `json:"files"`
	To          string            `json:"to"`
	From        string            `json:"from"`
	FastForward *bool             `json:"fast_forward"`
	Isolation   project.Isolation `json:"isolation"`
	Keep        *bool             `json:"keep"`
	OnConflict  OnConflict        `json:"on_conflict"`
	Remember    []string          `json:"remember"`
	Ask         bool              `json:"-"`
}

type Questions struct {
	Source      Source
	Files       Files
	To          To
	From        From
	Isolation   domain.Choice[project.Isolation]
	FastForward domain.Confirm
	Keep        domain.Confirm
	OnConflict  Conflicting
	Remember    []string
	Errors      []domain.FieldError
}

type Source struct {
	Given   string
	Options []domain.Option
}

type Files struct {
	Given   []string
	Options []worktree.Change
}

type To struct {
	Given   string
	Options []domain.Option
}

type From struct {
	domain.Answer[string]
	Options []domain.Option
	Pinned  string
	Skip    string
}

type Conflicting struct {
	domain.Choice[OnConflict]
	Conflicts []worktree.Conflict
}

const existingSkip = "the target worktree exists: nothing is created"

func Form(f Facts, req Request) Questions {
	recalled := f.Config.Remembered
	if req.Ask {
		f.Config.Remembered = nil
	}
	from := f.from(req)
	req.From = from.Value
	questions := Questions{
		Source:      Source{Given: req.Source, Options: f.sourceOptions()},
		Files:       Files{Given: req.Files, Options: f.Changes[req.Source]},
		To:          To{Given: req.To, Options: f.targetOptions(req.Source)},
		From:        from,
		Isolation:   f.isolation(req),
		FastForward: f.fastForward(req),
		Keep:        keep(req.Keep),
		OnConflict:  onConflict(req.OnConflict),
		Remember:    req.Remember,
	}
	questions.Isolation.Remembered = project.Isolation(recalled[project.RememberIsolation])
	questions.Errors = f.problems(req, questions)
	return questions
}

func (f Facts) sourceOptions() []domain.Option {
	var options []domain.Option
	for _, w := range f.sources("") {
		if count := len(f.Changes[w.Branch()]); count > 0 {
			options = append(options, domain.Option{Value: w.Branch(), Badge: changes(count)})
		}
	}
	return options
}

func changes(count int) string {
	if count == 1 {
		return "1 change"
	}
	return strconv.Itoa(count) + " changes"
}

func (f Facts) targetOptions(source string) []domain.Option {
	var options []domain.Option
	for _, w := range f.sources("") {
		if w.Branch() != source {
			options = append(options, domain.Option{Value: w.Branch()})
		}
	}
	return options
}

func (f Facts) from(req Request) From {
	recorded := f.parent(req.Source)
	base := f.Config.Worktrees.BaseBranch
	question := From{Options: f.fromOptions(req.To), Pinned: cmp.Or(recorded, base)}
	target, decided := f.planned(req)
	if decided && !target.creates() {
		question.Skip = existingSkip
	}
	switch {
	case req.From != "":
		question.Answer = domain.Answer[string]{Value: req.From, Origin: domain.OriginRequest}
	case decided && (!target.creates() || target.Action == ActionCheckOut):
	case recorded != "":
		question.Answer = domain.Answer[string]{Value: recorded, Origin: domain.OriginDefault}
	case base != "":
		question.Answer = domain.Answer[string]{Value: base, Origin: domain.OriginConfig}
	}
	return question
}

func (f Facts) fromOptions(to string) []domain.Option {
	options := make([]domain.Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, local := range f.Branches.Local {
		if local.Name != to {
			options = append(options, domain.Option{Value: local.Name, Badge: f.badge(local.Name)})
		}
	}
	for _, remote := range f.Branches.Remote {
		if !f.Local(remote.Name) && remote.Name != to {
			options = append(options, domain.Option{Value: worktree.OriginPrefix + remote.Name})
		}
	}
	return options
}

func (f Facts) badge(name string) string {
	counts := f.AheadBehind(name)
	switch f.Standing(name) {
	case worktree.Behind:
		return strconv.Itoa(counts.Behind) + " behind"
	case worktree.Ahead:
		return strconv.Itoa(counts.Ahead) + " ahead"
	case worktree.Diverged:
		return aheadAndBehind(counts)
	case worktree.NoOrigin, worktree.UpToDate:
	}
	return ""
}

func (f Facts) isolation(req Request) domain.Choice[project.Isolation] {
	question := domain.Choice[project.Isolation]{Options: project.Isolations()}
	existing := f.existing(req)
	switch {
	case existing:
		question.Skip = existingSkip
	case !f.Run.IsolationApplies():
		question.Skip = "run.toml declares nothing a worktree isolates"
	}
	remembered, found := f.remembered(project.RememberIsolation)
	switch {
	case req.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: req.Isolation, Origin: domain.OriginRequest}
	case existing:
	case found && question.Skip == "":
		question.Answer = domain.Answer[project.Isolation]{Value: project.Isolation(remembered), Origin: domain.OriginRemembered}
	case f.Run.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: f.Run.Isolation.Effective(), Origin: domain.OriginConfig}
	default:
		question.Answer = domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}
	}
	return question
}

func (f Facts) fastForward(req Request) domain.Confirm {
	question := domain.Confirm{}
	switch {
	case f.existing(req):
		question.Skip = existingSkip
	case f.subjectAt(req, worktree.Behind) == "":
		question.Skip = "nothing to fast-forward: the branch it starts from is not behind origin"
	}
	remembered, found := f.remembered(project.RememberSourceUpdate)
	switch {
	case req.FastForward != nil:
		question.Answer = domain.Answer[bool]{Value: *req.FastForward, Origin: domain.OriginRequest}
	case question.Skip != "":
	case found:
		question.Answer = domain.Answer[bool]{Value: remembered == project.SourceUpdateFastForward, Origin: domain.OriginRemembered}
	default:
		question.Answer = domain.Answer[bool]{Origin: domain.OriginDefault}
	}
	return question
}

func keep(given *bool) domain.Confirm {
	if given == nil {
		return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}
	}
	return domain.Confirm{Answer: domain.Answer[bool]{Value: *given, Origin: domain.OriginRequest}}
}

func onConflict(given OnConflict) Conflicting {
	question := Conflicting{Choice: domain.Choice[OnConflict]{Options: OnConflicts(), Skip: "nothing conflicts with the target"}}
	question.Answer = domain.Answer[OnConflict]{Value: OnConflictAbort, Origin: domain.OriginDefault}
	if given != "" {
		question.Answer = domain.Answer[OnConflict]{Value: given, Origin: domain.OriginRequest}
	}
	return question
}

func (q Questions) facing(conflicts []worktree.Conflict) Questions {
	q.OnConflict.Conflicts = conflicts
	if len(conflicts) > 0 {
		q.OnConflict.Skip = ""
	}
	return q
}

func (f Facts) remembered(id string) (string, bool) {
	value := f.Config.Remembered[id]
	return value, value != ""
}

const (
	codeRequired = "required"
	codeNotFound = "not_found"
	codeOneOf    = "one_of"
)

var remembered = []struct{ field, id string }{
	{"isolation", project.RememberIsolation},
	{"fast_forward", project.RememberSourceUpdate},
}

func rememberable() []string {
	fields := make([]string, 0, len(remembered))
	for _, r := range remembered {
		fields = append(fields, r.field)
	}
	return fields
}

func Resolve(f Facts, req Request) (Request, error) {
	questions := Form(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}
	return questions.Request(), nil
}

func (q Questions) Request() Request {
	return Request{
		Source:      q.Source.Given,
		Files:       q.Files.Given,
		To:          q.To.Given,
		From:        q.From.Value,
		FastForward: new(q.FastForward.Value),
		Isolation:   q.Isolation.Value,
		Keep:        new(q.Keep.Value),
		OnConflict:  q.OnConflict.Value,
		Remember:    slices.DeleteFunc(slices.Clone(q.Remember), q.skipped),
	}
}

func (q Questions) skipped(field string) bool {
	return field == "isolation" && q.Isolation.Skip != "" || field == "fast_forward" && q.FastForward.Skip != ""
}

func (f Facts) problems(req Request, q Questions) []domain.FieldError {
	problems := f.sourceProblems(req.Source)
	if f.nothing(req.Source) {
		return problems
	}
	problems = append(problems, f.fileProblems(req)...)
	if refused, isRefused := f.target(req).(refusal); isRefused {
		problems = append(problems, refused.field("to"))
	}
	problems = append(problems, f.fromProblems(req)...)
	if q.Isolation.Origin == domain.OriginRequest {
		problems = append(problems, oneOf(choice{Path: "isolation", Value: string(q.Isolation.Value), Accepted: Names(project.Isolations())})...)
	}
	problems = append(problems, oneOf(choice{Path: "on_conflict", Value: string(q.OnConflict.Value), Accepted: Names(OnConflicts())})...)
	for index, id := range q.Remember {
		problems = append(problems, oneOf(choice{Path: "remember[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable()})...)
	}
	return problems
}

func (f Facts) sourceProblems(source string) []domain.FieldError {
	switch {
	case source == "":
		return []domain.FieldError{{Path: "source", Code: codeRequired, Template: "{path} is required"}}
	case f.CheckedOutAt(source) == "":
		return []domain.FieldError{{Path: "source", Code: codeNotFound, Template: "{path}: no worktree holds {value}", Params: domain.Params{"value": source}}}
	}
	return nil
}

func (f Facts) fileProblems(req Request) []domain.FieldError {
	switch {
	case f.CheckedOutAt(req.Source) == "":
		return nil
	case len(req.Files) == 0:
		return []domain.FieldError{{Path: "files", Code: codeRequired, Template: "{path} is required"}}
	}
	if _, missed := worktree.SelectChanges(f.Changes[req.Source], req.Files); missed != "" {
		return []domain.FieldError{{Path: "files", Code: "extract.not_a_change", Template: "{path}: {value} is not an uncommitted change", Params: domain.Params{"value": missed}}}
	}
	return nil
}

func (f Facts) fromProblems(req Request) []domain.FieldError {
	target, decided := f.planned(req)
	from := req.From
	switch {
	case !decided || !target.creates():
		return nil
	case from == "" && target.Action == ActionCheckOut:
		return []domain.FieldError{{
			Path:     "from",
			Code:     codeRequired,
			Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
			Params:   domain.Params{"because": "extract.parent_required"},
		}}
	case from == "":
		return []domain.FieldError{{Path: "from", Code: codeRequired, Template: "{path} is required: no parent is recorded for the source and no base branch is configured"}}
	case from == target.Branch:
		return []domain.FieldError{{Path: "from", Code: "self_parent", Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": from}, With: []string{"to"}}}
	case !f.Candidate(from):
		return []domain.FieldError{{Path: "from", Code: codeNotFound, Template: "{path}: {value} not found", Params: domain.Params{"value": from}}}
	}
	return nil
}

type choice struct {
	Path     string
	Value    string
	Accepted []string
}

func oneOf(c choice) []domain.FieldError {
	if c.Value == "" || slices.Contains(c.Accepted, c.Value) {
		return nil
	}
	return []domain.FieldError{{
		Path:     c.Path,
		Code:     codeOneOf,
		Template: "{path} must be one of {accepted}, not {value}",
		Params:   domain.Params{"value": c.Value},
		Accepted: c.Accepted,
	}}
}

func Names[T ~string](values []T) []string {
	found := make([]string, 0, len(values))
	for _, value := range values {
		found = append(found, string(value))
	}
	return found
}
