package extract

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
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
	Source string   `json:"source"`
	Files  []string `json:"files"` // paths of changes, or directories to take every change below
	To     string   `json:"to"`
	From   string   `json:"from"`
	provision.Given
	Keep       *bool      `json:"keep"` // copy rather than move; nil when not given
	OnConflict OnConflict `json:"on_conflict"`
}

// The paths of extract's own fields; the questions of a new worktree name theirs in provision.
const (
	pathSource     = "source"
	pathFiles      = "files"
	PathTo         = "to"
	pathFrom       = "from"
	pathKeep       = "keep"
	pathOnConflict = "on_conflict"
)

// Answer is the request with the field at path answered by values, spelled as the field's Answer spells them; the path "remember" answers the fields whose answer is remembered. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	answer := domain.AnswerParams{Path: path, Values: values}
	next := r
	var err error
	switch path {
	case pathFiles:
		next.Files = values
	case pathSource:
		next.Source, err = domain.One[string](answer)
	case PathTo:
		next.To, err = domain.One[string](answer)
	case pathFrom:
		next.From, err = domain.One[string](answer)
	case pathOnConflict:
		next.OnConflict, err = domain.One[OnConflict](answer)
	case pathKeep:
		var yes bool
		yes, err = domain.Yes(answer)
		next.Keep = &yes
	default:
		next.Given, err = r.Given.Answer(path, values)
	}

	if err != nil {
		return r, err
	}

	return next, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case pathFiles:
		return r.Files
	case pathSource:
		return domain.Spelled(r.Source)
	case PathTo:
		return domain.Spelled(r.To)
	case pathFrom:
		return domain.Spelled(r.From)
	case pathOnConflict:
		return domain.Spelled(r.OnConflict)
	case pathKeep:
		return domain.Said(r.Keep)
	}

	return r.Given.Values(path)
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: each question with its options, its answer so far and where that answer comes from, the questions of a created target with the fields whose answer is to be remembered, and the request's problems as field errors.
type Questions struct {
	Source Source
	Files  Files
	To     To
	From   From
	provision.Questions
	Keep       domain.Confirm
	OnConflict Conflicting
	Errors     []domain.FieldError
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

// NewQuestions builds the questions for the request on the facts: a field given keeps its value, an empty one takes its remembered answer (unless the request asks again), its recorded or configured value or its default. Its conflict question has no conflicts yet: they are read once the request has no problem (UseCase.Questions).
func NewQuestions(f Facts, req Request) Questions {
	// The source branch is settled first: whether a fast-forward is asked depends on the branch the target starts from.
	from := f.from(req)
	req.From = from.Value

	questions := Questions{
		Source:     Source{Given: req.Source, Options: f.sourceOptions()},
		Files:      Files{Given: req.Files, Options: f.Changes[req.Source]},
		To:         To{Given: req.To, Options: f.targetOptions(req.Source)},
		From:       from,
		Questions:  f.newWorktree(req),
		Keep:       keep(req.Keep),
		OnConflict: onConflict(req.OnConflict),
	}
	questions.Errors = f.problems(req, questions)
	return questions
}

// Form is the questions as the schema a surface renders: the fields in the order the wizard asks them, each with its flag, label, help and answer so far. The target is asked twice: as a worktree to pick, then, for one to create, as its branch. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	source, to := q.Source.Given, q.To.Given
	fields := []domain.Field{
		domain.SelectField{
			FieldSpec: domain.FieldSpec{Path: pathSource, Flag: "<source>", Label: "Source worktree", Help: "Which worktree to extract changes from"},
			Value:     source,
			Options:   q.Source.Options,
		},
		domain.MultiSelectField{
			FieldSpec: domain.FieldSpec{Path: pathFiles, Flag: "--files", Label: "Files", Help: q.filesHelp()},
			Picked:    q.Files.Given,
			Options:   q.fileOptions(),
			Empty:     "Select at least one file",
		},
		domain.SelectField{
			FieldSpec: domain.FieldSpec{Path: PathTo, Flag: "--to", Label: "Target worktree", Help: "Target worktree branch; created if it does not exist"},
			Value:     to,
			Options:   append([]domain.Option{{Label: "+ Create a new worktree…", Short: "new worktree"}}, q.To.Options...),
		},
		domain.TextField{
			FieldSpec: domain.FieldSpec{Path: PathTo, Flag: "--to", Label: "Branch name", Help: "Name for the new worktree branch", Skip: q.branchSkip()},
			Value:     to,
		},
		domain.SelectField{
			FieldSpec:   domain.FieldSpec{Path: pathFrom, Flag: "--from", Label: "Source branch", Help: "Parent branch when creating the target worktree", Origin: q.From.Origin, Skip: q.From.Skip},
			Value:       q.From.Value,
			Options:     q.From.Options,
			Pinned:      q.From.Pinned,
			Refreshable: true,
		},
	}
	fields = append(fields, q.Questions.Fields(wording)...)
	fields = append(fields,
		domain.ConfirmField{
			FieldSpec: domain.FieldSpec{Path: pathKeep, Flag: "--keep", Label: "Mode", Help: "Copy instead of move (keep the changes in the source)", Origin: q.Keep.Origin},
			Value:     q.Keep.Value,
			Yes:       domain.Option{Value: "true", Label: "Copy — keep the files in " + source, Short: "copy"},
			No:        domain.Option{Value: "false", Label: "Move — remove the files from " + source, Short: "move"},
			NoFirst:   true,
		},
		domain.SelectField{
			FieldSpec: domain.FieldSpec{Path: pathOnConflict, Flag: "--on-conflict", Label: "On conflict", Help: q.conflictHelp(), Origin: q.OnConflict.Origin, Skip: q.OnConflict.Skip},
			Value:     string(q.OnConflict.Value),
			Options: []domain.Option{
				{Value: string(OnConflictAbort), Label: "Abort — leave both worktrees untouched"},
				{Value: string(OnConflictResolve), Label: "Resolve — write conflict markers in " + to},
			},
		},
	)

	return domain.Form{Problems: q.Errors, Fields: fields}
}

var wording = provision.Wording{
	EnvHelp:         "How the .env files of the new target are provisioned: config (config.toml's strategy), example (from its template), main (copied from the main checkout) or parent (from the worktree of the branch it starts from, --from); defaults to config.toml's strategy",
	FastForwardHelp: "Fast-forward the parent branch to origin before creating the target; skipped when it has diverged",
	Yes:             domain.Option{Value: "true", Label: "Fast-forward to origin"},
	No:              domain.Option{Value: "false", Label: "Keep it as-is"},
}

func (q Questions) filesHelp() string {
	if q.Source.Given != "" && len(q.Files.Options) == 0 {
		return "No changes to extract in " + q.Source.Given + " — press esc to pick another worktree."
	}

	return "Files to extract, or a directory to take everything below it"
}

func (q Questions) fileOptions() []domain.Option {
	options := make([]domain.Option, 0, len(q.Files.Options))
	for _, change := range q.Files.Options {
		options = append(options, domain.Option{Value: change.Path, Label: change.Label(), Badge: change.Status.Tag()})
	}

	return options
}

// branchSkip skips the branch of the target while the target is a worktree that exists: the source, or another one.
func (q Questions) branchSkip() string {
	if to := q.To.Given; to != "" && (to == q.Source.Given || q.held(to)) {
		return existingSkip
	}

	return ""
}

// conflictHelp names the files that conflict with the target and what each answer leaves behind; while nothing conflicts, it says what each answer does.
func (q Questions) conflictHelp() string {
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

// NewTarget is the problem of branch typed as the branch of a target to create, as a field error of the target: a worktree already holds it.
func (q Questions) NewTarget(branch string) []domain.FieldError {
	if !q.held(branch) {
		return nil
	}

	return []domain.FieldError{{Path: PathTo, Code: "extract.has_worktree", Template: "{value} already has a worktree — pick it as the target worktree", Params: domain.Params{"value": branch}}}
}

// held reports whether a worktree other than the source holds branch.
func (q Questions) held(branch string) bool {
	return slices.ContainsFunc(q.To.Options, func(o domain.Option) bool { return o.Value == branch })
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
	question := From{Options: domain.SourceOptions(f.Snapshot, req.To), Pinned: cmp.Or(recorded, base)}
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

// newWorktree are the questions of the target to create, on the facts: none is asked of a target that exists, and the fast-forward is asked while the branch it starts from is behind origin.
func (f Facts) newWorktree(req Request) provision.Questions {
	params := provision.QuestionsParams{Given: req.Given, Repository: f.Repository}
	switch {
	case f.existing(req):
		params.Exists = existingSkip
	case f.subjectAt(req, worktree.Behind) == "":
		params.NotBehind = "nothing to fast-forward: the branch it starts from is not behind origin"
	}

	return provision.NewQuestions(params)
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

// Resolve settles the request on the facts, every field answered as NewQuestions answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	return questions.Request(), nil
}

// Request is the request the questions answer.
func (q Questions) Request() Request {
	return Request{
		Source:     q.Source.Given,
		Files:      q.Files.Given,
		To:         q.To.Given,
		From:       q.From.Value,
		Given:      q.Questions.Given(),
		Keep:       new(q.Keep.Value),
		OnConflict: q.OnConflict.Value,
	}
}

// problems are the request's problems as field errors; a source with nothing to extract has no other, since nothing more is asked of it.
func (f Facts) problems(req Request, q Questions) []domain.FieldError {
	problems := f.sourceProblems(req.Source)
	if f.nothing(req.Source) {
		return problems
	}

	problems = append(problems, f.fileProblems(req)...)
	if refused, isRefused := f.target(req).(refusal); isRefused {
		problems = append(problems, refused.field(PathTo))
	}

	problems = append(problems, f.fromProblems(req)...)
	problems = append(problems, q.Questions.Problems()...)
	problems = append(problems, domain.OneOf(domain.OneOfParams{Path: pathOnConflict, Value: string(q.OnConflict.Value), Accepted: domain.Names(OnConflicts())})...)

	return problems
}

func (f Facts) sourceProblems(source string) []domain.FieldError {
	switch {
	case source == "":
		return []domain.FieldError{domain.Required(pathSource)}
	case f.CheckedOutAt(source) == "":
		return []domain.FieldError{{Path: pathSource, Code: domain.CodeNotFound, Template: "{path}: no worktree holds {value}", Params: domain.Params{"value": source}}}
	}

	return nil
}

// fileProblems are the problems of the files asked; with no source worktree they are left to the source's own problem.
func (f Facts) fileProblems(req Request) []domain.FieldError {
	switch {
	case f.CheckedOutAt(req.Source) == "":
		return nil
	case len(req.Files) == 0:
		return []domain.FieldError{domain.Required(pathFiles)}
	}

	if _, missed := worktree.SelectChanges(f.Changes[req.Source], req.Files); missed != "" {
		return []domain.FieldError{{Path: pathFiles, Code: "extract.not_a_change", Template: "{path}: {value} is not an uncommitted change", Params: domain.Params{"value": missed}}}
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
			Path:     pathFrom,
			Code:     domain.CodeRequired,
			Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
			Params:   domain.Params{"because": "extract.parent_required"},
		}}
	case from == "":
		return []domain.FieldError{{Path: pathFrom, Code: domain.CodeRequired, Template: "{path} is required: no parent is recorded for the source and no base branch is configured"}}
	case from == target.Branch:
		return []domain.FieldError{{Path: pathFrom, Code: domain.CodeSelfParent, Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": from}, With: []string{PathTo}}}
	case !f.Candidate(from):
		return []domain.FieldError{{Path: pathFrom, Code: domain.CodeNotFound, Template: "{path}: {value} not found", Params: domain.Params{"value": from}}}
	}

	return nil
}
