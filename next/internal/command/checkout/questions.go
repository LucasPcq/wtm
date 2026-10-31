package checkout

import (
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of checkout, as given: the pull request by its number, its parent, the filter of the pull requests to pick from (Mine or Review), whether the pull request's branch is read as last fetched (NoFetch, as a surface that fetched it a moment ago asks again), and the questions of a new worktree; a field left empty takes its remembered answer, its configured value or its default when the request is resolved.
type Request struct {
	PR      int    `json:"pr"`
	From    string `json:"from"`
	Mine    bool   `json:"mine"`
	Review  bool   `json:"review"`
	NoFetch bool   `json:"no_fetch"`
	provision.Given
}

// The paths of checkout's own fields, by which each surface words them; the questions of a new worktree name theirs in domain.
const (
	PathPR      = "pr"
	PathFrom    = "from"
	PathMine    = "mine"
	PathReview  = "review"
	PathNoFetch = "no_fetch"
)

// Answer is the request with the field at path answered by values, spelled as the field's Answer spells them; the path "remember" answers the fields whose answer is remembered. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case PathPR:
		next.PR, err = number(values)
	case PathFrom:
		next.From, err = domain.One[string](path, values)
	case PathMine:
		next.Mine, err = domain.Yes(path, values)
	case PathReview:
		next.Review, err = domain.Yes(path, values)
	case PathNoFetch:
		next.NoFetch, err = domain.Yes(path, values)
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
	case PathPR:
		if r.PR == 0 {
			return nil
		}
		return []string{strconv.Itoa(r.PR)}
	case PathFrom:
		return domain.Spelled(r.From)
	case PathMine:
		return domain.SaidYes(r.Mine)
	case PathReview:
		return domain.SaidYes(r.Review)
	case PathNoFetch:
		return domain.SaidYes(r.NoFetch)
	}

	return r.Given.Values(path)
}

// CodePRInvalid refuses a pull request that is not a number above zero.
const CodePRInvalid = Name + ".pr_invalid"

// number is the pull request values name, 0 for none.
func number(values []string) (int, error) {
	value, err := domain.One[string](PathPR, values)
	if err != nil || value == "" {
		return 0, err
	}

	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, domain.Invalid([]domain.FieldError{{Path: PathPR, Code: CodePRInvalid, Template: "{path} must be a pull request number, not {value}", Params: domain.Params{"value": value}}})
	}

	return n, nil
}

// filter is the filter of the pull requests to pick from the request asks.
func (r Request) filter() worktree.PullRequestFilter {
	switch {
	case r.Mine:
		return worktree.MyPullRequests
	case r.Review:
		return worktree.ReviewRequested
	}

	return worktree.AllPullRequests
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: each question with its options, its answer so far and where that answer comes from, the questions of a new worktree with the fields whose answer is to be remembered, and the request's problems as field errors.
type Questions struct {
	PR      PR
	From    From
	Mine    domain.Confirm
	Review  domain.Confirm
	NoFetch bool
	provision.Questions
	Errors []domain.FieldError
}

// PR is the question of the pull request: its number, 0 for none, and the open pull requests to pick from, each tagged when it cannot be checked out (a fork's, or one whose branch a worktree holds).
type PR struct {
	domain.Answer[int]
	Options []domain.Option
}

// From is the question of the parent: its answer, the branches it can be, the pull request's own aside, and the branch the pull request merges into, pinned at the top of the list.
type From struct {
	domain.Answer[string]
	Options []domain.Option
	Pinned  string
}

// NewQuestions builds the questions for the request on the facts: a field given keeps its value, an empty one takes its remembered answer (unless the request asks again), its configured value or its default.
func NewQuestions(f Facts, req Request) Questions {
	from := f.from(req)
	req.From = from.Value

	questions := Questions{
		PR:        PR{Answer: domain.Answer[int]{Value: req.PR, Origin: given(req.PR != 0)}, Options: f.prOptions()},
		From:      from,
		Mine:      flagOnly(req.Mine),
		Review:    flagOnly(req.Review),
		NoFetch:   req.NoFetch,
		Questions: f.newWorktree(req),
	}
	questions.Errors = f.problems(questions)
	return questions
}

// Form is the questions as the schema a surface renders: the fields in the order the wizard asks them, each with its answer so far. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	pr := ""
	if q.PR.Value != 0 {
		pr = strconv.Itoa(q.PR.Value)
	}

	fields := []domain.Field{
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathPR, Origin: q.PR.Origin}, Value: pr, Options: q.PR.Options},
		domain.SelectField{FieldSpec: domain.FieldSpec{Path: PathFrom, Origin: q.From.Origin}, Value: q.From.Value, Options: q.From.Options, Pinned: q.From.Pinned, Refreshable: true},
	}
	fields = append(fields, q.Questions.Fields()...)
	fields = append(fields,
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathMine, Origin: q.Mine.Origin, Skip: q.Mine.Skip}, Value: q.Mine.Value},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathReview, Origin: q.Review.Origin, Skip: q.Review.Skip}, Value: q.Review.Value},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathNoFetch, Skip: domain.SkipFlagOnly}, Value: q.NoFetch},
	)

	return domain.Form{Problems: q.Errors, Fields: fields}
}

func given(isGiven bool) domain.Origin {
	if isGiven {
		return domain.OriginRequest
	}

	return ""
}

// prOptions are the open pull requests, and the one asked when it is not open, each with what a picker shows of it: where its branch stands against origin when it exists locally (fetched for the one asked, as last fetched for the others), a fork's tagged fork, one whose branch a worktree holds tagged checked out, each of these two with the problem picking it raises.
func (f Facts) prOptions() []domain.Option {
	prs := f.Open
	if f.PR != nil && !slices.ContainsFunc(prs, func(pr worktree.PullRequest) bool { return pr.Number == f.PR.Number }) {
		prs = append(slices.Clone(prs), *f.PR)
	}

	options := make([]domain.Option, 0, len(prs))
	for _, pr := range prs {
		option := domain.Option{Value: strconv.Itoa(pr.Number), PR: &pr}
		switch at := f.CheckedOutAt(pr.Branch); {
		case pr.Fork:
			option.Tag, option.Problem = domain.TagFork, problemOf(forkOf(pr))
		case at != "":
			option.Tag, option.Problem = domain.TagCheckedOut, problemOf(checkedOut(pr, at))
		case f.Local(pr.Branch):
			option.Origin = f.OriginOf(pr.Branch)
		}
		options = append(options, option)
	}

	return options
}

// problemOf is refused as the problem of the pull request field.
func problemOf(refused refusal) *domain.FieldError {
	problem := domain.FieldError(refused)
	problem.Path = PathPR
	return &problem
}

// from answers the parent with the branch the pull request merges into, unless the request names one; the pull request's own branch is never offered.
func (f Facts) from(req Request) From {
	base := f.base()
	question := From{Options: domain.SourceOptions(f.Repository, f.branch()), Pinned: base}
	switch {
	case req.From != "":
		question.Answer = domain.Answer[string]{Value: req.From, Origin: domain.OriginRequest}
	case base != "":
		question.Answer = domain.Answer[string]{Value: base, Origin: domain.OriginPullRequest}
	}

	return question
}

// newWorktree are the questions of the new worktree, on the facts; the fast-forward is asked while the pull request's branch exists locally behind origin.
func (f Facts) newWorktree(req Request) provision.Questions {
	return provision.NewQuestions(provision.QuestionsParams{Given: req.Given, Repository: f.Repository, Behind: f.behind()})
}

// flagOnly is a filter of the pull requests to pick from, never asked, its flag the only way to set it; given, it stands in the trail with its flag.
func flagOnly(given bool) domain.Confirm {
	if given {
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	}

	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: domain.SkipFlagOnly}
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts, every field answered as NewQuestions answers it; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions, err := resolve(f, req)
	return questions.Request(), err
}

// resolve is the questions of the request on the facts, refused when one is not settled.
func resolve(f Facts, req Request) (Questions, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Questions{}, domain.Invalid(questions.Errors)
	}

	return questions, nil
}

// Request is the request the questions answer.
func (q Questions) Request() Request {
	return Request{PR: q.PR.Value, From: q.From.Value, Mine: q.Mine.Value, Review: q.Review.Value, NoFetch: q.NoFetch, Given: q.Questions.Given()}
}

// CodeFiltersExclusive refuses both filters of the pull requests to pick from at once.
const CodeFiltersExclusive = Name + ".filters_exclusive"

func (f Facts) problems(q Questions) []domain.FieldError {
	problems := f.prProblems(q.PR.Value)
	problems = append(problems, f.fromProblems(q.From.Value)...)
	problems = append(problems, q.Questions.Problems()...)
	if q.Mine.Value && q.Review.Value {
		problems = append(problems, domain.FieldError{Path: PathReview, Code: CodeFiltersExclusive, Template: "{path} cannot be given with the filter of your own pull requests: pick one", With: []string{PathMine}})
	}

	return problems
}

// prProblems are the refusals of the pull request asked: none asked, none of that number, or one that cannot be checked out.
func (f Facts) prProblems(number int) []domain.FieldError {
	switch {
	case number == 0:
		return []domain.FieldError{domain.Required(PathPR)}
	case f.PR == nil:
		return []domain.FieldError{{Path: PathPR, Code: domain.CodeNotFound, Template: "{path}: no pull request #{number}", Params: domain.Params{"number": strconv.Itoa(number)}}}
	}

	if refused, isRefused := f.decide().(refusal); isRefused {
		return []domain.FieldError{*problemOf(refused)}
	}

	return nil
}

// fromProblems are the problems of the parent: a branch that is none, the pull request's own, or none when the branch the pull request merges into is neither local nor on origin.
func (f Facts) fromProblems(from string) []domain.FieldError {
	if f.PR == nil || f.PR.Fork {
		return nil
	}

	branch := f.PR.Branch
	switch {
	case from == "":
		return []domain.FieldError{{Path: PathFrom, Code: domain.CodeRequired, Template: "{path} is required: {base}, the branch the pull request merges into, is neither local nor on origin", Params: domain.Params{"base": f.PR.Base}}}
	case from == branch || from == worktree.OriginPrefix+branch:
		return []domain.FieldError{{Path: PathFrom, Code: domain.CodeSelfParent, Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": from}, With: []string{PathPR}}}
	case !f.Candidate(from):
		return []domain.FieldError{{Path: PathFrom, Code: domain.CodeNotFound, Template: "{path}: {value} not found", Params: domain.Params{"value": from}}}
	}

	return nil
}
