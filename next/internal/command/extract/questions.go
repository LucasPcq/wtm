package extract

import (
	"cmp"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The request

// OnConflict is what to do when the changes conflict with the target.
type OnConflict string

// The answers to a conflict.
const (
	OnConflictAbort   OnConflict = "abort"   // refuse, both worktrees left untouched
	OnConflictResolve OnConflict = "resolve" // apply, writing conflict markers in the target
)

// OnConflicts are the answers to a conflict, in the order the wizard offers them.
func OnConflicts() []OnConflict {
	return []OnConflict{OnConflictAbort, OnConflictResolve}
}

// A Request is what the caller asks of extract, as given: a field left empty takes its remembered answer, its recorded or configured value or its default when the request is resolved.
type Request struct {
	Source      string            `json:"source"`
	Files       []string          `json:"files"` // paths of changes, or directories to take every change below
	To          string            `json:"to"`
	From        string            `json:"from"`
	FastForward *bool             `json:"fast_forward"` // nil when not given, so a remembered answer can apply
	Isolation   project.Isolation `json:"isolation"`
	Keep        *bool             `json:"keep"` // copy rather than move; nil when not given
	OnConflict  OnConflict        `json:"on_conflict"`
	Remember    []string          `json:"remember"` // the fields whose answer is remembered for the next runs
	Ask         bool              `json:"-"`        // ignore the remembered answers for this run
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: each question with its options, its answer so far and where that answer comes from, the fields whose answer is to be remembered, and the request's problems as field errors.
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

// Source is the question of the worktree to extract from: the branch given, and the worktrees that hold changes, each badged with how many.
type Source struct {
	Given   string
	Options []domain.Option
}

// Files is the question of the changes to extract: the paths given, and the source's uncommitted changes.
type Files struct {
	Given   []string
	Options []worktree.Change
}

// To is the question of the target: the branch given, and the worktrees other than the source.
type To struct {
	Given   string
	Options []domain.Option
}

// From is the question of the branch a created target starts from: its answer, the branches it can start from, the one pinned at the top of the list (the source's recorded parent, else the base branch), and why it is skipped.
type From struct {
	domain.Answer[string]
	Options []domain.Option
	Pinned  string
	Skip    string
}

// Conflicting is the question of what to do on conflict, with the conflicts the target would meet; it is skipped when there are none.
type Conflicting struct {
	domain.Choice[OnConflict]
	Conflicts []worktree.Conflict
}

const existingSkip = "the target worktree exists: nothing is created"

// Form builds the questions for the request on the facts: a field given keeps its value, an empty one takes its remembered answer (unless the request asks again), its recorded or configured value or its default. Its conflict question has no conflicts yet: they are read once the request has no problem (UseCase.Questions).
func Form(f Facts, req Request) Questions {
	recalled := f.Config.Remembered
	if req.Ask {
		f.Config.Remembered = nil
	}

	// The source branch is settled first: whether a fast-forward is asked depends on the branch the target starts from.
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
	// Even under --ask, the question carries what is remembered, so a form can show it to keep or untick.
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

// from answers the source branch with the source's recorded parent while it still exists, else the base branch; it leaves it unanswered for an existing target, and for a branch checked out as it exists, whose parent must be named.
func (f Facts) from(req Request) From {
	recorded := f.parent(req.Source)
	if !f.Candidate(recorded) {
		recorded = ""
	}
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

// fromOptions are the branches a created target can start from, the target excepted; a branch only on origin is offered as origin/<branch>.
func (f Facts) fromOptions(to string) []domain.Option {
	options := make([]domain.Option, 0, len(f.Branches.Local)+len(f.Branches.Remote))
	for _, local := range f.Branches.Local {
		if local.Name != to {
			options = append(options, domain.Option{Value: local.Name, Badge: f.Badge(local.Name)})
		}
	}

	for _, remote := range f.Branches.Remote {
		if !f.Local(remote.Name) && remote.Name != to {
			options = append(options, domain.Option{Value: worktree.OriginPrefix + remote.Name})
		}
	}

	return options
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

	switch {
	case req.Isolation != "":
		question.Answer = domain.Answer[project.Isolation]{Value: req.Isolation, Origin: domain.OriginRequest}
	case !existing:
		question.Answer = domain.IsolationAnswer(project.Isolation(f.Config.Remembered[project.RememberIsolation]), f.Run)
	}

	return question
}

func (f Facts) fastForward(req Request) domain.Confirm {
	params := domain.FastForwardParams{Given: req.FastForward, Remembered: f.Config.Remembered[project.RememberSourceUpdate]}
	switch {
	case f.existing(req):
		params.Skip = existingSkip
	case f.subjectAt(req, worktree.Behind) == "":
		params.Skip = "nothing to fast-forward: the branch it starts from is not behind origin"
	}

	return domain.FastForwardAnswer(params)
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

// facing gives the conflict question the conflicts the target would meet, asking it once there are any.
func (q Questions) facing(conflicts []worktree.Conflict) Questions {
	q.OnConflict.Conflicts = conflicts
	if len(conflicts) > 0 {
		q.OnConflict.Skip = ""
	}

	return q
}

// ----------------------------------------------------------------------------
// Resolve and validation

// The field-error codes extract spells as every command does, with no extract. prefix.
const (
	codeRequired = "required"
	codeNotFound = "not_found"
)

// rememberable are the fields whose answer can be remembered.
func rememberable() []string {
	return []string{"isolation", "fast_forward"}
}

// Resolve settles the request on the facts, every field answered as Form answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := Form(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	return questions.Request(), nil
}

// Request is the request the questions answer; a field whose question is skipped is not remembered.
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

// problems are the request's problems as field errors; a source with nothing to extract has no other, since nothing more is asked of it.
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
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: "isolation", Value: string(q.Isolation.Value), Accepted: domain.Names(project.Isolations())})...)
	}

	problems = append(problems, domain.OneOf(domain.OneOfParams{Path: "on_conflict", Value: string(q.OnConflict.Value), Accepted: domain.Names(OnConflicts())})...)
	for index, id := range q.Remember {
		problems = append(problems, domain.OneOf(domain.OneOfParams{Path: "remember[" + strconv.Itoa(index) + "]", Value: id, Accepted: rememberable()})...)
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

// fileProblems are the problems of the files asked; with no source worktree they are left to the source's own problem.
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

// fromProblems are the problems of the source branch of a created target; an existing target ignores it.
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
