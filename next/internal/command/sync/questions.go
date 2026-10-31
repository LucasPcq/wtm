package sync

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of sync: the worktrees it names, each by its branch or its path, else those that need a sync, or every worktree with All; whether the worktrees under those named come too (Descendants, yes unless said no); whether a worktree with uncommitted changes is rebased all the same, its changes stashed around the rebase (Force); whether a rebase that stops on a conflict is left in progress rather than aborted (KeepConflict); whether the branches rebased onto but not rebased, the base included, are fast-forwarded to origin first (FFParents, yes unless said no); whether each branch is force-pushed once synced (Push); and whether origin is read as last fetched (NoFetch). The worktrees it names are the user's choice, the others, descendants included, the command's: a named worktree it cannot rebase safely refuses the request, one the command chose is kept as it is. Caller is where the caller stands, from which a path is read; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees    []string      `json:"worktree"`
	All          bool          `json:"all"`
	Descendants  *bool         `json:"descendants,omitempty"`
	Force        bool          `json:"force"`
	KeepConflict bool          `json:"keep_conflict"`
	FFParents    *bool         `json:"ff_parents,omitempty"`
	Push         bool          `json:"push"`
	NoFetch      bool          `json:"no_fetch"`
	Caller       domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of sync's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathAll          = "all"
	PathDescendants  = "descendants"
	PathForce        = "force"
	PathKeepConflict = "keep_conflict"
	PathFFParents    = "ff_parents"
	PathPush         = "push"
	PathNoFetch      = "no_fetch"
)

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathAll:
		next.All, err = domain.Yes(path, values)
	case PathDescendants:
		next.Descendants, err = said(path, values)
	case PathForce:
		next.Force, err = domain.Yes(path, values)
	case PathKeepConflict:
		next.KeepConflict, err = domain.Yes(path, values)
	case PathFFParents:
		next.FFParents, err = said(path, values)
	case PathPush:
		next.Push, err = domain.Yes(path, values)
	case PathNoFetch:
		next.NoFetch, err = domain.Yes(path, values)
	default:
		return r, domain.Unknown(path)
	}

	if err != nil {
		return r, err
	}
	return next, nil
}

// Values is the request's answer to the field at path, spelled as Answer takes it; nil when the request leaves the field to its default.
func (r Request) Values(path string) []string {
	switch path {
	case domain.PathWorktree:
		return r.Worktrees
	case PathAll:
		return domain.SaidYes(r.All)
	case PathDescendants:
		return domain.Said(r.Descendants)
	case PathForce:
		return domain.SaidYes(r.Force)
	case PathKeepConflict:
		return domain.SaidYes(r.KeepConflict)
	case PathFFParents:
		return domain.Said(r.FFParents)
	case PathPush:
		return domain.SaidYes(r.Push)
	case PathNoFetch:
		return domain.SaidYes(r.NoFetch)
	}

	return nil
}

// WithDescendants reports whether the worktrees under those named come too: yes unless said no.
func (r Request) WithDescendants() bool {
	return r.Descendants == nil || *r.Descendants
}

// WithParents reports whether the branches rebased onto but not rebased are fast-forwarded first: yes unless said no.
func (r Request) WithParents() bool {
	return r.FFParents == nil || *r.FFParents
}

func said(path string, values []string) (*bool, error) {
	yes, err := domain.Yes(path, values)
	return &yes, err
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees to sync and those it may, whether every one is, whether those under the named come too and which they are, whether to force the worktrees Unsafe names, what a conflict does, whether to fast-forward the branches Behind names first, whether to push, and the request's problems as field errors.
type Questions struct {
	Worktrees    Worktrees
	All          bool
	Descendants  Descendants
	Force        domain.Confirm
	KeepConflict domain.Confirm
	FFParents    domain.Confirm
	Push         domain.Confirm
	NoFetch      bool
	Unsafe       []Rebase
	Behind       []Onto
	Errors       []domain.FieldError
}

// Worktrees is the question of the worktrees to sync: those given, else its default pick, the worktrees that need a sync; and every worktree sync may rebase, each with where it stands against origin and its uncommitted changes.
type Worktrees struct {
	Picked  domain.Answer[[]string]
	Options []domain.Option
}

// Descendants is the question of whether the worktrees under those named come too: its answer, and the worktrees under each named one that has some, by the named one.
type Descendants struct {
	domain.Confirm
	Under map[string][]string
}

// NewQuestions builds the questions for the request on the facts, as last fetched, each asked whenever the fetch the plan makes first could make it matter: the worktrees given, else the default pick; whether the worktrees under the named come too, asked when one named has some; the worktrees named with uncommitted changes that may rebase, and whether to force them, asked only when there are some: those the command chose are kept instead; what a conflict does, asked when a worktree may rebase; whether to fast-forward the branches rebased onto, asked when one is on origin; and whether to push, asked when a worktree that may rebase is on origin.
func NewQuestions(f Facts, req Request) Questions {
	decided := f.decide(req)
	questions := Questions{
		Worktrees: Worktrees{Picked: domain.Answer[[]string]{Value: req.Worktrees, Origin: domain.OriginRequest}, Options: f.worktreeOptions(req.Caller.Cwd)},
		All:       req.All,
		NoFetch:   req.NoFetch,
		Behind:    decided.behind(),
		Errors:    decided.problems,
	}
	if len(req.Worktrees) == 0 && !req.All {
		questions.Worktrees.Picked = domain.Answer[[]string]{Value: f.picked(), Origin: domain.OriginDefault}
	}
	// Asked on origin as last fetched while the plan stands on it fetched: a question is asked whenever the fetch could make it matter.
	if req.named() {
		questions.Unsafe = f.mayBeUnsafe(decided.plan.Worktrees)
	}

	under := f.under(req)
	questions.Descendants = Descendants{Confirm: given(req.Descendants, skipUnless(len(under) > 0, domain.SkipNoDescendant)), Under: under}
	if req.All {
		questions.Descendants.Skip = domain.SkipAll
	}
	questions.Force = confirm(req.Force, skipUnless(len(questions.Unsafe) > 0, domain.SkipSafe))
	questions.KeepConflict = confirm(req.KeepConflict, skipUnless(slices.ContainsFunc(decided.plan.Worktrees, Rebase.mayMove), domain.SkipNoRebase))
	questions.FFParents = given(req.FFParents, skipUnless(slices.ContainsFunc(decided.plan.Onto, func(o Onto) bool { return o.Origin != nil }), domain.SkipNotBehind))
	questions.Push = confirm(req.Push, skipUnless(slices.ContainsFunc(decided.plan.Worktrees, func(r Rebase) bool { return r.mayMove() && r.Origin != nil }), domain.SkipNothingToPush))

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, unless every one is synced, then whether every one is, a switch only its flag sets, whether the worktrees under the named come too, whether to force the unsafe ones, what a conflict does, whether to fast-forward the branches rebased onto, whether to push, and whether to fetch, a switch only its flag sets. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	worktrees := domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Picked.Origin}
	if q.All {
		worktrees.Skip = domain.SkipAll
	}

	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: worktrees, Picked: q.Worktrees.Picked.Value, Options: q.Worktrees.Options},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathAll, Skip: domain.SkipFlagOnly}, Value: q.All, NoFirst: true},
		confirmField(PathDescendants, q.Descendants.Confirm),
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Origin: q.Force.Origin, Skip: q.Force.Skip}, Value: q.Force.Value, NoFirst: true},
		noFirst(confirmField(PathKeepConflict, q.KeepConflict)),
		confirmField(PathFFParents, q.FFParents),
		noFirst(confirmField(PathPush, q.Push)),
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathNoFetch, Skip: domain.SkipFlagOnly}, Value: q.NoFetch},
	}}
}

func confirmField(path string, c domain.Confirm) domain.ConfirmField {
	return domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: path, Origin: c.Origin, Skip: c.Skip}, Value: c.Value}
}

// noFirst offers a switch whose no is its default with its no first.
func noFirst(field domain.ConfirmField) domain.ConfirmField {
	field.NoFirst = true
	return field
}

// worktreeOptions are the worktrees sync may rebase, tagged as every picker tags them from where the caller stands at cwd, with where each stands against origin as last fetched, its uncommitted changes, its parent, whether that is gone, the commits of it the worktree lacks, and the operation git stopped in the middle of in it.
func (f Facts) worktreeOptions(cwd string) []domain.Option {
	options := domain.WorktreeOptions(f.candidates(), domain.WorktreeQuery{Cwd: cwd})
	for i, option := range options {
		check := f.Checks[option.Value]
		parent, _ := f.Stack.Parent(option.Value)
		options[i].Origin, options[i].Changes, options[i].Behind = f.OriginOf(option.Value), check.Changes, check.Behind
		options[i].Parent, options[i].Gone, options[i].Operation = parent, f.ParentGone(parent), f.checkout(option.Value).Operation()
	}

	return options
}

// confirm is a switch whose no is its default: given, it stands as the request's; else no, its question skipped with skip unless skip is "".
func confirm(given bool, skip domain.Skip) domain.Confirm {
	if given {
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	}
	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: skip}
}

// given is a switch whose yes is its default: said, it stands as the request's; its question skipped with skip unless skip is "".
func given(said *bool, skip domain.Skip) domain.Confirm {
	if said != nil {
		return domain.Confirm{Answer: domain.Answer[bool]{Value: *said, Origin: domain.OriginRequest}, Skip: skip}
	}
	return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginDefault}, Skip: skip}
}

func skipUnless(asked bool, skip domain.Skip) domain.Skip {
	if asked {
		return ""
	}
	return skip
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts; a request with problems is refused with all of them, by domain.Invalid, and followed by fast-forward when it names the base branch, which only fast-forward refreshes.
func Resolve(f Facts, req Request) (Request, error) {
	problems := f.decide(req).problems
	if len(problems) == 0 {
		return req, nil
	}

	invalid := domain.Invalid(problems)
	if at := slices.IndexFunc(problems, func(p domain.FieldError) bool { return p.Code == CodeBaseBranch }); at >= 0 {
		base := problems[at].Params["value"]
		invalid.FollowUp = &domain.FollowUp{Command: domain.CommandFastForward, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{base}}}, Code: CodeFastForward, Message: "bring it up to origin instead"}
	}
	return Request{}, invalid
}

// named reports whether the request names its worktrees, the user's choice, rather than leaving them to the command.
func (r Request) named() bool {
	return len(r.Worktrees) > 0 && !r.All
}

// CodeAllAndNames refuses every worktree asked with some named; CodeDetached refuses a worktree on no branch, which has nothing to rebase; CodeBaseBranch refuses the main worktree or the base branch's, which sync rebases onto and never rebases, followed by CodeFastForward.
const (
	CodeAllAndNames = Name + ".all_and_names"
	CodeDetached    = Name + ".detached"
	CodeBaseBranch  = Name + ".base_branch"
	CodeFastForward = Name + ".fast_forward"
)

// named are the worktrees req names, in the order asked, and the problem of each entry it cannot take: blank, given twice, a worktree on no branch, the main worktree or the base branch's, a name that is no worktree, a name with All.
func (f Facts) named(req Request) (names []string, entries map[string]int, problems []domain.FieldError) {
	if req.All && len(req.Worktrees) > 0 {
		problems = append(problems, domain.FieldError{Path: PathAll, Code: CodeAllAndNames, Template: "{path} syncs every worktree: name none with it"})
	}
	if !req.named() {
		return nil, nil, problems
	}

	entries = map[string]int{}
	for index, name := range req.Worktrees {
		path := entryPath(index)
		w, found := f.subject(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(req.Worktrees[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case !found:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		case w.Branch() == "":
			problems = append(problems, domain.FieldError{Path: path, Code: CodeDetached, Template: "{value} is on no branch: nothing to rebase", Params: domain.Params{"value": name}})
		case w.Main || w.Branch() == f.Config.Worktrees.BaseBranch:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeBaseBranch, Template: "{value} is the base branch: sync rebases onto it, never it", Params: domain.Params{"value": w.Branch()}})
		case slices.Contains(names, w.Branch()):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} names a worktree given already", Params: domain.Params{"value": name}})
		default:
			names, entries[w.Branch()] = append(names, w.Branch()), index
		}
	}

	return names, entries, problems
}

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}
