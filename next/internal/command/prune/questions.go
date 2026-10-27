package prune

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// The request

// A Request is what the caller asks of prune: the worktrees to remove among those done with, none for every one safe to remove (every one when forced); whether the children they leave behind move onto the nearest worktree that stays; whether a worktree unsafe to remove is removed all the same; which reasons make a worktree done with, none for all three; and whether origin is fetched first. Caller is where the caller stands, which a removal of its own worktree steps out of; it is no field of the form: every surface sets it.
type Request struct {
	Worktrees        []string      `json:"worktree"`
	ReparentChildren bool          `json:"reparent_children"`
	Force            bool          `json:"force"`
	Merged           bool          `json:"merged"`
	Closed           bool          `json:"closed"`
	Gone             bool          `json:"gone"`
	NoFetch          bool          `json:"no_fetch"`
	Caller           domain.Caller `json:"-"`
}

// From is the request asked by caller.
func (r Request) From(caller domain.Caller) Request {
	r.Caller = caller
	return r
}

// The paths of prune's own fields, by which each surface words them; the worktrees' is domain.PathWorktree.
const (
	PathReparentChildren = "reparent_children"
	PathForce            = "force"
	PathMerged           = "merged"
	PathClosed           = "closed"
	PathGone             = "gone"
	PathNoFetch          = "no_fetch"
)

// Answer is the request with the field at path answered by values. A value the field cannot take leaves the request as it was.
func (r Request) Answer(path string, values []string) (Request, error) {
	next := r
	var err error
	switch path {
	case domain.PathWorktree:
		next.Worktrees = values
	case PathReparentChildren:
		next.ReparentChildren, err = domain.Yes(path, values)
	case PathForce:
		next.Force, err = domain.Yes(path, values)
	case PathMerged:
		next.Merged, err = domain.Yes(path, values)
	case PathClosed:
		next.Closed, err = domain.Yes(path, values)
	case PathGone:
		next.Gone, err = domain.Yes(path, values)
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
	case PathReparentChildren:
		return domain.SaidYes(r.ReparentChildren)
	case PathForce:
		return domain.SaidYes(r.Force)
	case PathMerged:
		return domain.SaidYes(r.Merged)
	case PathClosed:
		return domain.SaidYes(r.Closed)
	case PathGone:
		return domain.SaidYes(r.Gone)
	case PathNoFetch:
		return domain.SaidYes(r.NoFetch)
	}

	return nil
}

// Reasons are the reasons the request asks for, in their order; all three when it names none.
func (r Request) Reasons() []Reason {
	given := map[Reason]bool{ReasonMerged: r.Merged, ReasonClosed: r.Closed, ReasonGone: r.Gone}
	asked := slices.DeleteFunc(Reasons(), func(reason Reason) bool { return !given[reason] })
	if len(asked) == 0 {
		return Reasons()
	}

	return asked
}

// ----------------------------------------------------------------------------
// The form

// Questions is the request as a form: the worktrees done with, which the request picks among, those picked unless the request names some, whether to reparent the children they leave behind, named in Orphans, whether to force the picked worktrees Unsafe names, the reasons and the fetch, set by their flags only, and the request's problems as field errors.
type Questions struct {
	Worktrees  Worktrees
	Candidates []Candidate
	Reparent   domain.Confirm
	Force      domain.Confirm
	Merged     domain.Confirm
	Closed     domain.Confirm
	Gone       domain.Confirm
	NoFetch    domain.Confirm
	Orphans    []teardown.Move
	Unsafe     []Candidate
	Errors     []domain.FieldError
}

// Worktrees is the question of the worktrees to remove: those picked, the request's or by default the command's, and every worktree done with, by its branch; not asked when none is.
type Worktrees struct {
	Picked  []string
	Origin  domain.Origin
	Options []domain.Option
	Skip    domain.Skip
}

// NewQuestions builds the questions for the request on the facts: every worktree done with, those the request names picked, else each one safe to remove (each one when forced); the children the picked ones leave behind, and whether to reparent them, asked only when there are some; the picked ones unsafe to remove, and whether to force them, asked only when there are some. A switch given stands as given.
func NewQuestions(f Facts, req Request) Questions {
	candidates := f.Candidates(req)
	picked, problems := f.picked(req, candidates)
	questions := Questions{
		Worktrees:  Worktrees{Picked: names(picked), Origin: domain.OriginDefault, Options: options(candidates)},
		Candidates: candidates,
		Orphans:    f.Moves(removals(picked)),
		Errors:     problems,
		Merged:     flagOnly(req.Merged),
		Closed:     flagOnly(req.Closed),
		Gone:       flagOnly(req.Gone),
		NoFetch:    flagOnly(req.NoFetch),
	}
	if len(req.Worktrees) > 0 {
		questions.Worktrees.Origin, questions.Unsafe = domain.OriginRequest, blocked(picked)
	}
	if len(candidates) == 0 && len(req.Worktrees) == 0 {
		questions.Worktrees.Skip = domain.SkipNoCandidate
	}
	questions.Reparent = confirm(req.ReparentChildren, len(questions.Orphans) == 0, domain.SkipNoOrphan)
	questions.Force = confirm(req.Force, len(questions.Unsafe) == 0, domain.SkipSafe)

	return questions
}

// Form is the questions as the schema a surface renders: the worktrees, whether to reparent the children left behind, whether to force the unsafe ones, then the reasons and the fetch, which only their flags set. A zero Questions gives the schema the flags and --help are spelled from.
func (q Questions) Form() domain.Form {
	return domain.Form{Problems: q.Errors, Fields: []domain.Field{
		domain.MultiSelectField{FieldSpec: domain.FieldSpec{Path: domain.PathWorktree, Origin: q.Worktrees.Origin, Skip: q.Worktrees.Skip}, Picked: q.Worktrees.Picked, Options: q.Worktrees.Options},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathReparentChildren, Origin: q.Reparent.Origin, Skip: q.Reparent.Skip}, Value: q.Reparent.Value, NoFirst: true},
		domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: PathForce, Origin: q.Force.Origin, Skip: q.Force.Skip}, Value: q.Force.Value, NoFirst: true},
		flagField(PathMerged, q.Merged),
		flagField(PathClosed, q.Closed),
		flagField(PathGone, q.Gone),
		flagField(PathNoFetch, q.NoFetch),
	}}
}

func flagField(path string, c domain.Confirm) domain.ConfirmField {
	return domain.ConfirmField{FieldSpec: domain.FieldSpec{Path: path, Origin: c.Origin, Skip: c.Skip}, Value: c.Value}
}

// options are the candidates as the options of the worktrees, in git's order.
func options(candidates []Candidate) []domain.Option {
	found := make([]domain.Option, 0, len(candidates))
	for _, c := range candidates {
		found = append(found, domain.Option{Value: c.Worktree})
	}

	return found
}

// confirm is a switch: given, it stands as the request's; else no by default, not asked when moot says nothing would be asked of it.
func confirm(given, moot bool, skip domain.Skip) domain.Confirm {
	switch {
	case given:
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	case moot:
		return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: skip}
	}

	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}
}

// flagOnly is a switch never asked, its flag the only way to set it; given, it stands in the trail with its flag.
func flagOnly(given bool) domain.Confirm {
	if given {
		return domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}}
	}

	return domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}, Skip: domain.SkipFlagOnly}
}

// ----------------------------------------------------------------------------
// Resolve and validation

// Resolve settles the request on the facts; a request with problems is refused with all of them, by domain.Invalid.
func Resolve(f Facts, req Request) (Request, error) {
	questions := NewQuestions(f, req)
	if len(questions.Errors) > 0 {
		return Request{}, domain.Invalid(questions.Errors)
	}

	return req, nil
}
